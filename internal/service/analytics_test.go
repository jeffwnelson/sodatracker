package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jeffwnelson/sodatracker/internal/domain"
	"github.com/jeffwnelson/sodatracker/internal/store/sqlite"
)

func addDispense(t *testing.T, st *sqlite.SQLiteStore, productID string, slot int, at time.Time) {
	t.Helper()
	err := st.CreateDispense(context.Background(), &domain.Dispense{
		ID: fmt.Sprintf("d-%d-%d", slot, at.UnixNano()), MachineID: "m1", ProductID: productID,
		SlotPosition: slot, SourceType: "gpio", Timestamp: at.UTC(),
	})
	if err != nil {
		t.Fatalf("create dispense: %v", err)
	}
}

func addRestock(t *testing.T, st *sqlite.SQLiteStore, id string, at time.Time, slots ...domain.RestockSlot) {
	t.Helper()
	err := st.CreateRestock(context.Background(), &domain.Restock{ID: id, MachineID: "m1", Slots: slots, Timestamp: at.UTC()})
	if err != nil {
		t.Fatalf("create restock: %v", err)
	}
}

func TestConsumptionAnalytics(t *testing.T) {
	svc, st := setupService(t)
	ctx := context.Background()
	chicago, err := time.LoadLocation("America/Chicago")
	if err != nil {
		t.Fatal(err)
	}

	st.CreateMachine(ctx, &domain.Machine{ID: "m1", Name: "Test", SlotCount: 1})
	st.CreateProduct(ctx, &domain.Product{ID: "p1", Name: "Cola"})
	st.SetSlots(ctx, "m1", []domain.Slot{{MachineID: "m1", Position: 1, ProductID: "p1", Capacity: 10}})

	// Monday 2 March to Monday 9 March 2026, which spans the US DST change.
	from := time.Date(2026, 3, 2, 0, 0, 0, 0, chicago)
	to := time.Date(2026, 3, 9, 0, 0, 0, 0, chicago)
	// The previous window has the same length, 167h across the DST change.
	addDispense(t, st, "p1", 1, from.Add(-to.Sub(from))) // first instant of the previous window
	addDispense(t, st, "p1", 1, from.AddDate(0, 0, -3))  // previous window
	for i := range 3 {
		addDispense(t, st, "p1", 1, time.Date(2026, 3, 2, 14, i*10, 0, 0, chicago)) // Mon 2pm
	}
	addDispense(t, st, "p1", 1, time.Date(2026, 3, 7, 10, 0, 0, 0, chicago)) // Sat 10am
	addDispense(t, st, "p1", 1, to.Add(time.Hour))                           // after the window

	c, err := svc.GetConsumptionAnalytics(ctx, AnalyticsQuery{MachineID: "m1", From: from, To: to, Location: chicago})
	if err != nil {
		t.Fatalf("consumption: %v", err)
	}
	if c.Total != 4 {
		t.Errorf("total = %d, want 4", c.Total)
	}
	if c.PreviousTotal == nil || *c.PreviousTotal != 2 {
		t.Errorf("previousTotal = %v, want 2", c.PreviousTotal)
	}
	if c.Range.Bucket != BucketDay || len(c.Series) != 7 {
		t.Fatalf("bucket %q with %d points, want day with 7", c.Range.Bucket, len(c.Series))
	}
	if c.Series[6].Bucket != "2026-03-08T00:00:00-06:00" {
		t.Errorf("DST day bucket = %s", c.Series[6].Bucket)
	}
	if c.Series[0].Count != 3 || c.Series[5].Count != 1 {
		t.Errorf("series counts = %+v", c.Series)
	}
	if c.Heatmap[0][14] != 3 || c.Heatmap[5][10] != 1 {
		t.Errorf("heatmap Mon 14h = %d, Sat 10h = %d", c.Heatmap[0][14], c.Heatmap[5][10])
	}
	if c.PeakHour != 14 || c.PeakWeekday != 0 {
		t.Errorf("peak hour %d weekday %d, want 14 and 0", c.PeakHour, c.PeakWeekday)
	}
	if c.WeekdayPerDay != 0.6 || c.WeekendPerDay != 0.5 {
		t.Errorf("weekday %.2f weekend %.2f per day, want 0.6 and 0.5", c.WeekdayPerDay, c.WeekendPerDay)
	}
}

