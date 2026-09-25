package service

import (
	"context"
	"math"
	"sort"
	"strconv"
	"time"

	"github.com/jeffwnelson/sodatracker/internal/domain"
)

// analyticsRowLimit caps how many dispenses one analytics request loads. The
// analytics replay full history in memory (stock levels need every dispense
// since the relevant restock), which is fine at one office machine's volume.
const analyticsRowLimit = 1_000_000

// Bucket sizes for time series.
const (
	BucketDay   = "day"
	BucketWeek  = "week"
	BucketMonth = "month"
)

// daysPerMonth is the mean Gregorian month length, used for projections.
const daysPerMonth = 30.44

// AnalyticsQuery selects the machine, window, timezone and bucket size for an
// analytics request. Bucket may be empty to pick one from the window length.
type AnalyticsQuery struct {
	MachineID string
	From      time.Time
	To        time.Time
	Location  *time.Location
	Bucket    string
}

// AnalyticsRange echoes the window actually used. From can be later than
// requested: it is clamped to when tracking began so averages are not diluted
// by days before the machine had any data.
type AnalyticsRange struct {
	From     string  `json:"from" example:"2026-08-25T00:00:00-05:00"`
	To       string  `json:"to" example:"2026-09-24T12:00:00-05:00"`
	Days     float64 `json:"days" example:"30.5"`
	Bucket   string  `json:"bucket" example:"day"`
	Timezone string  `json:"timezone" example:"America/Chicago"`
}

// analyticsData is everything the analytics loaded once per request: all-time
// dispenses and restocks in ascending order, plus product resolution tables.
type analyticsData struct {
	q            AnalyticsQuery
	slots        []domain.Slot
	products     map[string]domain.Product
	slotProducts map[int]domain.Product
	dispenses    []domain.Dispense
	restocks     []domain.Restock
	buckets      []time.Time
	// hasPrevious reports whether tracking covered the whole preceding window
	// of the same length. A partly covered one would make any comparison
	// with it look like a jump.
	hasPrevious bool
}

func (s *Service) loadAnalytics(ctx context.Context, q AnalyticsQuery) (*analyticsData, error) {
	machine, err := s.store.GetMachine(ctx, q.MachineID)
	if err != nil {
		return nil, err
	}
	if machine == nil {
		return nil, nil
	}
	if q.Location == nil {
		q.Location = time.UTC
	}
	if now := time.Now(); q.To.IsZero() || q.To.After(now) {
		q.To = now
	}
	if q.From.IsZero() {
		q.From = q.To.AddDate(0, 0, -30)
	}

	slots, err := s.store.GetSlots(ctx, q.MachineID)
	if err != nil {
		return nil, err
	}
	products, err := s.store.ListProducts(ctx)
	if err != nil {
		return nil, err
	}
	dispenses, err := s.store.ListDispenses(ctx, q.MachineID, time.Time{}, analyticsRowLimit)
	if err != nil {
		return nil, err
	}
	restocks, err := s.store.ListRestocks(ctx, q.MachineID, analyticsRowLimit)
	if err != nil {
		return nil, err
	}
	sort.SliceStable(dispenses, func(i, j int) bool { return dispenses[i].Timestamp.Before(dispenses[j].Timestamp) })
	sort.SliceStable(restocks, func(i, j int) bool { return restocks[i].Timestamp.Before(restocks[j].Timestamp) })

	a := &analyticsData{
		slots:        slots,
		products:     make(map[string]domain.Product, len(products)),
		slotProducts: make(map[int]domain.Product),
		dispenses:    dispenses,
		restocks:     restocks,
	}
	for _, p := range products {
		a.products[p.ID] = p
	}
	for _, sl := range slots {
		if p, ok := a.products[sl.ProductID]; ok {
			a.slotProducts[sl.Position] = p
		}
	}

	var earliest time.Time
	if len(dispenses) > 0 {
		earliest = dispenses[0].Timestamp
	}
	if len(restocks) > 0 && (earliest.IsZero() || restocks[0].Timestamp.Before(earliest)) {
		earliest = restocks[0].Timestamp
	}
	if !earliest.IsZero() && earliest.After(q.From) && earliest.Before(q.To) {
		q.From = maxTime(q.From, bucketStart(earliest, q.Location, BucketDay))
	}
	a.hasPrevious = !earliest.IsZero() && !earliest.After(q.From.Add(-q.To.Sub(q.From)))

	if q.Bucket == "" {
		q.Bucket = autoBucket(q.To.Sub(q.From))
	}
	for t := bucketStart(q.From, q.Location, q.Bucket); t.Before(q.To); t = bucketNext(t, q.Bucket) {
		a.buckets = append(a.buckets, t)
	}
	a.q = q
	return a, nil
}

