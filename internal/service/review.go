package service

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jeffwnelson/sodatracker/internal/domain"
)

// MonthlyReview is one calendar month's stats written up as a Slack message
// to copy and paste: totals, the top products, habits, and the month's soda,
// caffeine and aluminum.
type MonthlyReview struct {
	Month string `json:"month" example:"2026-10"`
	// Partial is true for the month in progress; its stats run to now and it
	// is compared with the same stretch of the month before.
	Partial bool   `json:"partial" example:"false"`
	Cans    int    `json:"cans" example:"412"`
	Text    string `json:"text"`
}

// Rough figures for the review's facts.
const (
	sodaGramsPerOz  = 29.5735 * 1.03 // fl oz × density of soda (diet ~1.00, regular ~1.04)
	canHeightIn12oz = 4.83           // a 12 oz can's height; other sizes scale with volume
)

// reviewStats is everything the review text reports, gathered from the
// analytics views so the text can be rendered (and tested) on its own.
type reviewStats struct {
	month      time.Time // first instant of the month, local
	partial    bool
	days       float64
	cans       int
	prevCans   *int // nil when tracking didn't cover the comparison period
	perDay     float64
	weekday    float64
	peakHour   int
	products   []reviewProduct // dispensed this month, most first
	climber    *reviewProduct  // biggest increase on last month
	firstCan   *TimeOfDay
	lastCan    *TimeOfDay
	busiest    *CountAt
	busiestHr  *CountAt // the window's busiest clock hour
	weekend    int      // cans on Saturdays and Sundays
	drought    *drySpell
	opens      []ProductCount
	closes     []ProductCount
	lunch      []ProductCount
	activeDays int
	ounces     float64
	caffeine   float64
	aluminum   float64 // lb
	restocks   int
	offlineH   float64
}

type reviewProduct struct {
	name     string
	count    int
	share    float64
	rank     int
	prevRank int // 0 when not dispensed last month or no comparison
	gain     int
}

// GetMonthlyReview writes up one calendar month in loc. It returns nil for an
// unknown machine.
func (s *Service) GetMonthlyReview(ctx context.Context, machineID string, year int, month time.Month, loc *time.Location) (*MonthlyReview, error) {
	if loc == nil {
		loc = time.UTC
	}
	start := time.Date(year, month, 1, 0, 0, 0, 0, loc)
	end := start.AddDate(0, 1, 0)
	now := time.Now()
	if !start.Before(now) {
		return nil, fmt.Errorf("%s hasn't started yet", start.Format("January 2006"))
	}
	partial := end.After(now)
	if partial {
		end = now
	}
	// The month before, cut to the same length when this one is in progress.
	prevStart := start.AddDate(0, -1, 0)
	prevEnd := start
	if partial {
		if e := prevStart.Add(end.Sub(start)); e.Before(prevEnd) {
			prevEnd = e
		}
	}

	q := AnalyticsQuery{MachineID: machineID, From: start, To: end, Location: loc, Bucket: BucketDay}
	pq := AnalyticsQuery{MachineID: machineID, From: prevStart, To: prevEnd, Location: loc, Bucket: BucketDay}

	cons, err := s.GetConsumptionAnalytics(ctx, q)
	if err != nil || cons == nil {
		return nil, err
	}
	prods, err := s.GetProductAnalytics(ctx, q)
	if err != nil {
		return nil, err
	}
	hl, err := s.GetHighlightsAnalytics(ctx, q)
	if err != nil {
		return nil, err
	}
	up, err := s.GetUptimeAnalytics(ctx, q)
	if err != nil {
		return nil, err
	}

	st := reviewStats{
		month:     start,
		partial:   partial,
		days:      cons.Range.Days,
		cans:      cons.Total,
		perDay:    cons.PerDay,
		weekday:   cons.WeekdayPerDay,
		peakHour:  cons.PeakHour,
		firstCan:  hl.FirstCan,
		lastCan:   hl.LastCan,
		busiest:   hl.BusiestDay,
		busiestHr: hl.BusiestHour,
		opens:     hl.FirstCanProducts,
		closes:    hl.LastCanProducts,
		lunch:     hl.LunchProducts,
		ounces:    hl.Ounces,
		caffeine:  hl.CaffeineMg,
		aluminum:  hl.AluminumLb,
		offlineH:  up.OfflineHours,
	}
	for _, p := range hl.FirstCanProducts {
		st.activeDays += p.Count
	}

	// Compare with last month only when tracking covered all of it.
	prevRanks := map[string]int{}
	prevCounts := map[string]int{}
	earliest, err := s.store.FirstActivity(ctx, machineID)
	if err != nil {
		return nil, err
	}
	if !earliest.IsZero() && !earliest.After(prevStart) {
		prev, err := s.GetProductAnalytics(ctx, pq)
		if err != nil {
			return nil, err
		}
		total := 0
		for _, p := range rankedProducts(prev.Products) {
			total += p.Count
			prevRanks[p.ProductID] = len(prevRanks) + 1
			prevCounts[p.ProductID] = p.Count
		}
		st.prevCans = &total
	}

	for i, p := range rankedProducts(prods.Products) {
		rp := reviewProduct{name: p.Name, count: p.Count, share: p.Share, rank: i + 1, prevRank: prevRanks[p.ProductID]}
		if st.prevCans != nil {
			rp.gain = p.Count - prevCounts[p.ProductID]
		}
		st.products = append(st.products, rp)
	}
	for i := range st.products {
		if p := &st.products[i]; p.gain > 0 && (st.climber == nil || p.gain > st.climber.gain) {
			st.climber = p
		}
	}

	dispenses, err := s.store.ListDispensesBetween(ctx, machineID, start, end)
	if err != nil {
		return nil, err
	}
	for _, d := range dispenses {
		if wd := d.Timestamp.In(loc).Weekday(); wd == time.Saturday || wd == time.Sunday {
			st.weekend++
		}
	}
	st.drought = longestDrySpell(dispenses, loc, end)

	restocks, err := s.store.ListRestocks(ctx, machineID, analyticsRowLimit)
	if err != nil {
		return nil, err
	}
	for _, r := range restocks {
		if !r.Timestamp.Before(start) && r.Timestamp.Before(end) {
			st.restocks++
		}
	}

	return &MonthlyReview{
		Month:   start.Format("2006-01"),
		Partial: partial,
		Cans:    st.cans,
		Text:    renderReview(st),
	}, nil
}

