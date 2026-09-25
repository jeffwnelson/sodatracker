// Package slackcmd implements the /soda slash command. It is transport-free:
// the signed HTTP endpoint and the Socket Mode client both call Run.
package slackcmd

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jeffwnelson/sodatracker/internal/service"
	"github.com/jeffwnelson/sodatracker/internal/store"
)

// Commands answers /soda subcommands from the store and service.
type Commands struct {
	store store.Store
	svc   *service.Service
}

func New(st store.Store, svc *service.Service) *Commands {
	return &Commands{store: st, svc: svc}
}

// Run answers one /soda invocation. text is everything after the command;
// an empty text means status.
func (c *Commands) Run(ctx context.Context, text string) Response {
	args := strings.Fields(text)
	sub := "status"
	if len(args) > 0 {
		sub = strings.ToLower(args[0])
	}

	switch sub {
	case "status":
		return c.status(ctx)
	case "top":
		return c.top(ctx)
	case "help":
		return Ephemeral(HelpText)
	default:
		return Ephemeral(fmt.Sprintf("Unknown command %q.\n\n%s", sub, HelpText))
	}
}

// Response is the immediate reply to a slash command. "ephemeral" shows
// only to the caller; "in_channel" posts to everyone in the channel.
type Response struct {
	ResponseType string `json:"response_type"`
	Text         string `json:"text"`
}

func Ephemeral(text string) Response {
	return Response{ResponseType: "ephemeral", Text: text}
}

func InChannel(text string) Response {
	return Response{ResponseType: "in_channel", Text: text}
}

const HelpText = "*SodaTracker*\n" +
	"`/soda status` — what's in the machine right now\n" +
	"`/soda top` — most dispensed over the last 7 days\n" +
	"`/soda help` — this message"

// status reports current stock for every machine.
func (c *Commands) status(ctx context.Context) Response {
	machines, err := c.store.ListMachines(ctx)
	if err != nil {
		return Ephemeral("Couldn't reach SodaTracker just now.")
	}
	if len(machines) == 0 {
		return Ephemeral("No machines are configured yet.")
	}

	var b strings.Builder
	for i, m := range machines {
		if i > 0 {
			b.WriteString("\n")
		}
		status, err := c.svc.GetMachineStatus(ctx, m.ID)
		if err != nil {
			continue
		}
		fmt.Fprintf(&b, "*%s*\n", m.Name)
		for _, s := range status.Slots {
			name := fmt.Sprintf("Slot %d", s.Slot.Position)
			if s.Product != nil && s.Product.Name != "" {
				name = s.Product.Name
			}
			fmt.Fprintf(&b, "%s %s — %d left\n", statusIcon(s.Status), name, s.CurrentStock)
		}
	}
	return InChannel(strings.TrimRight(b.String(), "\n"))
}

func statusIcon(status string) string {
	switch status {
	case "ok":
		return ":large_green_circle:"
	case "low":
		return ":large_yellow_circle:"
	default:
		return ":red_circle:"
	}
}

// top ranks products by dispense count over the last 7 days.
func (c *Commands) top(ctx context.Context) Response {
	machines, err := c.store.ListMachines(ctx)
	if err != nil || len(machines) == 0 {
		return Ephemeral("No dispense data yet.")
	}

	since := time.Now().AddDate(0, 0, -7)
	counts := map[string]int{}
	for _, m := range machines {
		stats, err := c.svc.GetDispenseStats(ctx, m.ID, since)
		if err != nil {
			continue
		}
		for _, p := range stats.TopProducts {
			counts[p.ProductName] += p.Count
		}
	}
	if len(counts) == 0 {
		return Ephemeral("Nothing has been dispensed in the last 7 days.")
	}

	type row struct {
		name  string
		count int
	}
	rows := make([]row, 0, len(counts))
	for name, count := range counts {
		rows = append(rows, row{name, count})
	}
	// Sort by count, then name, so equal counts do not shuffle between calls.
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].count != rows[j].count {
			return rows[i].count > rows[j].count
		}
		return rows[i].name < rows[j].name
	})
	if len(rows) > 5 {
		rows = rows[:5]
	}

	var b strings.Builder
	b.WriteString("*Most dispensed, last 7 days*\n")
	for i, r := range rows {
		fmt.Fprintf(&b, "%d. %s — %d\n", i+1, r.name, r.count)
	}
	return InChannel(strings.TrimRight(b.String(), "\n"))
}