func autoBucket(span time.Duration) string {
	days := span.Hours() / 24
	switch {
	case days <= 62:
		return BucketDay
	case days <= 366:
		return BucketWeek
	default:
		return BucketMonth
	}
}

// bucketStart truncates t to the start of its day, ISO week (Monday) or month
// in loc.
func bucketStart(t time.Time, loc *time.Location, bucket string) time.Time {
	t = t.In(loc)
	y, m, d := t.Date()
	switch bucket {
	case BucketMonth:
		return time.Date(y, m, 1, 0, 0, 0, 0, loc)
	case BucketWeek:
		return time.Date(y, m, d-mondayIndex(t.Weekday()), 0, 0, 0, 0, loc)
	default:
		return time.Date(y, m, d, 0, 0, 0, 0, loc)
	}
}

// bucketNext uses calendar arithmetic so DST days stay aligned to midnight.
func bucketNext(t time.Time, bucket string) time.Time {
	switch bucket {
	case BucketMonth:
		return t.AddDate(0, 1, 0)
	case BucketWeek:
		return t.AddDate(0, 0, 7)
	default:
		return t.AddDate(0, 0, 1)
	}
}

// mondayIndex maps Go's Sunday-first weekday to Monday=0..Sunday=6.
func mondayIndex(w time.Weekday) int {
	return (int(w) + 6) % 7
}

func (a *analyticsData) rangeInfo() AnalyticsRange {
	return AnalyticsRange{
		From:     a.q.From.In(a.q.Location).Format(time.RFC3339),
		To:       a.q.To.In(a.q.Location).Format(time.RFC3339),
		Days:     round1(a.days()),
		Bucket:   a.q.Bucket,
		Timezone: a.q.Location.String(),
	}
}

func (a *analyticsData) days() float64 {
	return a.q.To.Sub(a.q.From).Hours() / 24
}

func (a *analyticsData) inRange(t time.Time) bool {
	return !t.Before(a.q.From) && t.Before(a.q.To)
}

// inPrevious reports whether t falls in the window of equal length just
// before the current one.
func (a *analyticsData) inPrevious(t time.Time) bool {
	prevFrom := a.q.From.Add(-a.q.To.Sub(a.q.From))
	return !t.Before(prevFrom) && t.Before(a.q.From)
}

// bucketIndex returns the series index for t, or -1 when outside the series.
func (a *analyticsData) bucketIndex(t time.Time) int {
	i := sort.Search(len(a.buckets), func(i int) bool { return a.buckets[i].After(t) }) - 1
	if i < 0 || !a.inRange(t) {
		return -1
	}
	return i
}

func (a *analyticsData) bucketLabel(i int) string {
	return a.buckets[i].Format(time.RFC3339)
}

// product resolves a dispense's product. Products recreated under a new UUID
// orphan older dispenses, so fall back to whatever the slot holds now — the
// same rule GetDispenseStats uses.
func (a *analyticsData) product(d domain.Dispense) domain.Product {
	if p, ok := a.products[d.ProductID]; ok {
		return p
	}
	if p, ok := a.slotProducts[d.SlotPosition]; ok {
		return p
	}
	return domain.Product{ID: d.ProductID, Name: d.ProductID}
}

// countDays counts the local calendar days in the window, split into weekdays
// and weekend days, weighting partial first/last days by the covered fraction.
func (a *analyticsData) countDays() (weekdays, weekend float64) {
	loc := a.q.Location
	for day := bucketStart(a.q.From, loc, BucketDay); day.Before(a.q.To); day = day.AddDate(0, 0, 1) {
		next := day.AddDate(0, 0, 1)
		start, end := maxTime(day, a.q.From), minTime(next, a.q.To)
		frac := end.Sub(start).Hours() / next.Sub(day).Hours()
		if w := day.Weekday(); w == time.Saturday || w == time.Sunday {
			weekend += frac
		} else {
			weekdays += frac
		}
	}
	return weekdays, weekend
}

