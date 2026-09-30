package piagent

import "time"

// debouncer turns raw pin samples from a vend contact into one event per vend.
//
// The contacts on the machine are bare wires that an arm presses together
// while the vend cam turns, so a single vend can bounce open and closed
// several times. A closure only counts once the pin has read LOW (contact
// closed) continuously for minClosed, which rejects brief blips from wires
// being handled. It ends, and the vend is reported, only after the pin has
// read HIGH continuously for minOpen, so mid-vend bounces shorter than that
// are ignored.
type debouncer struct {
	minClosed time.Duration
	minOpen   time.Duration

	started  bool
	rawLow   bool      // last raw sample: true when the contact is closed
	since    time.Time // when rawLow last changed
	closed   bool      // debounced state
	closedAt time.Time // when the current debounced closure began
	armed    bool      // false while a closure that began before the agent started is still open
}

func newDebouncer(minClosed, minOpen time.Duration) *debouncer {
	return &debouncer{minClosed: minClosed, minOpen: minOpen}
}

// update feeds one sample (high is the raw pin level) taken at now. It
// returns true, with the time the contact first closed, when a vend has
// completed.
func (d *debouncer) update(high bool, now time.Time) (bool, time.Time) {
	low := !high
	if !d.started {
		// A contact already closed at startup belongs to a vend we did not
		// see begin, or to a stuck contact; don't report it when it opens.
		d.started = true
		d.rawLow = low
		d.since = now
		d.closed = low
		d.armed = !low
		return false, time.Time{}
	}

	if low != d.rawLow {
		d.rawLow = low
		d.since = now
	}
	stable := now.Sub(d.since)

	switch {
	case !d.closed && d.rawLow && stable >= d.minClosed:
		d.closed = true
		d.closedAt = d.since
	case d.closed && !d.rawLow && stable >= d.minOpen:
		d.closed = false
		counted := d.armed
		d.armed = true
		if counted {
			return true, d.closedAt
		}
	}
	return false, time.Time{}
}
