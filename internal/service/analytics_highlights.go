package service

import (
	"context"
	"math"
	"sort"
	"time"
)

// HighlightsAnalytics are records and habits from the window's dispenses:
// when the first and last cans of the day go, the busiest day and hour,
// totals for soda, caffeine and aluminum, which product opens and closes
// the day, and what goes at lunch. Null fields mean there were no
// dispenses.
type HighlightsAnalytics struct {
	Range       AnalyticsRange `json:"range"`
	FirstCan    *TimeOfDay     `json:"firstCan"`
	LastCan     *TimeOfDay     `json:"lastCan"`
	BusiestDay  *CountAt       `json:"busiestDay"`
	BusiestHour *CountAt       `json:"busiestHour"`
	// FirstCanProducts and LastCanProducts count days by the product of
	// their first and last can. LunchProducts counts cans dispensed from
	// LunchStartHour to LunchEndHour, local time.
	FirstCanProducts []ProductCount `json:"firstCanProducts"`
	LastCanProducts  []ProductCount `json:"lastCanProducts"`
	LunchProducts    []ProductCount `json:"lunchProducts"`

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

// Lunch is 11:00 to 14:00 local time.
const (
	LunchStartHour = 11
	LunchEndHour   = 14
)

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
	out := &HighlightsAnalytics{
		Range:            a.rangeInfo(),
		FirstCanProducts: []ProductCount{},
		LastCanProducts:  []ProductCount{},
		LunchProducts:    []ProductCount{},
	}

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
	lunch := productCounter{}
	for _, i := range inWindow {
		p := a.product(a.dispenses[i])
		if h := a.dispenses[i].Timestamp.In(loc).Hour(); h >= LunchStartHour && h < LunchEndHour {
			lunch.add(p.ID, p.Name)
		}
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
	opens, closes := productCounter{}, productCounter{}
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
		opens.add(p.ID, p.Name)
		p = a.product(a.dispenses[dd.last])
		closes.add(p.ID, p.Name)
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

	out.FirstCanProducts = opens.sorted()
	out.LastCanProducts = closes.sorted()
	out.LunchProducts = lunch.sorted()
	return out, nil
}

// productCounter tallies counts by product.
type productCounter map[string]*ProductCount

func (c productCounter) add(id, name string) {
	if c[id] == nil {
		c[id] = &ProductCount{ProductID: id, Name: name}
	}
	c[id].Count++
}

// sorted returns the counts, largest first, then by name so ties are stable.
func (c productCounter) sorted() []ProductCount {
	out := make([]ProductCount, 0, len(c))
	for _, p := range c {
		out = append(out, *p)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Name < out[j].Name
	})
	return out
}
