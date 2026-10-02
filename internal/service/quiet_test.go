package service

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/jeffwnelson/sodatracker/internal/domain"
	"github.com/jeffwnelson/sodatracker/internal/events"
)

type eventLog struct {
	mu  sync.Mutex
	evs []events.Event
}

func (l *eventLog) record(bus events.EventBus, types ...events.EventType) {
	for _, t := range types {
		bus.Subscribe(t, func(_ context.Context, e events.Event) error {
			l.mu.Lock()
			defer l.mu.Unlock()
			l.evs = append(l.evs, e)
			return nil
		})
	}
}

func (l *eventLog) types() []events.EventType {
	l.mu.Lock()
	defer l.mu.Unlock()
	var out []events.EventType
	for _, e := range l.evs {
		out = append(out, e.Type)
	}
	return out
}

// quietMachine sets up a three-slot machine restocked at restockAt.
func quietMachine(t *testing.T, restockAt time.Time, qty int) (*Service, *eventLog, func(slot int, at time.Time)) {
	t.Helper()
	svc, st, bus := setupServiceWithBus(t)
	ctx := context.Background()
	st.CreateMachine(ctx, &domain.Machine{ID: "m1", Name: "Office", SlotCount: 3})
	var slots []domain.Slot
	for i := 1; i <= 3; i++ {
		st.CreateProduct(ctx, &domain.Product{ID: fmt.Sprintf("p%d", i), Name: fmt.Sprintf("Soda %d", i)})
		slots = append(slots, domain.Slot{MachineID: "m1", Position: i, ProductID: fmt.Sprintf("p%d", i), Capacity: 36, LowThreshold: 3})
	}
	st.SetSlots(ctx, "m1", slots)
	svc.now = func() time.Time { return restockAt }
	svc.CreateRestock(ctx, &domain.Restock{ID: "r1", MachineID: "m1", Timestamp: restockAt,
		Slots: []domain.RestockSlot{{Position: 1, Quantity: 36}, {Position: 2, Quantity: 36}, {Position: 3, Quantity: qty}}})
	log := &eventLog{}
	log.record(bus, events.EventQuietSlot, events.EventQuietMachine, events.EventQuietCleared)
	n := 0
	dispense := func(slot int, at time.Time) {
		n++
		st.CreateDispense(ctx, &domain.Dispense{ID: fmt.Sprintf("d%d", n), MachineID: "m1",
			ProductID: fmt.Sprintf("p%d", slot), SlotPosition: slot, SourceType: "gpio", Timestamp: at})
	}
	return svc, log, dispense
}

func activeQuiet(t *testing.T, svc *Service) map[string][]int {
	t.Helper()
	aes, err := svc.ListAlertEvents(context.Background(), "m1", true, 50)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string][]int{}
	for _, ae := range aes {
		if ae.AlertType == AlertQuietSlot || ae.AlertType == AlertQuietMachine {
			out[ae.AlertType] = append(out[ae.AlertType], ae.SlotPosition)
		}
	}
	return out
}

func TestQuietSlot(t *testing.T) {
	ctx := context.Background()
	t0 := time.Date(2026, 9, 30, 13, 0, 0, 0, time.UTC)
	svc, log, dispense := quietMachine(t, t0, 36)
	svc.SetQuietSettings(ctx, QuietSettings{SlotCans: 15, MachineHours: 0})

	// Slots 1 and 2 alternate: each sees fewer than 15 others since its own
	// last dispense. Slot 3 sees all of them.
	at := t0
	for i := 0; i < 14; i++ {
		at = at.Add(10 * time.Minute)
		dispense(1+i%2, at)
	}
	svc.now = func() time.Time { return at }
	svc.EvaluateQuiet(ctx, "m1")
	if got := activeQuiet(t, svc); len(got) != 0 {
		t.Fatalf("14 cans elsewhere already alerted: %v", got)
	}
	at = at.Add(10 * time.Minute)
	dispense(1, at)
	svc.EvaluateQuiet(ctx, "m1")
	if got := activeQuiet(t, svc)[AlertQuietSlot]; len(got) != 1 || got[0] != 3 {
		t.Fatalf("quiet slots = %v, want [3]", got)
	}
	// Further checks don't repeat it.
	svc.EvaluateQuiet(ctx, "m1")
	if got := log.types(); len(got) != 1 || got[0] != events.EventQuietSlot {
		t.Fatalf("events = %v", got)
	}

	// A restock doesn't clear it: that resets stock, not the sensor.
	at = at.Add(time.Hour)
	svc.now = func() time.Time { return at }
	svc.CreateRestock(ctx, &domain.Restock{ID: "r2", MachineID: "m1", Timestamp: at,
		Slots: []domain.RestockSlot{{Position: 3, Quantity: 36}}})
	svc.EvaluateQuiet(ctx, "m1")
	if got := activeQuiet(t, svc)[AlertQuietSlot]; len(got) != 1 {
		t.Fatalf("restock cleared the quiet alert: %v", got)
	}

	// The slot registering again clears it, and says so.
	at = at.Add(time.Minute)
	dispense(3, at)
	svc.EvaluateQuiet(ctx, "m1")
	if got := activeQuiet(t, svc)[AlertQuietSlot]; len(got) != 0 {
		t.Fatalf("still quiet after a dispense: %v", got)
	}
	if got := log.types(); len(got) != 2 || got[1] != events.EventQuietCleared {
		t.Fatalf("events = %v", got)
	}
}