func TestAnalyticsClampsToTrackingStart(t *testing.T) {
	svc, st := setupService(t)
	ctx := context.Background()
	st.CreateMachine(ctx, &domain.Machine{ID: "m1", Name: "Test", SlotCount: 1})

	first := time.Date(2026, 3, 10, 9, 0, 0, 0, time.UTC)
	addDispense(t, st, "p1", 1, first)
	to := first.AddDate(0, 0, 4)

	c, err := svc.GetConsumptionAnalytics(ctx, AnalyticsQuery{MachineID: "m1", From: first.AddDate(-1, 0, 0), To: to})
	if err != nil {
		t.Fatalf("consumption: %v", err)
	}
	if c.Range.From != "2026-03-10T00:00:00Z" {
		t.Errorf("from = %s, want clamped to 2026-03-10", c.Range.From)
	}
	if c.PreviousTotal != nil {
		t.Errorf("previousTotal = %d, want null before tracking began", *c.PreviousTotal)
	}
	// Tracking began two days before a 4-day window: the previous window is
	// only half covered, so no comparison.
	c, err = svc.GetConsumptionAnalytics(ctx, AnalyticsQuery{MachineID: "m1", From: first.AddDate(0, 0, 2), To: first.AddDate(0, 0, 6)})
	if err != nil {
		t.Fatalf("consumption: %v", err)
	}
	if c.PreviousTotal != nil {
		t.Errorf("previousTotal = %d, want null for a partly tracked previous window", *c.PreviousTotal)
	}
	if c.Range.Bucket != BucketDay {
		t.Errorf("bucket = %s, want day for a 4.4 day window", c.Range.Bucket)
	}
}

func TestProductAnalytics(t *testing.T) {
	svc, st := setupService(t)
	ctx := context.Background()
	st.CreateMachine(ctx, &domain.Machine{ID: "m1", Name: "Test", SlotCount: 3})
	st.CreateProduct(ctx, &domain.Product{ID: "a", Name: "Apple Soda", CostPerCan: 0.4})
	st.CreateProduct(ctx, &domain.Product{ID: "b", Name: "Berry Soda", CostPerCan: 1.0})
	st.CreateProduct(ctx, &domain.Product{ID: "c", Name: "Cherry Soda", CostPerCan: 0.5})
	st.SetSlots(ctx, "m1", []domain.Slot{
		{MachineID: "m1", Position: 1, ProductID: "a", Capacity: 10},
		{MachineID: "m1", Position: 2, ProductID: "b", Capacity: 10},
		{MachineID: "m1", Position: 3, ProductID: "c", Capacity: 10},
	})

	from := time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC)
	to := from.AddDate(0, 0, 7)
	for i := range 3 {
		addDispense(t, st, "b", 2, from.Add(-time.Duration(i+1)*24*time.Hour)) // previous window
	}
	addRestock(t, st, "r0", from.AddDate(0, 0, -7))         // tracking covers the whole previous window
	addDispense(t, st, "a", 1, from.Add(8*time.Hour))       // Morning
	addDispense(t, st, "a", 1, from.Add(15*time.Hour))      // Afternoon
	addDispense(t, st, "orphan", 2, from.Add(12*time.Hour)) // resolves to b via slot 2, Lunch

	p, err := svc.GetProductAnalytics(ctx, AnalyticsQuery{MachineID: "m1", From: from, To: to})
	if err != nil {
		t.Fatalf("products: %v", err)
	}
	if len(p.Products) != 3 {
		t.Fatalf("got %d products, want 3 including the unsold one", len(p.Products))
	}
	a, b, c := p.Products[0], p.Products[1], p.Products[2]
	if a.ProductID != "a" || a.Count != 2 || a.Rank != 1 || a.Cost != 0.8 {
		t.Errorf("first = %+v", a)
	}
	if a.PreviousRank != nil || a.PreviousCount == nil || *a.PreviousCount != 0 {
		t.Errorf("a previous rank/count = %v/%v, want nil/0", a.PreviousRank, a.PreviousCount)
	}
	if b.ProductID != "b" || b.Count != 1 || b.PreviousRank == nil || *b.PreviousRank != 1 {
		t.Errorf("second = %+v", b)
	}
	if c.ProductID != "c" || c.Count != 0 || c.LastDispensed != nil || len(c.Slots) != 1 || c.Slots[0] != 3 {
		t.Errorf("unsold = %+v", c)
	}
	if got := p.Dayparts[0].Counts["a"]; got != 1 {
		t.Errorf("morning a = %d, want 1", got)
	}
	if got := p.Dayparts[1].Counts["b"]; got != 1 {
		t.Errorf("lunch b = %d, want 1", got)
	}
}