// ---------------------------------------------------------------------------
// Consumption

type ConsumptionAnalytics struct {
	Range AnalyticsRange `json:"range"`
	Total int            `json:"total" example:"212"`
	// PreviousTotal is the count for the preceding window of the same length,
	// or null when tracking did not cover all of it.
	PreviousTotal *int    `json:"previousTotal" example:"187"`
	PerDay        float64 `json:"perDay" example:"7.1"`
	WeekdayPerDay float64 `json:"weekdayPerDay" example:"9.4"`
	WeekendPerDay float64 `json:"weekendPerDay" example:"1.2"`
	// PeakHour is the local hour (0-23) with the most dispenses, -1 if none.
	PeakHour int `json:"peakHour" example:"14"`
	// PeakWeekday is Monday=0..Sunday=6, -1 if none.
	PeakWeekday int                `json:"peakWeekday" example:"2"`
	Series      []ConsumptionPoint `json:"series"`
	// Heatmap is indexed [weekday Monday=0][local hour].
	Heatmap [7][24]int `json:"heatmap"`
}

type ConsumptionPoint struct {
	Bucket string `json:"bucket" example:"2026-09-01T00:00:00-05:00"`
	Count  int    `json:"count" example:"9"`
	// Trend is the trailing moving average (7 days, 4 weeks or 3 months).
	Trend float64 `json:"trend" example:"7.4"`
}