func TestQuietSlotIgnoresSoldOutSlot(t *testing.T) {
	ctx := context.Background()
	t0 := time.Date(2026, 9, 30, 13, 0, 0, 0, time.UTC)
	svc, _, dispense := quietMachine(t, t0, 0) // slot 3 restocked empty
	svc.SetQuietSettings(ctx, QuietSettings{SlotCans: 5, MachineHours: 0})
	at := t0
	for i := 0; i < 20; i++ {
		at = at.Add(time.Minute)
		dispense(1+i%2, at)
	}
	svc.now = func() time.Time { return at }
	svc.EvaluateQuiet(ctx, "m1")
	if got := activeQuiet(t, svc); len(got) != 0 {
		t.Fatalf("sold-out slot alerted: %v", got)
	}
}

func TestQuietMachine(t *testing.T) {
	ctx := context.Background()
	chicago, _ := time.LoadLocation("America/Chicago")
	friday := time.Date(2026, 10, 2, 16, 0, 0, 0, chicago) // a Friday
	svc, log, dispense := quietMachine(t, friday, 36)
	svc.SetQuietSettings(ctx, QuietSettings{SlotCans: 0, MachineHours: 24, WeekdaysOnly: true, Timezone: "America/Chicago"})

	check := func(at time.Time) {
		svc.now = func() time.Time { return at }
		if err := svc.RecordAgentHeartbeat(ctx, "m1"); err != nil {
			t.Fatal(err)
		}
		if err := svc.EvaluateQuiet(ctx, "m1"); err != nil {
			t.Fatal(err)
		}
	}

	// Friday 16:00 to Monday 10:00 is 18 weekday hours: no alert.
	check(time.Date(2026, 10, 5, 10, 0, 0, 0, chicago))
	if got := activeQuiet(t, svc); len(got) != 0 {
		t.Fatalf("weekend alerted: %v", got)
	}
	// Tuesday 10:00 is 42.
	check(time.Date(2026, 10, 6, 10, 0, 0, 0, chicago))
	if got := activeQuiet(t, svc)[AlertQuietMachine]; len(got) != 1 || got[0] != 0 {
		t.Fatalf("quiet machine = %v", got)
	}
	if got := log.types(); len(got) != 1 || got[0] != events.EventQuietMachine || log.evs[0].Data["quietHours"] != 42 {
		t.Fatalf("events = %v %v", got, log.evs)
	}

	// Any dispense clears it.
	dispense(2, time.Date(2026, 10, 6, 11, 0, 0, 0, chicago))
	check(time.Date(2026, 10, 6, 11, 1, 0, 0, chicago))
	if got := activeQuiet(t, svc); len(got) != 0 {
		t.Fatalf("still quiet: %v", got)
	}
	if got := log.types(); len(got) != 2 || got[1] != events.EventQuietCleared {
		t.Fatalf("events = %v", got)
	}
}

func TestQuietMachineNeedsAgentOnline(t *testing.T) {
	ctx := context.Background()
	t0 := time.Date(2026, 10, 1, 13, 0, 0, 0, time.UTC)
	svc, _, _ := quietMachine(t, t0, 36)
	svc.SetQuietSettings(ctx, QuietSettings{MachineHours: 1})
	svc.now = func() time.Time { return t0.Add(48 * time.Hour) }
	svc.EvaluateQuiet(ctx, "m1") // no heartbeat ever: agent offline
	if got := activeQuiet(t, svc); len(got) != 0 {
		t.Fatalf("alerted with the agent offline: %v", got)
	}
}

func TestQuietSettingsValidation(t *testing.T) {
	svc, _ := setupService(t)
	ctx := context.Background()
	q, err := svc.GetQuietSettings(ctx)
	if err != nil || q.SlotCans != 15 || q.MachineHours != 24 || !q.WeekdaysOnly || q.Timezone != "America/Chicago" {
		t.Fatalf("defaults = %+v, %v", q, err)
	}
	for _, bad := range []QuietSettings{{SlotCans: -1}, {MachineHours: 99999}, {Timezone: "Mars/Olympus"}} {
		if _, err := svc.SetQuietSettings(ctx, bad); err == nil {
			t.Errorf("accepted %+v", bad)
		}
	}
	if _, err := svc.SetQuietSettings(ctx, QuietSettings{SlotCans: 0, MachineHours: 8, WeekdaysOnly: false, Timezone: "UTC"}); err != nil {
		t.Fatal(err)
	}
	if q, _ := svc.GetQuietSettings(ctx); q.SlotCans != 0 || q.MachineHours != 8 || q.WeekdaysOnly || q.Timezone != "UTC" {
		t.Fatalf("stored = %+v", q)
	}
}

func TestWeekdayDuration(t *testing.T) {
	chicago, _ := time.LoadLocation("America/Chicago")
	fri := time.Date(2026, 10, 2, 16, 0, 0, 0, chicago)
	mon := time.Date(2026, 10, 5, 10, 0, 0, 0, chicago)
	if got := weekdayDuration(fri, mon, chicago); got != 18*time.Hour {
		t.Errorf("Fri 16:00 → Mon 10:00 = %v, want 18h", got)
	}
	sat := time.Date(2026, 10, 3, 9, 0, 0, 0, chicago)
	if got := weekdayDuration(sat, sat.Add(5*time.Hour), chicago); got != 0 {
		t.Errorf("within Saturday = %v", got)
	}
}
