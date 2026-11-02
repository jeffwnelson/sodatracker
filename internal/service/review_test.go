package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/jeffwnelson/sodatracker/internal/domain"
)

func TestMonthlyReview(t *testing.T) {
	svc, st := setupService(t)
	ctx := context.Background()
	chicago, err := time.LoadLocation("America/Chicago")
	if err != nil {
		t.Fatal(err)
	}

	st.CreateMachine(ctx, &domain.Machine{ID: "m1", Name: "Test", SlotCount: 2})
	st.CreateProduct(ctx, &domain.Product{ID: "p1", Name: "Cola", CostPerCan: 0.5, SizeOz: 12, CaffeineMg: 34})
	st.CreateProduct(ctx, &domain.Product{ID: "p2", Name: "Lemon", CostPerCan: 0.25, SizeOz: 12})
	st.SetSlots(ctx, "m1", []domain.Slot{
		{MachineID: "m1", Position: 1, ProductID: "p1", Capacity: 10},
		{MachineID: "m1", Position: 2, ProductID: "p2", Capacity: 10},
	})

	// Tracking starts in July, so August is wholly covered.
	addRestock(t, st, "r0", time.Date(2026, 7, 31, 8, 0, 0, 0, chicago), domain.RestockSlot{Position: 2, Quantity: 10})
	// August: 2 Lemon, 1 Cola (Lemon first). September: 4 Cola, 1 Lemon.
	addDispense(t, st, "p2", 2, time.Date(2026, 8, 3, 9, 0, 0, 0, chicago))
	addDispense(t, st, "p2", 2, time.Date(2026, 8, 3, 9, 30, 0, 0, chicago))
	addDispense(t, st, "p1", 1, time.Date(2026, 8, 4, 9, 0, 0, 0, chicago))
	addRestock(t, st, "r1", time.Date(2026, 9, 1, 8, 0, 0, 0, chicago), domain.RestockSlot{Position: 1, Quantity: 10})
	addDispense(t, st, "p1", 1, time.Date(2026, 9, 1, 7, 15, 0, 0, chicago))
	addDispense(t, st, "p1", 1, time.Date(2026, 9, 1, 12, 0, 0, 0, chicago))
	addDispense(t, st, "p1", 1, time.Date(2026, 9, 1, 14, 0, 0, 0, chicago))
	addDispense(t, st, "p2", 2, time.Date(2026, 9, 2, 16, 45, 0, 0, chicago))
	addDispense(t, st, "p1", 1, time.Date(2026, 9, 30, 23, 30, 0, 0, chicago)) // last minutes of September
	addDispense(t, st, "p1", 1, time.Date(2026, 10, 1, 0, 30, 0, 0, chicago))  // October

	r, err := svc.GetMonthlyReview(ctx, "m1", 2026, time.September, chicago)
	if err != nil {
		t.Fatal(err)
	}
	if r.Month != "2026-09" || r.Partial || r.Cans != 5 {
		t.Fatalf("review = %+v", r)
	}
	for _, want := range []string{
		"September 2026*",
		"*5 cans* left the machine in September, up 67% from August (3).",
		"🥇 Cola: 4 cans (80%) ▲1",
		"🥈 Lemon: 1 can (20%) ▼1",
		"📈 Biggest climber: Cola, +3 on August",
		"(earliest: 7:15 AM on Tue Sep 1)",
		"Busiest day: Tue Sep 1, 3 cans",
		"Opens the day: Cola (2 of 3 days)",
		"Lunch favorite: Cola",
		"136 mg of caffeine",
		"of a capybara*",
		"Boeing 737",
		"$2.25 on cans",
		"Restocked once.",
	} {
		if !strings.Contains(r.Text, want) {
			t.Errorf("review missing %q:\n%s", want, r.Text)
		}
	}

	// Tracking began partway through July, so August isn't compared.
	r, err = svc.GetMonthlyReview(ctx, "m1", 2026, time.August, chicago)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(r.Text, "from July") || strings.Contains(r.Text, "▲") || strings.Contains(r.Text, "🆕") {
		t.Errorf("august compared with untracked july:\n%s", r.Text)
	}

	r, err = svc.GetMonthlyReview(ctx, "m1", 2026, time.June, chicago)
	if err != nil || r.Cans != 0 || !strings.Contains(r.Text, "No cans") {
		t.Errorf("empty month: %+v, %v", r, err)
	}

	if r, err := svc.GetMonthlyReview(ctx, "nope", 2026, time.June, chicago); r != nil || err != nil {
		t.Errorf("unknown machine: %+v, %v", r, err)
	}
}

func TestReviewFormatting(t *testing.T) {
	for _, c := range []struct{ got, want string }{
		{oneDecimal(1234.56), "1,234.6"},
		{oneDecimal(3.0), "3"},
		{oneDecimal(0.04), "0"},
		{commas(1234567), "1,234,567"},
		{fraction(1.0 / 12345), "1/12,000th"},
		{fraction(1.0 / 7), "1/7th"},
		{stackCompare(9), "50% of a giraffe"},
		{stackCompare(80), "2 telephone poles"},
		{clock(0), "12:00 AM"},
		{clock(13*60 + 5), "1:05 PM"},
	} {
		if c.got != c.want {
			t.Errorf("got %q, want %q", c.got, c.want)
		}
	}
}
