package service

import (
	"context"
	"testing"
	"time"

	"github.com/jeffwnelson/sodatracker/internal/domain"
	"github.com/jeffwnelson/sodatracker/internal/store/sqlite"
)

func addSession(t *testing.T, st *sqlite.SQLiteStore, from, last time.Time) {
	t.Helper()
	ctx := context.Background()
	if err := st.StartAgentSession(ctx, "m1", from); err != nil {
		t.Fatal(err)
	}
	s, err := st.LatestAgentSession(ctx, "m1")
	if err != nil || s == nil {
		t.Fatalf("latest session: %v", err)
	}
	if err := st.TouchAgentSession(ctx, s.ID, last); err != nil {
		t.Fatal(err)
	}
}

func TestUptimeAnalytics(t *testing.T) {
	svc, st := setupService(t)
	ctx := context.Background()
	st.CreateMachine(ctx, &domain.Machine{ID: "m1", Name: "Test", SlotCount: 1})

	from := time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC)
	to := from.Add(48 * time.Hour)
	q := AnalyticsQuery{MachineID: "m1", From: from, To: to, Location: time.UTC}

	u, err := svc.GetUptimeAnalytics(ctx, q)
	if err != nil {
		t.Fatal(err)
	}
	if u.Uptime != nil || u.UntrackedHours != 48 || u.TrackingSince != nil {
		t.Fatalf("no heartbeats: %+v", u)
	}

	// Two minutes of heartbeats, still running: 100%, though the hours
	// round to zero.
	addSession(t, st, to.Add(-2*time.Minute), to.Add(-30*time.Second))
	if u, _ = svc.GetUptimeAnalytics(ctx, q); u.Uptime == nil || *u.Uptime != 1 {
		t.Fatalf("two minutes tracked: uptime %v", u.Uptime)
	}
	st.DeleteMachine(ctx, "m1")
	st.CreateMachine(ctx, &domain.Machine{ID: "m1", Name: "Test", SlotCount: 1})

	// Online 06:00–12:00, an hour's outage, then online through the end
	// (last beat 30s before it, so the session is still running).
	addSession(t, st, from.Add(6*time.Hour), from.Add(12*time.Hour))
	addSession(t, st, from.Add(13*time.Hour), to.Add(-30*time.Second))

	u, err = svc.GetUptimeAnalytics(ctx, q)
	if err != nil {
		t.Fatal(err)
	}
	if u.OnlineHours != 41 || u.OfflineHours != 1 || u.UntrackedHours != 6 {
		t.Errorf("hours online/offline/untracked = %v/%v/%v, want 41/1/6", u.OnlineHours, u.OfflineHours, u.UntrackedHours)
	}
	if u.Uptime == nil || *u.Uptime != 0.976 {
		t.Errorf("uptime = %v, want 0.976", u.Uptime)
	}
	if u.Outages != 1 || u.LongestOutageHours != 1 || len(u.OutageList) != 1 || u.OutageList[0].Ongoing {
		t.Errorf("outages = %d longest %v list %+v", u.Outages, u.LongestOutageHours, u.OutageList)
	}
	if len(u.Series) != 2 {
		t.Fatalf("series = %+v", u.Series)
	}
	if p := u.Series[0]; p.OnlineHours != 17 || p.OfflineHours != 1 || p.UntrackedHours != 6 {
		t.Errorf("day 1 = %+v", p)
	}
	if p := u.Series[1]; p.OnlineHours != 24 || p.OfflineHours != 0 {
		t.Errorf("day 2 = %+v", p)
	}

	// Seen from four hours later, the agent has been silent since: an
	// ongoing outage at the end.
	q.To = to.Add(4 * time.Hour)
	u, err = svc.GetUptimeAnalytics(ctx, q)
	if err != nil {
		t.Fatal(err)
	}
	if u.Outages != 2 || !u.OutageList[0].Ongoing || u.OutageList[0].Hours != 4.01 {
		t.Errorf("trailing outage: %+v", u.OutageList)
	}
}

