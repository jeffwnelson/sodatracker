package piagent

import (
	"testing"
	"time"
)

// edge is a raw pin transition: at offsetMs the pin goes LOW (closed) or HIGH.
type edge struct {
	offsetMs int
	low      bool
}

// simulate samples a pin every pollMs from 0 to endMs, applying edges in
// order, and returns the closure times (as offsets) of reported vends.
func simulate(t *testing.T, pollMs, endMs int, edges []edge) []int {
	t.Helper()
	d := newDebouncer(100*time.Millisecond, 500*time.Millisecond)
	base := time.Date(2026, 9, 30, 13, 0, 0, 0, time.UTC)
	low := false
	next := 0
	var vends []int
	for ms := 0; ms <= endMs; ms += pollMs {
		for next < len(edges) && edges[next].offsetMs <= ms {
			low = edges[next].low
			next++
		}
		if ok, at := d.update(!low, base.Add(time.Duration(ms)*time.Millisecond)); ok {
			vends = append(vends, int(at.Sub(base)/time.Millisecond))
		}
	}
	return vends
}

func TestDebouncer(t *testing.T) {
	tests := []struct {
		name  string
		edges []edge
		want  int
	}{
		{
			name:  "clean vend",
			edges: []edge{{1000, true}, {2800, false}},
			want:  1,
		},
		{
			// Coke Zero, 2026-09-30 13:30:36: 0.4 s closed, open 107 ms, 1.05 s closed.
			name:  "bounce mid-vend",
			edges: []edge{{1000, true}, {1407, false}, {1514, true}, {2564, false}},
			want:  1,
		},
		{
			// Diet Coke, 2026-09-30 13:30:55: a clean vend, then 1.4 s later a
			// second one that chatters as the arm meets the wires.
			name: "two vends, second with chatter",
			edges: []edge{
				{1000, true}, {2991, false},
				{4412, true}, {4630, false}, {4683, true}, {4737, false},
				{4818, true}, {4871, false}, {5141, true}, {6523, false},
			},
			want: 2,
		},
		{
			name:  "short handling blips are ignored",
			edges: []edge{{1000, true}, {1050, false}, {1500, true}, {1560, false}},
			want:  0,
		},
		{
			name:  "still closed at end is not reported",
			edges: []edge{{1000, true}},
			want:  0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, pollMs := range []int{20, 100} {
				got := simulate(t, pollMs, 9000, tt.edges)
				if len(got) != tt.want {
					t.Errorf("pollMs=%d: got %d vends %v, want %d", pollMs, len(got), got, tt.want)
				}
			}
		})
	}
}

func TestDebouncerReportsClosureTime(t *testing.T) {
	got := simulate(t, 20, 5000, []edge{{1000, true}, {1407, false}, {1514, true}, {2564, false}})
	if len(got) != 1 || got[0] != 1000 {
		t.Fatalf("got %v, want one vend at 1000 ms", got)
	}
}

func TestDebouncerIgnoresClosureInProgressAtStartup(t *testing.T) {
	d := newDebouncer(100*time.Millisecond, 500*time.Millisecond)
	base := time.Date(2026, 9, 30, 13, 0, 0, 0, time.UTC)
	at := func(ms int) time.Time { return base.Add(time.Duration(ms) * time.Millisecond) }

	d.update(false, at(0)) // contact already closed when the agent starts
	if ok, _ := d.update(true, at(200)); ok {
		t.Fatal("reported on release")
	}
	if ok, _ := d.update(true, at(800)); ok {
		t.Fatal("reported the closure that began before startup")
	}
	// The next full vend counts.
	d.update(false, at(1000))
	d.update(false, at(1200))
	d.update(true, at(2000))
	if ok, _ := d.update(true, at(2600)); !ok {
		t.Fatal("did not report the first full vend after startup")
	}
}
