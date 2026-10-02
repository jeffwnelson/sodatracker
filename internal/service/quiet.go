package service

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jeffwnelson/sodatracker/internal/domain"
	"github.com/jeffwnelson/sodatracker/internal/events"
)

// Quiet alerts catch dispenses the machine made but SodaTracker didn't see.
// A heartbeat says the Pi agent is running, not that its contacts work: a
// bent arm or a corroded contact silences one slot, and a loose shared
// ground silences all of them, with the agent online throughout.
//
// They are stored as alert events. quiet_slot uses CurrentStock for the cans
// the other slots dispensed and Threshold for the setting; quiet_machine
// (slotPosition 0) uses them for the quiet hours and the setting.

const (
	AlertQuietSlot    = "quiet_slot"
	AlertQuietMachine = "quiet_machine"
)

const (
	keyQuietSlotCans     = "alerts.quiet_slot_cans"
	keyQuietMachineHours = "alerts.quiet_machine_hours"
	keyQuietWeekdaysOnly = "alerts.quiet_machine_weekdays_only"
	keyAlertTimezone     = "alerts.timezone"

	defaultQuietSlotCans     = 15
	defaultQuietMachineHours = 24
	defaultAlertTimezone     = "America/Chicago"

	maxQuietSlotCans     = 1000
	maxQuietMachineHours = 24 * 30
)

// QuietSettings configure the quiet alerts. Zero turns either one off.
type QuietSettings struct {
	// SlotCans: alert when a stocked slot registers nothing while the other
	// slots dispense this many cans.
	SlotCans int `json:"quietSlotCans" example:"15"`
	// MachineHours: alert when no slot registers anything for this many
	// hours while the Pi agent is online.
	MachineHours int `json:"quietMachineHours" example:"24"`
	// WeekdaysOnly counts only Monday–Friday hours toward MachineHours, so
	// a quiet weekend doesn't alert.
	WeekdaysOnly bool `json:"quietMachineWeekdaysOnly" example:"true"`
	// Timezone (IANA) decides where weekends start and end.
	Timezone string `json:"timezone" example:"America/Chicago"`
}

func (s *Service) GetQuietSettings(ctx context.Context) (QuietSettings, error) {
	get := func(key string) (string, error) { return s.store.GetSetting(ctx, key) }
	q := QuietSettings{
		SlotCans:     defaultQuietSlotCans,
		MachineHours: defaultQuietMachineHours,
		WeekdaysOnly: true,
		Timezone:     defaultAlertTimezone,
	}
	for _, f := range []struct {
		key string
		set func(string)
	}{
		{keyQuietSlotCans, func(v string) {
			if n, err := strconv.Atoi(v); err == nil {
				q.SlotCans = n
			}
		}},
		{keyQuietMachineHours, func(v string) {
			if n, err := strconv.Atoi(v); err == nil {
				q.MachineHours = n
			}
		}},
		{keyQuietWeekdaysOnly, func(v string) { q.WeekdaysOnly = v != "0" }},
		{keyAlertTimezone, func(v string) { q.Timezone = v }},
	} {
		v, err := get(f.key)
		if err != nil {
			return q, err
		}
		if v != "" {
			f.set(v)
		}
	}
	return q, nil
}

// ErrInvalidSettings wraps a rejected SetQuietSettings.
type ErrInvalidSettings struct{ Msg string }

func (e *ErrInvalidSettings) Error() string { return e.Msg }

// SetQuietSettings validates and stores q, then re-evaluates every machine
// so the change takes effect now.
func (s *Service) SetQuietSettings(ctx context.Context, q QuietSettings) (QuietSettings, error) {
	if q.SlotCans < 0 || q.SlotCans > maxQuietSlotCans {
		return q, &ErrInvalidSettings{fmt.Sprintf("quietSlotCans must be 0 (off) to %d", maxQuietSlotCans)}
	}
	if q.MachineHours < 0 || q.MachineHours > maxQuietMachineHours {
		return q, &ErrInvalidSettings{fmt.Sprintf("quietMachineHours must be 0 (off) to %d", maxQuietMachineHours)}
	}
	if q.Timezone == "" {
		q.Timezone = defaultAlertTimezone
	}
	if _, err := time.LoadLocation(q.Timezone); err != nil {
		return q, &ErrInvalidSettings{"unknown timezone " + strconv.Quote(q.Timezone)}
	}
	weekdays := "0"
	if q.WeekdaysOnly {
		weekdays = "1"
	}
	for _, kv := range [][2]string{
		{keyQuietSlotCans, strconv.Itoa(q.SlotCans)},
		{keyQuietMachineHours, strconv.Itoa(q.MachineHours)},
		{keyQuietWeekdaysOnly, weekdays},
		{keyAlertTimezone, q.Timezone},
	} {
		if err := s.store.SetSetting(ctx, kv[0], kv[1], false); err != nil {
			return q, err
		}
	}
	machines, err := s.store.ListMachines(ctx)
	if err != nil {
		return q, err
	}
	for _, m := range machines {
		if err := s.EvaluateQuiet(ctx, m.ID); err != nil {
			slog.Warn("evaluate quiet alerts", "machine", m.ID, "error", err)
		}
	}
	return q, nil
}