// GetConsumptionAnalytics reports dispense volume over time and when in the
// week people use the machine.
func (s *Service) GetConsumptionAnalytics(ctx context.Context, q AnalyticsQuery) (*ConsumptionAnalytics, error) {
	a, err := s.loadAnalytics(ctx, q)
	if err != nil || a == nil {
		return nil, err
	}
	out := &ConsumptionAnalytics{Range: a.rangeInfo(), PeakHour: -1, PeakWeekday: -1}
	counts := make([]int, len(a.buckets))
	prev := 0
	var hourTotals [24]int
	var weekdayTotals [7]int
	var weekdayCount, weekendCount int

	for _, d := range a.dispenses {
		if a.inPrevious(d.Timestamp) {
			prev++
		}
		i := a.bucketIndex(d.Timestamp)
		if i < 0 {
			continue
		}
		out.Total++
		counts[i]++
		local := d.Timestamp.In(a.q.Location)
		wd := mondayIndex(local.Weekday())
		out.Heatmap[wd][local.Hour()]++
		hourTotals[local.Hour()]++
		weekdayTotals[wd]++
		if wd >= 5 {
			weekendCount++
		} else {
			weekdayCount++
		}
	}
	if a.hasPrevious {
		out.PreviousTotal = &prev
	}
	if out.Total > 0 {
		out.PeakHour = argmax(hourTotals[:])
		out.PeakWeekday = argmax(weekdayTotals[:])
	}

	if days := a.days(); days > 0 {
		out.PerDay = round1(float64(out.Total) / days)
	}
	weekdays, weekend := a.countDays()
	if weekdays > 0 {
		out.WeekdayPerDay = round1(float64(weekdayCount) / weekdays)
	}
	if weekend > 0 {
		out.WeekendPerDay = round1(float64(weekendCount) / weekend)
	}

	window := map[string]int{BucketDay: 7, BucketWeek: 4, BucketMonth: 3}[a.q.Bucket]
	out.Series = make([]ConsumptionPoint, len(a.buckets))
	sum := 0
	for i, c := range counts {
		sum += c
		if i >= window {
			sum -= counts[i-window]
		}
		out.Series[i] = ConsumptionPoint{
			Bucket: a.bucketLabel(i),
			Count:  c,
			Trend:  round1(float64(sum) / float64(min(i+1, window))),
		}
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// Products

type ProductAnalytics struct {
	Range    AnalyticsRange       `json:"range"`
	Products []ProductStat        `json:"products"`
	Series   []ProductSeriesPoint `json:"series"`
	Dayparts []DaypartStat        `json:"dayparts"`
}

type ProductStat struct {
	ProductID string  `json:"productId" example:"550e8400-e29b-41d4-a716-446655440000"`
	Name      string  `json:"name" example:"Coca-Cola"`
	Count     int     `json:"count" example:"64"`
	Share     float64 `json:"share" example:"0.31"`
	Cost      float64 `json:"cost" example:"22.40"`
	// Rank is 1-based by count. PreviousRank is null when the product had no
	// dispenses in the preceding window (or tracking did not cover all of it).
	Rank          int     `json:"rank" example:"1"`
	PreviousRank  *int    `json:"previousRank" example:"2"`
	PreviousCount *int    `json:"previousCount" example:"51"`
	Slots         []int   `json:"slots"`
	LastDispensed *string `json:"lastDispensed"`
}

type ProductSeriesPoint struct {
	Bucket string `json:"bucket" example:"2026-09-01T00:00:00-05:00"`
	// Counts maps productId to dispenses in the bucket.
	Counts map[string]int `json:"counts"`
}

type DaypartStat struct {
	Name      string `json:"name" example:"Afternoon"`
	StartHour int    `json:"startHour" example:"14"`
	EndHour   int    `json:"endHour" example:"17"`
	Total     int    `json:"total" example:"80"`
	// Counts maps productId to dispenses in this part of the day.
	Counts map[string]int `json:"counts"`
}

// dayparts split the local day; the last one wraps past midnight.
var dayparts = []struct {
	name       string
	start, end int
}{
	{"Morning", 5, 11},
	{"Lunch", 11, 14},
	{"Afternoon", 14, 17},
	{"Evening", 17, 22},
	{"Overnight", 22, 5},
}

func daypartIndex(hour int) int {
	for i, dp := range dayparts {
		if dp.start < dp.end && hour >= dp.start && hour < dp.end {
			return i
		}
		if dp.start > dp.end && (hour >= dp.start || hour < dp.end) {
			return i
		}
	}
	return len(dayparts) - 1
}

// GetProductAnalytics ranks products, compares with the previous window, and
// shows what sells at which time of day. Products stocked in a slot appear
// even with zero dispenses so slow movers are visible.
func (s *Service) GetProductAnalytics(ctx context.Context, q AnalyticsQuery) (*ProductAnalytics, error) {
	a, err := s.loadAnalytics(ctx, q)
	if err != nil || a == nil {
		return nil, err
	}
	out := &ProductAnalytics{Range: a.rangeInfo()}

	stats := map[string]*ProductStat{}
	get := func(p domain.Product) *ProductStat {
		st, ok := stats[p.ID]
		if !ok {
			st = &ProductStat{ProductID: p.ID, Name: p.Name, Slots: []int{}}
			stats[p.ID] = st
		}
		return st
	}
	for _, sl := range a.slots {
		if p, ok := a.slotProducts[sl.Position]; ok {
			st := get(p)
			st.Slots = append(st.Slots, sl.Position)
		}
	}

	out.Series = make([]ProductSeriesPoint, len(a.buckets))
	for i := range a.buckets {
		out.Series[i] = ProductSeriesPoint{Bucket: a.bucketLabel(i), Counts: map[string]int{}}
	}
	out.Dayparts = make([]DaypartStat, len(dayparts))
	for i, dp := range dayparts {
		out.Dayparts[i] = DaypartStat{Name: dp.name, StartHour: dp.start, EndHour: dp.end, Counts: map[string]int{}}
	}

	prevCounts := map[string]int{}
	total := 0
	for _, d := range a.dispenses {
		p := a.product(d)
		if a.inPrevious(d.Timestamp) {
			prevCounts[p.ID]++
		}
		i := a.bucketIndex(d.Timestamp)
		if i < 0 {
			continue
		}
		total++
		st := get(p)
		st.Count++
		st.Cost += p.CostPerCan
		ts := d.Timestamp.In(a.q.Location).Format(time.RFC3339)
		st.LastDispensed = &ts
		out.Series[i].Counts[p.ID]++
		dp := &out.Dayparts[daypartIndex(d.Timestamp.In(a.q.Location).Hour())]
		dp.Total++
		dp.Counts[p.ID]++
	}

	for _, st := range stats {
		st.Cost = round2(st.Cost)
		if total > 0 {
			st.Share = round3(float64(st.Count) / float64(total))
		}
		out.Products = append(out.Products, *st)
	}
	sort.Slice(out.Products, func(i, j int) bool {
		pi, pj := out.Products[i], out.Products[j]
		if pi.Count != pj.Count {
			return pi.Count > pj.Count
		}
		return pi.Name < pj.Name
	})
	for i := range out.Products {
		out.Products[i].Rank = i + 1
	}

	if a.hasPrevious {
		prevRanks := rankByCount(prevCounts)
		for i := range out.Products {
			id := out.Products[i].ProductID
			c := prevCounts[id]
			out.Products[i].PreviousCount = &c
			if r, ok := prevRanks[id]; ok {
				out.Products[i].PreviousRank = &r
			}
		}
	}
	if out.Products == nil {
		out.Products = []ProductStat{}
	}
	return out, nil
}

// rankByCount assigns 1-based ranks by descending count, ties broken by ID
// for stability.
func rankByCount(counts map[string]int) map[string]int {
	ids := make([]string, 0, len(counts))
	for id := range counts {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		if counts[ids[i]] != counts[ids[j]] {
			return counts[ids[i]] > counts[ids[j]]
		}
		return ids[i] < ids[j]
	})
	ranks := make(map[string]int, len(ids))
	for i, id := range ids {
		ranks[id] = i + 1
	}
	return ranks
}

// ---------------------------------------------------------------------------
// Inventory

type InventoryAnalytics struct {
	Range    AnalyticsRange    `json:"range"`
	Summary  InventorySummary  `json:"summary"`
	Slots    []SlotInventory   `json:"slots"`
	Series   []StockLevelPoint `json:"series"`
	Restocks []RestockSummary  `json:"restocks"`
}

type InventorySummary struct {
	StockoutHours float64 `json:"stockoutHours" example:"41.5"`
	Stockouts     int     `json:"stockouts" example:"3"`
	RestockCount  int     `json:"restockCount" example:"4"`
	CansRestocked int     `json:"cansRestocked" example:"96"`
	// AvgRestockIntervalDays is null with fewer than two restocks.
	AvgRestockIntervalDays *float64 `json:"avgRestockIntervalDays" example:"7.5"`
	// SellThrough is the share of cans restocked in the window that were
	// dispensed before that slot's next restock. Null with no restocks.
	SellThrough *float64 `json:"sellThrough" example:"0.82"`
	// UntrackedDispenses counts dispenses recorded while computed stock was
	// already zero: the slot held more cans than the last restock logged.
	UntrackedDispenses int `json:"untrackedDispenses" example:"2"`
}

type SlotInventory struct {
	Position    int    `json:"position" example:"1"`
	ProductID   string `json:"productId,omitempty" example:"550e8400-e29b-41d4-a716-446655440000"`
	ProductName string `json:"productName,omitempty" example:"Coca-Cola"`
	Capacity    int    `json:"capacity" example:"12"`
	// LowThreshold is the slot's own low-stock level (alert configs can
	// override it for alerting, but not here).
	LowThreshold int `json:"lowThreshold" example:"3"`
	// CurrentStock is null when the slot has never been restocked.
	CurrentStock           *int     `json:"currentStock" example:"5"`
	StockoutHours          float64  `json:"stockoutHours" example:"12.5"`
	Stockouts              int      `json:"stockouts" example:"1"`
	Restocks               int      `json:"restocks" example:"2"`
	CansRestocked          int      `json:"cansRestocked" example:"24"`
	Dispensed              int      `json:"dispensed" example:"19"`
	AvgRestockIntervalDays *float64 `json:"avgRestockIntervalDays" example:"6.0"`
	SellThrough            *float64 `json:"sellThrough" example:"0.79"`
	UntrackedDispenses     int      `json:"untrackedDispenses" example:"0"`
}

type StockLevelPoint struct {
	Bucket string `json:"bucket" example:"2026-09-01T00:00:00-05:00"`
	// Levels maps slot position to stock at the end of the bucket, or null
	// when the slot had not been restocked yet.
	Levels map[string]*int `json:"levels"`
}

type RestockSummary struct {
	ID                string   `json:"id" example:"550e8400-e29b-41d4-a716-446655440003"`
	Timestamp         string   `json:"timestamp" example:"2026-09-10T09:15:00-05:00"`
	Cans              int      `json:"cans" example:"24"`
	Slots             int      `json:"slots" example:"3"`
	DaysSincePrevious *float64 `json:"daysSincePrevious" example:"7.1"`
}

// stockStep is the stock level from At until the next step.
type stockStep struct {
	At    time.Time
	Stock int
}

// GetInventoryAnalytics replays restocks and dispenses per slot to rebuild
// stock levels over time. It uses the same model as GetMachineStatus: a
// restock sets the level, each later dispense takes one can, floored at zero.
func (s *Service) GetInventoryAnalytics(ctx context.Context, q AnalyticsQuery) (*InventoryAnalytics, error) {
	a, err := s.loadAnalytics(ctx, q)
	if err != nil || a == nil {
		return nil, err
	}
	out := &InventoryAnalytics{Range: a.rangeInfo(), Slots: []SlotInventory{}, Restocks: []RestockSummary{}}

	type event struct {
		at      time.Time
		restock bool
		qty     int
	}
	perSlot := map[int][]event{}
	for _, r := range a.restocks {
		for _, rs := range r.Slots {
			perSlot[rs.Position] = append(perSlot[rs.Position], event{at: r.Timestamp, restock: true, qty: rs.Quantity})
		}
	}
	for _, d := range a.dispenses {
		perSlot[d.SlotPosition] = append(perSlot[d.SlotPosition], event{at: d.Timestamp})
	}

	steps := map[int][]stockStep{}
	var soldTotal, cycleCansTotal int
	var slotIntervals []float64

	for _, sl := range a.slots {
		evs := perSlot[sl.Position]
		// Restocks sort ahead of dispenses at the same instant, matching the
		// ">= restock time" count in GetMachineStatus.
		sort.SliceStable(evs, func(i, j int) bool {
			if evs[i].at.Equal(evs[j].at) {
				return evs[i].restock && !evs[j].restock
			}
			return evs[i].at.Before(evs[j].at)
		})

		si := SlotInventory{Position: sl.Position, Capacity: sl.Capacity, LowThreshold: sl.LowThreshold}
		if p, ok := a.slotProducts[sl.Position]; ok {
			si.ProductID, si.ProductName = p.ID, p.Name
		}

		known := false
		stock := 0
		var lastAt, prevRestock time.Time
		// The open restock cycle, counted only if it began in the window.
		cycleQty, cycleStart, cycleInRange := 0, 0, false
		var intervals []float64
		var sold, cycleCans int

		closeCycle := func() {
			if cycleInRange {
				cycleCans += cycleQty
				sold += cycleStart - stock
			}
		}

		for _, e := range evs {
			if known && stock == 0 {
				si.StockoutHours += overlapHours(lastAt, e.at, a.q.From, a.q.To)
			}
			if e.restock {
				closeCycle()
				inRange := a.inRange(e.at)
				if inRange {
					si.Restocks++
					si.CansRestocked += e.qty
					if !prevRestock.IsZero() {
						intervals = append(intervals, e.at.Sub(prevRestock).Hours()/24)
					}
				}
				prevRestock = e.at
				known, stock = true, e.qty
				cycleQty, cycleStart, cycleInRange = e.qty, e.qty, inRange
			} else {
				if a.inRange(e.at) {
					si.Dispensed++
					if known && stock == 0 {
						si.UntrackedDispenses++
					} else if known && stock == 1 {
						si.Stockouts++
					}
				}
				if known && stock > 0 {
					stock--
				}
			}
			lastAt = e.at
			if known {
				steps[sl.Position] = append(steps[sl.Position], stockStep{At: e.at, Stock: stock})
			}
		}
		if known && stock == 0 {
			si.StockoutHours += overlapHours(lastAt, a.q.To, a.q.From, a.q.To)
		}
		closeCycle()

		if known {
			cur := stock
			si.CurrentStock = &cur
		}
		si.StockoutHours = round1(si.StockoutHours)
		if len(intervals) > 0 {
			avg := round1(mean(intervals))
			si.AvgRestockIntervalDays = &avg
			slotIntervals = append(slotIntervals, intervals...)
		}
		if cycleCans > 0 {
			st := round3(float64(sold) / float64(cycleCans))
			si.SellThrough = &st
		}
		soldTotal += sold
		cycleCansTotal += cycleCans

		out.Summary.StockoutHours += si.StockoutHours
		out.Summary.Stockouts += si.Stockouts
		out.Summary.CansRestocked += si.CansRestocked
		out.Summary.UntrackedDispenses += si.UntrackedDispenses
		out.Slots = append(out.Slots, si)
	}
	out.Summary.StockoutHours = round1(out.Summary.StockoutHours)
	if cycleCansTotal > 0 {
		st := round3(float64(soldTotal) / float64(cycleCansTotal))
		out.Summary.SellThrough = &st
	}

	// Machine-level restock log. Intervals are between restock visits,
	// regardless of which slots each touched.
	var visitIntervals []float64
	var prev time.Time
	for _, r := range a.restocks {
		if a.inRange(r.Timestamp) {
			rsum := RestockSummary{ID: r.ID, Timestamp: r.Timestamp.In(a.q.Location).Format(time.RFC3339), Slots: len(r.Slots)}
			for _, rs := range r.Slots {
				rsum.Cans += rs.Quantity
			}
			if !prev.IsZero() {
				d := round1(r.Timestamp.Sub(prev).Hours() / 24)
				rsum.DaysSincePrevious = &d
				visitIntervals = append(visitIntervals, r.Timestamp.Sub(prev).Hours()/24)
			}
			out.Restocks = append(out.Restocks, rsum)
		}
		prev = r.Timestamp
	}
	out.Summary.RestockCount = len(out.Restocks)
	if len(visitIntervals) > 0 {
		avg := round1(mean(visitIntervals))
		out.Summary.AvgRestockIntervalDays = &avg
	}
	// Newest first, like the restock history elsewhere in the app.
	for i, j := 0, len(out.Restocks)-1; i < j; i, j = i+1, j-1 {
		out.Restocks[i], out.Restocks[j] = out.Restocks[j], out.Restocks[i]
	}

	out.Series = make([]StockLevelPoint, len(a.buckets))
	for i, b := range a.buckets {
		sample := minTime(bucketNext(b, a.q.Bucket), a.q.To)
		levels := map[string]*int{}
		for _, sl := range a.slots {
			levels[strconv.Itoa(sl.Position)] = levelAt(steps[sl.Position], sample)
		}
		out.Series[i] = StockLevelPoint{Bucket: a.bucketLabel(i), Levels: levels}
	}
	return out, nil
}

// levelAt returns the stock in effect just before t, or nil if unknown.
func levelAt(steps []stockStep, t time.Time) *int {
	i := sort.Search(len(steps), func(i int) bool { return !steps[i].At.Before(t) }) - 1
	if i < 0 {
		return nil
	}
	v := steps[i].Stock
	return &v
}

// overlapHours is the length of [a, b) ∩ [from, to) in hours.
func overlapHours(a, b, from, to time.Time) float64 {
	start, end := maxTime(a, from), minTime(b, to)
	if !end.After(start) {
		return 0
	}
	return end.Sub(start).Hours()
}

// ---------------------------------------------------------------------------
// Spend

type SpendAnalytics struct {
	Range AnalyticsRange `json:"range"`
	Total float64        `json:"total" example:"74.20"`
	// PreviousTotal is spend in the preceding window of the same length, or
	// null when tracking did not cover all of it.
	PreviousTotal    *float64 `json:"previousTotal" example:"65.10"`
	Count            int      `json:"count" example:"212"`
	PerDay           float64  `json:"perDay" example:"2.47"`
	ProjectedMonthly float64  `json:"projectedMonthly" example:"75.30"`
	AvgCostPerCan    float64  `json:"avgCostPerCan" example:"0.35"`
	// RestockedValue prices the cans restocked in the window at each slot's
	// current product cost.
	RestockedValue float64 `json:"restockedValue" example:"33.60"`
	// UnpricedDispenses counts dispenses whose product has no cost set; they
	// add nothing to the totals.
	UnpricedDispenses int            `json:"unpricedDispenses" example:"0"`
	Series            []SpendPoint   `json:"series"`
	ByProduct         []ProductSpend `json:"byProduct"`
}

type SpendPoint struct {
	Bucket     string  `json:"bucket" example:"2026-09-01T00:00:00-05:00"`
	Spend      float64 `json:"spend" example:"3.15"`
	Count      int     `json:"count" example:"9"`
	Cumulative float64 `json:"cumulative" example:"28.70"`
}

type ProductSpend struct {
	ProductID string  `json:"productId" example:"550e8400-e29b-41d4-a716-446655440000"`
	Name      string  `json:"name" example:"Coca-Cola"`
	Spend     float64 `json:"spend" example:"22.40"`
	Count     int     `json:"count" example:"64"`
	Share     float64 `json:"share" example:"0.30"`
}

// GetSpendAnalytics prices dispenses at each product's cost per can.
func (s *Service) GetSpendAnalytics(ctx context.Context, q AnalyticsQuery) (*SpendAnalytics, error) {
	a, err := s.loadAnalytics(ctx, q)
	if err != nil || a == nil {
		return nil, err
	}
	out := &SpendAnalytics{Range: a.rangeInfo(), ByProduct: []ProductSpend{}}
	spend := make([]float64, len(a.buckets))
	counts := make([]int, len(a.buckets))
	byProduct := map[string]*ProductSpend{}
	prev := 0.0

	for _, d := range a.dispenses {
		p := a.product(d)
		if a.inPrevious(d.Timestamp) {
			prev += p.CostPerCan
		}
		i := a.bucketIndex(d.Timestamp)
		if i < 0 {
			continue
		}
		out.Count++
		out.Total += p.CostPerCan
		spend[i] += p.CostPerCan
		counts[i]++
		if p.CostPerCan == 0 {
			out.UnpricedDispenses++
		}
		ps, ok := byProduct[p.ID]
		if !ok {
			ps = &ProductSpend{ProductID: p.ID, Name: p.Name}
			byProduct[p.ID] = ps
		}
		ps.Spend += p.CostPerCan
		ps.Count++
	}

	for _, r := range a.restocks {
		if !a.inRange(r.Timestamp) {
			continue
		}
		for _, rs := range r.Slots {
			out.RestockedValue += float64(rs.Quantity) * a.slotProducts[rs.Position].CostPerCan
		}
	}

	if a.hasPrevious {
		p := round2(prev)
		out.PreviousTotal = &p
	}
	if days := a.days(); days > 0 {
		out.PerDay = round2(out.Total / days)
		out.ProjectedMonthly = round2(out.Total / days * daysPerMonth)
	}
	if out.Count > 0 {
		out.AvgCostPerCan = round2(out.Total / float64(out.Count))
	}

	out.Series = make([]SpendPoint, len(a.buckets))
	cum := 0.0
	for i := range a.buckets {
		cum += spend[i]
		out.Series[i] = SpendPoint{Bucket: a.bucketLabel(i), Spend: round2(spend[i]), Count: counts[i], Cumulative: round2(cum)}
	}
	for _, ps := range byProduct {
		if out.Total > 0 {
			ps.Share = round3(ps.Spend / out.Total)
		}
		ps.Spend = round2(ps.Spend)
		out.ByProduct = append(out.ByProduct, *ps)
	}
	sort.Slice(out.ByProduct, func(i, j int) bool {
		if out.ByProduct[i].Spend != out.ByProduct[j].Spend {
			return out.ByProduct[i].Spend > out.ByProduct[j].Spend
		}
		return out.ByProduct[i].Name < out.ByProduct[j].Name
	})
	out.Total = round2(out.Total)
	out.RestockedValue = round2(out.RestockedValue)
	return out, nil
}

// ---------------------------------------------------------------------------
// helpers

func argmax(xs []int) int {
	best := 0
	for i, x := range xs {
		if x > xs[best] {
			best = i
		}
	}
	return best
}

func mean(xs []float64) float64 {
	sum := 0.0
	for _, x := range xs {
		sum += x
	}
	return sum / float64(len(xs))
}

func round1(x float64) float64 { return math.Round(x*10) / 10 }
func round2(x float64) float64 { return math.Round(x*100) / 100 }
func round3(x float64) float64 { return math.Round(x*1000) / 1000 }

func minTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}

func maxTime(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}
