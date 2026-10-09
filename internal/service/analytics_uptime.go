package service

import (
	"context"
	"sort"
	"time"
)

// maxOutagesListed caps the outage table; the totals still count every one.
const maxOutagesListed = 50

// UptimeAnalytics breaks the window into time the Pi agent was heartbeating
// (online), time it wasn't (offline: dispenses weren't being counted), and
// time before heartbeats were first recorded (untracked).
type UptimeAnalytics struct {
	Range AnalyticsRange `json:"range"`
	// TrackingSince is the agent's first recorded heartbeat, ever.
	TrackingSince  *string `json:"trackingSince" example:"2026-10-01T01:30:27Z"`
	OnlineHours    float64 `json:"onlineHours" example:"712.4"`
	OfflineHours   float64 `json:"offlineHours" example:"3.1"`
	UntrackedHours float64 `json:"untrackedHours" example:"0"`
	// Uptime is online / (online + offline); null when nothing was tracked.
	Uptime             *float64      `json:"uptime" example:"0.996"`
	Outages            int           `json:"outages" example:"2"`
	LongestOutageHours float64       `json:"longestOutageHours" example:"2.5"`
	Series             []UptimePoint `json:"series"`
	// OutageList is newest first, at most 50.
	OutageList []Outage `json:"outageList"`
}

type UptimePoint struct {
	Bucket         string  `json:"bucket" example:"2026-09-01T00:00:00-05:00"`
	OnlineHours    float64 `json:"onlineHours" example:"23.5"`
	OfflineHours   float64 `json:"offlineHours" example:"0.5"`
	UntrackedHours float64 `json:"untrackedHours" example:"0"`
}

type Outage struct {
	From  string  `json:"from" example:"2026-09-03T14:02:00-05:00"`
	To    string  `json:"to" example:"2026-09-03T14:40:00-05:00"`
	Hours float64 `json:"hours" example:"0.6"`
	// Ongoing means the agent is still offline; To is the end of the window.
	Ongoing bool `json:"ongoing"`
}

type interval struct{ from, to time.Time }

// GetUptimeAnalytics replays the agent's heartbeat sessions over the window.
// A session counts as online from its first beat to its last; the gaps
// between sessions are outages. The current session runs to the end of the
// window while its last beat is recent.
func (s *Service) GetUptimeAnalytics(ctx context.Context, q AnalyticsQuery) (*UptimeAnalytics, error) {
	a, err := s.loadAnalytics(ctx, q, needNothing)
	if err != nil || a == nil {
		return nil, err
	}
	sessions, err := s.store.ListAgentSessions(ctx, q.MachineID)
	if err != nil {
		return nil, err
	}
	from, to, loc := a.q.From, a.q.To, a.q.Location
	out := &UptimeAnalytics{Range: a.rangeInfo(), OutageList: []Outage{}}

	var online []interval
	for i, ss := range sessions {
		end := ss.LastSeen
		if i == len(sessions)-1 && to.Sub(ss.LastSeen) <= AgentOfflineAfter {
			end = to
		}
		online = append(online, interval{ss.StartedAt, end})
	}

	// Heartbeats started after the window opened: the start is untracked.
	trackedFrom := from
	if len(sessions) > 0 {
		first := sessions[0].StartedAt
		since := first.In(loc).Format(time.RFC3339)
		out.TrackingSince = &since
		trackedFrom = maxTime(from, first)
	} else {
		trackedFrom = to
	}
	trackedFrom = minTime(trackedFrom, to)

	// Outages are the gaps in the tracked part of the window.
	var offline []interval
	cursor := trackedFrom
	for _, iv := range online {
		if iv.to.Before(cursor) || !iv.to.After(trackedFrom) {
			continue
		}
		if iv.from.After(cursor) {
			offline = append(offline, interval{cursor, minTime(iv.from, to)})
		}
		cursor = maxTime(cursor, iv.to)
		if !cursor.Before(to) {
			break
		}
	}
	if cursor.Before(to) {
		offline = append(offline, interval{cursor, to})
	}

	hours := func(ivs []interval, lo, hi time.Time) float64 {
		var d time.Duration
		for _, iv := range ivs {
			if o := minTime(iv.to, hi).Sub(maxTime(iv.from, lo)); o > 0 {
				d += o
			}
		}
		return d.Hours()
	}
	untracked := []interval{{from, trackedFrom}}

	onlineH, offlineH := hours(online, from, to), hours(offline, from, to)
	out.OnlineHours = round1(onlineH)
	out.OfflineHours = round1(offlineH)
	out.UntrackedHours = round1(hours(untracked, from, to))
	// From the unrounded hours, so a few minutes of tracking still counts.
	if tracked := onlineH + offlineH; tracked > 0 {
		u := round3(onlineH / tracked)
		out.Uptime = &u
	}

	for i, b := range a.buckets {
		lo := maxTime(b, from)
		hi := to
		if i+1 < len(a.buckets) {
			hi = minTime(a.buckets[i+1], to)
		}
		out.Series = append(out.Series, UptimePoint{
			Bucket:         a.bucketLabel(i),
			OnlineHours:    round2(hours(online, lo, hi)),
			OfflineHours:   round2(hours(offline, lo, hi)),
			UntrackedHours: round2(hours(untracked, lo, hi)),
		})
	}

	out.Outages = len(offline)
	for _, iv := range offline {
		if h := iv.to.Sub(iv.from).Hours(); h > out.LongestOutageHours {
			out.LongestOutageHours = h
		}
	}
	out.LongestOutageHours = round1(out.LongestOutageHours)
	sort.Slice(offline, func(i, j int) bool { return offline[i].from.After(offline[j].from) })
	for i, iv := range offline {
		if i == maxOutagesListed {
			break
		}
		out.OutageList = append(out.OutageList, Outage{
			From:    iv.from.In(loc).Format(time.RFC3339),
			To:      iv.to.In(loc).Format(time.RFC3339),
			Hours:   round2(iv.to.Sub(iv.from).Hours()),
			Ongoing: iv.to.Equal(to),
		})
	}
	return out, nil
}