// Work hours for dry spells, in minutes after local midnight, Monday to Friday.
const (
	WorkStartMinute = 8 * 60
	WorkEndMinute   = 17 * 60
)

// drySpell is the longest stretch of work hours without a can.
type drySpell struct{ start, end time.Time }

// longestDrySpell finds the longest gap between cans within one weekday's
// work hours, counting from the start of work to the first can and from the
// last can to the end of work. Days with no cans in work hours are skipped
// (holidays, or the tracker was down), as is work time after until. dispenses must be in
// ascending order.
func longestDrySpell(dispenses []domain.Dispense, loc *time.Location, until time.Time) *drySpell {
	byDay := map[time.Time][]time.Time{}
	var days []time.Time
	for _, d := range dispenses {
		t := d.Timestamp.In(loc)
		if wd := t.Weekday(); wd == time.Saturday || wd == time.Sunday {
			continue
		}
		day := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc)
		if _, ok := byDay[day]; !ok {
			days = append(days, day)
		}
		byDay[day] = append(byDay[day], t)
	}
	var best *drySpell
	for _, day := range days {
		open := day.Add(WorkStartMinute * time.Minute)
		shut := day.Add(WorkEndMinute * time.Minute)
		if until.Before(shut) {
			shut = until
		}
		inHours := false
		for _, t := range byDay[day] {
			if !t.Before(open) && !t.After(shut) {
				inHours = true
			}
		}
		if !inHours {
			continue
		}
		prev := open
		consider := func(end time.Time) {
			if end.After(shut) {
				end = shut
			}
			if end.After(prev) && (best == nil || end.Sub(prev) > best.end.Sub(best.start)) {
				best = &drySpell{prev, end}
			}
		}
		for _, t := range byDay[day] {
			if t.Before(open) {
				continue
			}
			if t.After(shut) {
				break
			}
			consider(t)
			prev = t
		}
		consider(shut)
	}
	return best
}

// hoursMinutes writes a duration as "3 h 40 min", "45 min" or "2 h".
func hoursMinutes(d time.Duration) string {
	m := int(d.Round(time.Minute).Minutes())
	switch {
	case m < 60:
		return fmt.Sprintf("%d min", m)
	case m%60 == 0:
		return fmt.Sprintf("%d h", m/60)
	}
	return fmt.Sprintf("%d h %d min", m/60, m%60)
}