// RunQuietChecks evaluates every machine every interval until ctx ends.
// Dispenses re-evaluate immediately; this is what notices time passing.
func (s *Service) RunQuietChecks(ctx context.Context, every time.Duration) {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		machines, err := s.store.ListMachines(ctx)
		if err != nil {
			slog.Error("quiet checks: list machines", "error", err)
		}
		for _, m := range machines {
			if err := s.EvaluateQuiet(ctx, m.ID); err != nil {
				slog.Error("quiet checks", "machine", m.ID, "error", err)
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// EvaluateQuiet raises and clears a machine's quiet alerts.
func (s *Service) EvaluateQuiet(ctx context.Context, machineID string) error {
	return s.evaluateQuietAt(ctx, machineID, s.now().UTC())
}

func (s *Service) evaluateQuietAt(ctx context.Context, machineID string, now time.Time) error {
	q, err := s.GetQuietSettings(ctx)
	if err != nil {
		return err
	}
	status, err := s.GetMachineStatus(ctx, machineID)
	if err != nil || status == nil {
		return err
	}
	dispenses, err := s.store.ListDispenses(ctx, machineID, time.Time{}, 1_000_000)
	if err != nil {
		return err
	}
	lastBySlot := map[int]time.Time{}
	var lastAny time.Time
	for _, d := range dispenses {
		if d.Timestamp.After(lastBySlot[d.SlotPosition]) {
			lastBySlot[d.SlotPosition] = d.Timestamp
		}
		if d.Timestamp.After(lastAny) {
			lastAny = d.Timestamp
		}
	}

	var lastRestockAny time.Time
	for _, ss := range status.Slots {
		pos := ss.Slot.Position
		restock, _, err := s.store.GetLastRestock(ctx, machineID, pos)
		if err != nil {
			return err
		}
		var restockedAt time.Time
		if restock != nil {
			restockedAt = restock.Timestamp
			if restockedAt.After(lastRestockAny) {
				lastRestockAny = restockedAt
			}
		}
		if err := s.evaluateQuietSlot(ctx, q, machineID, ss, lastBySlot[pos], restockedAt, dispenses, now); err != nil {
			return err
		}
	}
	return s.evaluateQuietMachine(ctx, q, status, lastAny, lastRestockAny, now)
}

func (s *Service) evaluateQuietSlot(ctx context.Context, q QuietSettings, machineID string, ss domain.SlotStatus,
	lastDispense, restockedAt time.Time, dispenses []domain.Dispense, now time.Time) error {
	pos := ss.Slot.Position
	active, err := s.store.GetActiveAlert(ctx, machineID, pos, AlertQuietSlot)
	if err != nil {
		return err
	}
	if active != nil {
		// Only the slot itself registering again clears it (or turning the
		// alert off, or emptying the slot's product). A restock doesn't:
		// that resets the stock, not the sensor.
		switch {
		case lastDispense.After(active.Timestamp):
			return s.clearQuiet(ctx, active, true)
		case q.SlotCans == 0 || ss.Slot.ProductID == "":
			return s.clearQuiet(ctx, active, false)
		}
		return nil
	}
	// A slot that can't vend isn't quiet: no product, or sold out.
	if q.SlotCans == 0 || ss.Slot.ProductID == "" || ss.CurrentStock <= 0 {
		return nil
	}
	// Count from the slot's last dispense or restock, whichever is later.
	// Before a restock the slot may have been sold out.
	since := lastDispense
	if restockedAt.After(since) {
		since = restockedAt
	}
	if since.IsZero() {
		return nil // never stocked or used: nothing to compare against
	}
	others := 0
	for _, d := range dispenses {
		if d.SlotPosition != pos && d.Timestamp.After(since) {
			others++
		}
	}
	if others < q.SlotCans {
		return nil
	}
	ae := &domain.AlertEvent{
		ID: uuid.NewString(), MachineID: machineID, SlotPosition: pos, ProductID: ss.Slot.ProductID,
		AlertType: AlertQuietSlot, CurrentStock: others, Threshold: q.SlotCans, Timestamp: now,
	}
	if err := s.store.CreateAlertEvent(ctx, ae); err != nil {
		return err
	}
	slog.Warn("slot is quiet", "machine", machineID, "slot", pos, "otherDispenses", others, "since", since)
	s.publish(ctx, events.EventQuietSlot, now, map[string]any{
		"machineId": machineID, "slotPosition": pos, "productId": ss.Slot.ProductID,
		"otherDispenses": others, "since": since.UTC().Format(time.RFC3339),
	})
	return nil
}

func (s *Service) evaluateQuietMachine(ctx context.Context, q QuietSettings, status *domain.MachineStatus,
	lastDispense, lastRestock time.Time, now time.Time) error {
	machineID := status.Machine.ID
	active, err := s.store.GetActiveAlert(ctx, machineID, 0, AlertQuietMachine)
	if err != nil {
		return err
	}
	if active != nil {
		switch {
		case lastDispense.After(active.Timestamp):
			return s.clearQuiet(ctx, active, true)
		case q.MachineHours == 0:
			return s.clearQuiet(ctx, active, false)
		}
		return nil
	}
	// With the agent offline, silence is expected; Uptime covers that.
	if q.MachineHours == 0 || !status.Agent.Online {
		return nil
	}
	since := lastDispense
	if lastRestock.After(since) {
		since = lastRestock
	}
	if since.IsZero() {
		return nil
	}
	loc, err := time.LoadLocation(q.Timezone)
	if err != nil {
		loc = time.UTC
	}
	quiet := now.Sub(since)
	if q.WeekdaysOnly {
		quiet = weekdayDuration(since, now, loc)
	}
	if quiet < time.Duration(q.MachineHours)*time.Hour {
		return nil
	}
	ae := &domain.AlertEvent{
		ID: uuid.NewString(), MachineID: machineID, SlotPosition: 0,
		AlertType: AlertQuietMachine, CurrentStock: int(quiet / time.Hour), Threshold: q.MachineHours, Timestamp: now,
	}
	if err := s.store.CreateAlertEvent(ctx, ae); err != nil {
		return err
	}
	slog.Warn("machine is quiet", "machine", machineID, "quietHours", int(quiet/time.Hour), "since", since)
	data := map[string]any{
		"machineId": machineID, "slotPosition": 0, "quietHours": int(quiet / time.Hour),
		"weekdaysOnly": q.WeekdaysOnly, "since": since.UTC().Format(time.RFC3339),
	}
	s.publish(ctx, events.EventQuietMachine, now, data)
	return nil
}

// clearQuiet clears a quiet alert. Only a real recovery (the slot or machine
// registering again) is announced; a settings change clears it silently.
func (s *Service) clearQuiet(ctx context.Context, ae *domain.AlertEvent, recovered bool) error {
	if err := s.store.ClearAlert(ctx, ae.ID); err != nil {
		return err
	}
	if recovered {
		s.publish(ctx, events.EventQuietCleared, s.now().UTC(), map[string]any{
			"machineId": ae.MachineID, "slotPosition": ae.SlotPosition, "productId": ae.ProductID,
			"alertType": ae.AlertType,
		})
	}
	return nil
}

func (s *Service) publish(ctx context.Context, t events.EventType, at time.Time, data map[string]any) {
	if s.bus == nil {
		return
	}
	s.bus.Publish(ctx, events.Event{Type: t, Timestamp: at.UTC().Format(time.RFC3339), Data: data})
}

// weekdayDuration is how much of [from, to) falls on Monday–Friday in loc.
func weekdayDuration(from, to time.Time, loc *time.Location) time.Duration {
	var total time.Duration
	for t := from.In(loc); t.Before(to); {
		y, m, d := t.Date()
		next := time.Date(y, m, d+1, 0, 0, 0, 0, loc)
		end := next
		if to.Before(end) {
			end = to.In(loc)
		}
		if wd := t.Weekday(); wd != time.Saturday && wd != time.Sunday {
			total += end.Sub(t)
		}
		t = next
	}
	return total
}
