package service

import (
	"context"
	"math"
	"sort"
	"time"
)

// HighlightsAnalytics are records and habits from the window's dispenses:
// when the first and last cans of the day go, the busiest day and hour,
// totals for soda, caffeine and aluminum, and which product people reach for
// first. Null fields mean there were no dispenses.
type HighlightsAnalytics struct {
	Range            AnalyticsRange `json:"range"`
	FirstCan         *TimeOfDay     `json:"firstCan"`
	LastCan          *TimeOfDay     `json:"lastCan"`
	BusiestDay       *CountAt       `json:"busiestDay"`
	BusiestHour      *CountAt       `json:"busiestHour"`
	FirstCanProducts []ProductCount `json:"firstCanProducts"`

	// Totals for the window, from each product's can size and caffeine.
	Ounces     float64 `json:"ounces" example:"1092"`
	CaffeineMg float64 `json:"caffeineMg" example:"4120"`
	// AluminumLb is the cans' aluminum, as if every one is recycled.
	AluminumLb float64 `json:"aluminumLb" example:"2.8"`
}

// aluminumGPer12oz is the aluminum in a 12 oz can, about 32 cans to the
// pound; other sizes scale with volume.
const aluminumGPer12oz = 14.2

const gramsPerLb = 453.59237

// TimeOfDay summarises one dispense per active day (the first or the last).
// AvgMinute is minutes after local midnight; Record is the earliest first can
// or the latest last can, so the window's earliest or latest can.
type TimeOfDay struct {
	AvgMinute int    `json:"avgMinute" example:"512"`
	Record    string `json:"record" example:"2026-09-14T06:58:00-05:00"`
}

type CountAt struct {
	At    string `json:"at" example:"2026-09-16T00:00:00-05:00"`
	Count int    `json:"count" example:"19"`
}

type ProductCount struct {
	ProductID string `json:"productId" example:"550e8400-e29b-41d4-a716-446655440000"`
	Name      string `json:"name" example:"Diet Coke"`
	Count     int    `json:"count" example:"9"`
}

func (s *Service) GetHighlightsAnalytics(ctx context.Context, q AnalyticsQuery) (*HighlightsAnalytics, error) {
	a, err := s.loadAnalytics(ctx, q)
	if err != nil || a == nil {
		return nil, err
	}
	loc := a.q.Location
	out := &HighlightsAnalytics{Range: a.rangeInfo(), FirstCanProducts: []ProductCount{}}

	type day struct {
		first, last int // indexes into the window's dispenses
		count       int
	}
	days := map[time.Time]*day{}
	hours := map[time.Time]int{}
	var inWindow []int
	for i, d := range a.dispenses {
		if !a.inRange(d.Timestamp) {
			continue
		}
		inWindow = append(inWindow, i)
		t := d.Timestamp.In(loc)
		key := bucketStart(t, loc, BucketDay)
		if dd, ok := days[key]; ok {
			dd.last = i
			dd.count++
		} else {
			days[key] = &day{first: i, last: i, count: 1}
		}
		hours[time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), 0, 0, 0, loc)]++
	}

	minuteOf := func(i int) int {
		t := a.dispenses[i].Timestamp.In(loc)
		return t.Hour()*60 + t.Minute()
	}
	if len(inWindow) == 0 {
		return out, nil
	}

	var alumG float64
	for _, i := range inWindow {
		p := a.product(a.dispenses[i])
		size := p.SizeOz
		if size <= 0 {
			size = 12
		}
		out.Ounces += size
		out.CaffeineMg += p.CaffeineMg
		alumG += aluminumGPer12oz * size / 12
	}
	out.Ounces = round1(out.Ounces)
	out.CaffeineMg = math.Round(out.CaffeineMg)
	out.AluminumLb = round2(alumG / gramsPerLb)

	var firstSum, lastSum int
	firstRec, lastRec := -1, -1
	byProduct := map[string]*ProductCount{}
	var busiestDay time.Time
	for key, dd := range days {
		firstSum += minuteOf(dd.first)
		lastSum += minuteOf(dd.last)
		if firstRec < 0 || minuteOf(dd.first) < minuteOf(firstRec) {
			firstRec = dd.first
		}
		if lastRec < 0 || minuteOf(dd.last) > minuteOf(lastRec) {
			lastRec = dd.last
		}
		p := a.product(a.dispenses[dd.first])
		if byProduct[p.ID] == nil {
			byProduct[p.ID] = &ProductCount{ProductID: p.ID, Name: p.Name}
		}
		byProduct[p.ID].Count++
		// Ties go to the earlier day, so the answer is stable.
		if b := days[busiestDay]; b == nil || dd.count > b.count || (dd.count == b.count && key.Before(busiestDay)) {
			busiestDay = key
		}
	}
	n := float64(len(days))
	out.FirstCan = &TimeOfDay{
		AvgMinute: int(math.Round(float64(firstSum) / n)),
		Record:    a.dispenses[firstRec].Timestamp.In(loc).Format(time.RFC3339),
	}
	out.LastCan = &TimeOfDay{
		AvgMinute: int(math.Round(float64(lastSum) / n)),
		Record:    a.dispenses[lastRec].Timestamp.In(loc).Format(time.RFC3339),
	}
	out.BusiestDay = &CountAt{At: busiestDay.Format(time.RFC3339), Count: days[busiestDay].count}

	var busiestHour time.Time
	for h, c := range hours {
		if c > hours[busiestHour] || (c == hours[busiestHour] && h.Before(busiestHour)) {
			busiestHour = h
		}
	}
	out.BusiestHour = &CountAt{At: busiestHour.Format(time.RFC3339), Count: hours[busiestHour]}

	for _, p := range byProduct {
		out.FirstCanProducts = append(out.FirstCanProducts, *p)
	}
	sort.Slice(out.FirstCanProducts, func(i, j int) bool {
		pi, pj := out.FirstCanProducts[i], out.FirstCanProducts[j]
		if pi.Count != pj.Count {
			return pi.Count > pj.Count
		}
		return pi.Name < pj.Name
	})
	return out, nil
}