// rankedProducts returns the products that were dispensed, most first, then
// by name.
func rankedProducts(ps []ProductStat) []ProductStat {
	out := make([]ProductStat, 0, len(ps))
	for _, p := range ps {
		if p.Count > 0 {
			out = append(out, p)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// renderReview writes the review in Slack markup: *bold*, _italic_ and
// Unicode emoji, which survive a copy and paste where :shortcodes: may not.
func renderReview(st reviewStats) string {
	var b strings.Builder
	monthName := st.month.Format("January")
	title := st.month.Format("January 2006")
	if st.partial {
		title += " (so far)"
	}
	fmt.Fprintf(&b, "*🥤 SodaTracker Monthly Review: %s*\n\n", title)

	if st.cans == 0 {
		fmt.Fprintf(&b, "No cans left the machine in %s. Either everyone quit soda, or the tracker did. 🤔\n", monthName)
		return b.String()
	}

	prevName := st.month.AddDate(0, -1, 0).Format("January")
	fmt.Fprintf(&b, "*%s %s* left the machine in %s", commas(st.cans), plural(st.cans, "can", "cans"), monthName)
	if st.prevCans != nil && *st.prevCans > 0 {
		pct := math.Round(float64(st.cans-*st.prevCans) / float64(*st.prevCans) * 100)
		switch {
		case pct > 0:
			fmt.Fprintf(&b, ", up %.0f%% from %s (%s)", pct, prevName, commas(*st.prevCans))
		case pct < 0:
			fmt.Fprintf(&b, ", down %.0f%% from %s (%s)", -pct, prevName, commas(*st.prevCans))
		default:
			fmt.Fprintf(&b, ", the same as %s", prevName)
		}
		if st.partial {
			b.WriteString(" at this point")
		}
	}
	b.WriteString(".\n")
	fmt.Fprintf(&b, "That's %s a day", oneDecimal(st.perDay))
	if st.weekday > 0 {
		fmt.Fprintf(&b, ", or %s on a weekday", oneDecimal(st.weekday))
	}
	b.WriteString(".\n")

	// Top products.
	b.WriteString("\n*🏆 Top sodas*\n")
	medals := []string{"🥇", "🥈", "🥉"}
	for i, p := range st.products {
		if i == len(medals) {
			break
		}
		fmt.Fprintf(&b, "%s %s: %s %s (%.0f%%)%s\n", medals[i], p.name, commas(p.count), plural(p.count, "can", "cans"), p.share*100, rankChange(p, st.prevCans != nil))
	}
	if st.climber != nil {
		fmt.Fprintf(&b, "📈 Biggest climber: %s, +%s on %s\n", st.climber.name, commas(st.climber.gain), prevName)
	}

	// Habits.
	b.WriteString("\n*⏰ Habits*\n")
	if st.firstCan != nil {
		fmt.Fprintf(&b, "• First can of the day, on average: %s (earliest: %s)\n", clock(st.firstCan.AvgMinute), recordTime(st.firstCan.Record))
	}
	if st.lastCan != nil {
		fmt.Fprintf(&b, "• Last can of the day, on average: %s (latest: %s)\n", clock(st.lastCan.AvgMinute), recordTime(st.lastCan.Record))
	}
	if st.busiest != nil {
		if t, err := time.Parse(time.RFC3339, st.busiest.At); err == nil {
			fmt.Fprintf(&b, "• Busiest day: %s, %s %s\n", t.Format("Mon Jan 2"), commas(st.busiest.Count), plural(st.busiest.Count, "can", "cans"))
		}
	}
	if h := st.busiestHr; h != nil {
		if t, err := time.Parse(time.RFC3339, h.At); err == nil {
			fmt.Fprintf(&b, "• Most cans in one hour: %s, %s, %s to %s\n", commas(h.Count), t.Format("Mon Jan 2"), hourName(t.Hour()), hourName((t.Hour()+1)%24))
		}
	}
	if st.drought != nil {
		fmt.Fprintf(&b, "• Longest workday dry spell: %s, %s (%s to %s)\n", hoursMinutes(st.drought.end.Sub(st.drought.start)), st.drought.start.Format("Mon Jan 2"), st.drought.start.Format("3:04 PM"), st.drought.end.Format("3:04 PM"))
	}
	if st.peakHour >= 0 {
		fmt.Fprintf(&b, "• Rush hour: %s to %s\n", hourName(st.peakHour), hourName((st.peakHour+1)%24))
	}
	if len(st.opens) > 0 {
		fmt.Fprintf(&b, "• Opens the day: %s (%d of %d days)\n", st.opens[0].Name, st.opens[0].Count, st.activeDays)
	}
	if len(st.closes) > 0 {
		fmt.Fprintf(&b, "• Closes the day: %s (%d of %d days)\n", st.closes[0].Name, st.closes[0].Count, st.activeDays)
	}
	if len(st.lunch) > 0 {
		fmt.Fprintf(&b, "• Lunch favorite: %s\n", st.lunch[0].Name)
	}
	fmt.Fprintf(&b, "• Weekend cans: %s\n", commas(st.weekend))

	// Fun equivalents.
	b.WriteString("\n*🧮 By the numbers*\n")
	sodaLb := st.ounces * sodaGramsPerOz / gramsPerLb
	stackFt := st.ounces / 12 * canHeightIn12oz / 12
	fmt.Fprintf(&b, "• 🫗 %s gallons of soda\n", oneDecimal(st.ounces/128))
	fmt.Fprintf(&b, "• ⚖️ %s lb of soda\n", commas(int(math.Round(sodaLb))))
	if st.caffeine > 0 {
		fmt.Fprintf(&b, "• ⚡ %s mg of caffeine\n", commas(int(st.caffeine)))
	}
	fmt.Fprintf(&b, "• 🥫 %s lb of aluminum\n", oneDecimal(st.aluminum))
	fmt.Fprintf(&b, "• 🗼 %s ft of cans, stacked end to end\n", commas(int(math.Round(stackFt))))

	// Footnotes.
	var notes []string
	if st.restocks > 0 {
		notes = append(notes, fmt.Sprintf("Restocked %s.", times(st.restocks)))
	}
	if st.offlineH >= 0.1 {
		notes = append(notes, fmt.Sprintf("The tracker was offline for %s, so a few cans may have gone uncounted.", duration(st.offlineH)))
	}
	if len(notes) > 0 {
		fmt.Fprintf(&b, "\n_%s_\n", strings.Join(notes, " "))
	}
	return b.String()
}

func rankChange(p reviewProduct, comparable bool) string {
	switch {
	case !comparable:
		return ""
	case p.prevRank == 0:
		return " 🆕"
	case p.prevRank > p.rank:
		return fmt.Sprintf(" ▲%d", p.prevRank-p.rank)
	case p.prevRank < p.rank:
		return fmt.Sprintf(" ▼%d", p.rank-p.prevRank)
	}
	return ""
}

func recordTime(rfc string) string {
	t, err := time.Parse(time.RFC3339, rfc)
	if err != nil {
		return rfc
	}
	return fmt.Sprintf("%s on %s", clock(t.Hour()*60+t.Minute()), t.Format("Mon Jan 2"))
}

// hourName names an hour of the day: "noon", "midnight", "2 PM".
func hourName(h int) string {
	switch h {
	case 0:
		return "midnight"
	case 12:
		return "noon"
	}
	return time.Date(2000, 1, 1, h, 0, 0, 0, time.UTC).Format("3 PM")
}

// duration writes hours as minutes under an hour ("24 minutes"), else as
// hours to one decimal ("2.5 hours").
func duration(hours float64) string {
	if m := int(math.Round(hours * 60)); m < 60 {
		return fmt.Sprintf("%d %s", m, plural(m, "minute", "minutes"))
	}
	return fmt.Sprintf("%s %s", oneDecimal(hours), plural2(hours, "hour", "hours"))
}

// clock formats minutes after midnight as "8:05 AM".
func clock(min int) string {
	return time.Date(2000, 1, 1, min/60, min%60, 0, 0, time.UTC).Format("3:04 PM")
}

func times(n int) string {
	switch n {
	case 1:
		return "once"
	case 2:
		return "twice"
	}
	return fmt.Sprintf("%d times", n)
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

// plural2 is plural for a figure shown to one decimal place.
func plural2(f float64, one, many string) string {
	if oneDecimal(f) == "1" {
		return one
	}
	return many
}

// oneDecimal formats f with at most one decimal place and thousands
// separators: 1234.56 → "1,234.6", 3.0 → "3".
func oneDecimal(f float64) string {
	r := math.Round(f*10) / 10
	whole := math.Trunc(r)
	s := commas(int(whole))
	if frac := int(math.Round(math.Abs(r-whole) * 10)); frac != 0 {
		s += "." + strconv.Itoa(frac)
	}
	return s
}

func commas(n int) string {
	s := strconv.Itoa(n)
	neg := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(s, "-")
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	if neg {
		s = "-" + s
	}
	return s
}