func TestInventoryAnalytics(t *testing.T) {
	svc, st := setupService(t)
	ctx := context.Background()
	st.CreateMachine(ctx, &domain.Machine{ID: "m1", Name: "Test", SlotCount: 2})
	st.CreateProduct(ctx, &domain.Product{ID: "p1", Name: "Cola"})
	st.SetSlots(ctx, "m1", []domain.Slot{
		{MachineID: "m1", Position: 1, ProductID: "p1", Capacity: 10},
		{MachineID: "m1", Position: 2, ProductID: "p1", Capacity: 10},
	})

	t0 := time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC)
	h := func(n float64) time.Time { return t0.Add(time.Duration(n * float64(time.Hour))) }
	addRestock(t, st, "r1", t0, domain.RestockSlot{Position: 1, Quantity: 2})
	addDispense(t, st, "p1", 1, h(1))
	addDispense(t, st, "p1", 1, h(2)) // empty from here
	addDispense(t, st, "p1", 1, h(3)) // untracked: stock was already 0
	addRestock(t, st, "r2", h(10), domain.RestockSlot{Position: 1, Quantity: 5})
	addDispense(t, st, "p1", 1, h(11))

	inv, err := svc.GetInventoryAnalytics(ctx, AnalyticsQuery{MachineID: "m1", From: h(-1), To: h(24)})
	if err != nil {
		t.Fatalf("inventory: %v", err)
	}
	s1, s2 := inv.Slots[0], inv.Slots[1]
	if s1.CurrentStock == nil || *s1.CurrentStock != 4 {
		t.Errorf("slot 1 stock = %v, want 4", s1.CurrentStock)
	}
	if s1.StockoutHours != 8 || s1.Stockouts != 1 || s1.UntrackedDispenses != 1 {
		t.Errorf("slot 1 stockout %.1fh x%d untracked %d, want 8h x1 untracked 1", s1.StockoutHours, s1.Stockouts, s1.UntrackedDispenses)
	}
	if s1.Restocks != 2 || s1.CansRestocked != 7 || s1.Dispensed != 4 {
		t.Errorf("slot 1 restocks %d cans %d dispensed %d", s1.Restocks, s1.CansRestocked, s1.Dispensed)
	}
	// Cycle 1 used 2 of 2, cycle 2 has used 1 of 5 so far.
	if s1.UsedShare == nil || *s1.UsedShare != 0.429 {
		t.Errorf("slot 1 used share = %v, want 0.429", s1.UsedShare)
	}
	if s1.AvgRestockIntervalDays == nil || *s1.AvgRestockIntervalDays != 0.4 {
		t.Errorf("slot 1 interval = %v, want 0.4 days", s1.AvgRestockIntervalDays)
	}
	if s2.CurrentStock != nil || s2.StockoutHours != 0 {
		t.Errorf("never-restocked slot 2 = %+v, want unknown stock and no stockout", s2)
	}
	if inv.Summary.RestockCount != 2 || inv.Summary.CansRestocked != 7 || inv.Restocks[0].ID != "r2" {
		t.Errorf("summary %+v, newest restock %s", inv.Summary, inv.Restocks[0].ID)
	}
	if inv.Restocks[0].DaysSincePrevious == nil || *inv.Restocks[0].DaysSincePrevious != 0.4 {
		t.Errorf("r2 days since previous = %v", inv.Restocks[0].DaysSincePrevious)
	}
	last := inv.Series[len(inv.Series)-1]
	if lv := last.Levels["1"]; lv == nil || *lv != 4 {
		t.Errorf("last level slot 1 = %v, want 4", lv)
	}
	if lv := last.Levels["2"]; lv != nil {
		t.Errorf("last level slot 2 = %d, want null", *lv)
	}
}