func TestHighlightsAnalytics(t *testing.T) {
	svc, st := setupService(t)
	ctx := context.Background()
	st.CreateMachine(ctx, &domain.Machine{ID: "m1", Name: "Test", SlotCount: 2})
	st.CreateProduct(ctx, &domain.Product{ID: "p1", Name: "Cola", SizeOz: 12, CaffeineMg: 34})
	st.CreateProduct(ctx, &domain.Product{ID: "p2", Name: "Diet", CaffeineMg: 46}) // size unset: 12

	// Thu 5 – Wed 11 March 2026, UTC.
	from := time.Date(2026, 3, 5, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 3, 11, 12, 0, 0, 0, time.UTC)
	at := func(day, h, m int) time.Time { return time.Date(2026, 3, day, h, m, 0, 0, time.UTC) }
	addDispense(t, st, "p1", 1, at(5, 8, 0)) // Thu: first 8:00
	addDispense(t, st, "p2", 2, at(5, 15, 0))
	addDispense(t, st, "p2", 2, at(6, 7, 0)) // Fri: first 7:00 (record)
	addDispense(t, st, "p1", 1, at(6, 14, 5))
	addDispense(t, st, "p1", 1, at(6, 14, 40))
	addDispense(t, st, "p1", 1, at(6, 18, 0)) // latest last can
	// Weekend and Monday empty; Monday breaks the streak.
	addDispense(t, st, "p1", 1, at(10, 9, 0))                                 // Tue
	addDispense(t, st, "p1", 1, at(11, 9, 0))                                 // Wed (today)
	addDispense(t, st, "p1", 1, time.Date(2026, 2, 2, 6, 30, 0, 0, time.UTC)) // long before the window

	h, err := svc.GetHighlightsAnalytics(ctx, AnalyticsQuery{MachineID: "m1", From: from, To: to, Location: time.UTC})
	if err != nil {
		t.Fatal(err)
	}
	// First cans 8:00, 7:00, 9:00, 9:00 → 8:15. Last cans 15:00, 18:00, 9:00, 9:00 → 12:45.
	if h.FirstCan.AvgMinute != 8*60+15 || h.FirstCan.Record != "2026-03-06T07:00:00Z" {
		t.Errorf("first can %+v", h.FirstCan)
	}
	if h.LastCan.AvgMinute != 12*60+45 || h.LastCan.Record != "2026-03-06T18:00:00Z" {
		t.Errorf("last can %+v", h.LastCan)
	}
	if h.BusiestDay.At != "2026-03-06T00:00:00Z" || h.BusiestDay.Count != 4 {
		t.Errorf("busiest day %+v", h.BusiestDay)
	}
	if h.BusiestHour.At != "2026-03-06T14:00:00Z" || h.BusiestHour.Count != 2 {
		t.Errorf("busiest hour %+v", h.BusiestHour)
	}
	// 6 Cola + 2 Diet, all 12 oz.
	if h.Ounces != 96 || h.CaffeineMg != 6*34+2*46 || h.AluminumLb != 0.25 {
		t.Errorf("totals oz %v caffeine %v aluminum %v", h.Ounces, h.CaffeineMg, h.AluminumLb)
	}
	// The earliest and latest cans stay inside the window: the 06:30 can
	// in February doesn't count.
	if h.FirstCan.Record != "2026-03-06T07:00:00Z" || h.LastCan.Record != "2026-03-06T18:00:00Z" {
		t.Errorf("earliest %s latest %s", h.FirstCan.Record, h.LastCan.Record)
	}
	if len(h.FirstCanProducts) != 2 || h.FirstCanProducts[0].Name != "Cola" || h.FirstCanProducts[0].Count != 3 {
		t.Errorf("first can products %+v", h.FirstCanProducts)
	}
	// Last cans: Thu Diet, then Cola Fri, Tue and Wed.
	if len(h.LastCanProducts) != 2 || h.LastCanProducts[0].Name != "Cola" || h.LastCanProducts[0].Count != 3 ||
		h.LastCanProducts[1].Name != "Diet" || h.LastCanProducts[1].Count != 1 {
		t.Errorf("last can products %+v", h.LastCanProducts)
	}
	// Nothing went between 11:30 and 13:00 (14:05 is after lunch).
	if len(h.LunchProducts) != 0 {
		t.Errorf("lunch products %+v", h.LunchProducts)
	}

}

func TestHighlightsLunch(t *testing.T) {
	svc, st := setupService(t)
	ctx := context.Background()
	st.CreateMachine(ctx, &domain.Machine{ID: "m1", Name: "Test", SlotCount: 2})
	st.CreateProduct(ctx, &domain.Product{ID: "p1", Name: "Cola"})
	st.CreateProduct(ctx, &domain.Product{ID: "p2", Name: "Diet"})
	chicago, _ := time.LoadLocation("America/Chicago")
	at := func(h, m int) time.Time { return time.Date(2026, 3, 5, h, m, 0, 0, chicago) }
	addDispense(t, st, "p1", 1, at(11, 29)) // before lunch
	addDispense(t, st, "p2", 2, at(11, 30))
	addDispense(t, st, "p2", 2, at(12, 15))
	addDispense(t, st, "p1", 1, at(12, 59))
	addDispense(t, st, "p1", 1, at(13, 0)) // after lunch

	h, err := svc.GetHighlightsAnalytics(ctx, AnalyticsQuery{MachineID: "m1",
		From: at(0, 0), To: at(23, 0), Location: chicago})
	if err != nil {
		t.Fatal(err)
	}
	// Lunch is local time: 11:30 to 12:59 in Chicago.
	if len(h.LunchProducts) != 2 || h.LunchProducts[0].Name != "Diet" || h.LunchProducts[0].Count != 2 ||
		h.LunchProducts[1].Name != "Cola" || h.LunchProducts[1].Count != 1 {
		t.Errorf("lunch products %+v", h.LunchProducts)
	}
}