func TestSpendAnalytics(t *testing.T) {
	svc, st := setupService(t)
	ctx := context.Background()
	st.CreateMachine(ctx, &domain.Machine{ID: "m1", Name: "Test", SlotCount: 2})
	st.CreateProduct(ctx, &domain.Product{ID: "a", Name: "Apple Soda", CostPerCan: 0.4})
	st.CreateProduct(ctx, &domain.Product{ID: "free", Name: "Free Soda"})
	st.SetSlots(ctx, "m1", []domain.Slot{
		{MachineID: "m1", Position: 1, ProductID: "a", Capacity: 10},
		{MachineID: "m1", Position: 2, ProductID: "free", Capacity: 10},
	})

	from := time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC)
	to := from.AddDate(0, 0, 10)
	addRestock(t, st, "r0", from.AddDate(0, 0, -10)) // tracking covers the whole previous window
	addDispense(t, st, "a", 1, from.Add(-time.Hour)) // previous window
	addRestock(t, st, "r1", from.Add(time.Hour), domain.RestockSlot{Position: 1, Quantity: 10})
	addDispense(t, st, "a", 1, from.Add(2*time.Hour))
	addDispense(t, st, "a", 1, from.AddDate(0, 0, 3))
	addDispense(t, st, "free", 2, from.AddDate(0, 0, 3))

	sp, err := svc.GetSpendAnalytics(ctx, AnalyticsQuery{MachineID: "m1", From: from, To: to})
	if err != nil {
		t.Fatalf("spend: %v", err)
	}
	if sp.Total != 0.8 || sp.Count != 3 || sp.UnpricedDispenses != 1 {
		t.Errorf("total %.2f count %d unpriced %d", sp.Total, sp.Count, sp.UnpricedDispenses)
	}
	if sp.PreviousTotal == nil || *sp.PreviousTotal != 0.4 {
		t.Errorf("previousTotal = %v, want 0.4", sp.PreviousTotal)
	}
	if sp.PerDay != 0.08 || sp.ProjectedMonthly != 2.44 || sp.AvgCostPerCan != 0.27 {
		t.Errorf("perDay %.2f projected %.2f avg %.2f", sp.PerDay, sp.ProjectedMonthly, sp.AvgCostPerCan)
	}
	if sp.RestockedValue != 4 {
		t.Errorf("restockedValue = %.2f, want 4", sp.RestockedValue)
	}
	if last := sp.Series[len(sp.Series)-1]; last.Cumulative != 0.8 {
		t.Errorf("final cumulative = %.2f", last.Cumulative)
	}
	if sp.ByProduct[0].ProductID != "a" || sp.ByProduct[0].Share != 1 {
		t.Errorf("byProduct = %+v", sp.ByProduct)
	}
}
