package notify

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jeffwnelson/sodatracker/internal/events"
	"github.com/jeffwnelson/sodatracker/internal/store"
)

// RegisterSlack subscribes Slack delivery to the stock alert events. It is a
// no-op when the client is nil, so an unconfigured webhook costs nothing.
//
// Delivery rides on the existing alert engine, which already deduplicates
// while an alert is active and clears it on restock — so a slot that keeps
// dispensing past its threshold produces one message, not one per can.
func RegisterSlack(bus events.EventBus, st store.Store, c *SlackClient) {
	if c == nil || bus == nil {
		return
	}

	post := func(ctx context.Context, m Message) error {
		if err := c.Post(ctx, m); err != nil {
			// A chat outage must never fail the dispense that triggered it.
			slog.Error("slack notification failed", "error", err)
		}
		return nil
	}

	bus.Subscribe(events.EventOutOfStock, func(ctx context.Context, evt events.Event) error {
		d := describe(ctx, st, evt)
		return post(ctx, Message{
			Text: fmt.Sprintf("Out of stock: %s in %s", d.product, d.machine),
			Blocks: []Block{
				{Type: "section", Text: mrkdwn(":red_circle: *Out of stock* — %s", d.product)},
				{Type: "context", Elements: []TextObject{
					{Type: "mrkdwn", Text: fmt.Sprintf("%s · slot %d", d.machine, d.slot)},
				}},
			},
		})
	})

	bus.Subscribe(events.EventLowStock, func(ctx context.Context, evt events.Event) error {
		d := describe(ctx, st, evt)
		return post(ctx, Message{
			Text: fmt.Sprintf("Low stock: %s in %s (%d left)", d.product, d.machine, d.stock),
			Blocks: []Block{
				{Type: "section", Text: mrkdwn(":large_yellow_circle: *Running low* — %s", d.product)},
				{Type: "context", Elements: []TextObject{
					{Type: "mrkdwn", Text: fmt.Sprintf("%s · slot %d · %d left (threshold %d)", d.machine, d.slot, d.stock, d.threshold)},
				}},
			},
		})
	})

	// EventStockCleared is deliberately not subscribed: CreateRestock publishes
	// it once per slot, so a five-slot refill would post five messages. A single
	// "restocked" notification would need the event to carry the whole slot list.
}

// details holds the human-readable fields a message needs. Events carry only
// IDs, so names are looked up here rather than widening the event payload.
type details struct {
	machine   string
	product   string
	slot      int
	stock     int
	threshold int
}

func describe(ctx context.Context, st store.Store, evt events.Event) details {
	d := details{
		machine:   "the machine",
		product:   "a slot",
		slot:      intField(evt, "slotPosition"),
		stock:     intField(evt, "currentStock"),
		threshold: intField(evt, "threshold"),
	}

	if id, ok := evt.Data["machineId"].(string); ok && id != "" {
		if m, err := st.GetMachine(ctx, id); err == nil && m != nil && m.Name != "" {
			d.machine = m.Name
		}
	}
	if id, ok := evt.Data["productId"].(string); ok && id != "" {
		if p, err := st.GetProduct(ctx, id); err == nil && p != nil && p.Name != "" {
			d.product = p.Name
		}
	}
	if d.product == "a slot" && d.slot > 0 {
		d.product = fmt.Sprintf("slot %d", d.slot)
	}
	return d
}

// intField reads a numeric event field, which arrives as int in-process and as
// float64 once it has been through JSON.
func intField(evt events.Event, key string) int {
	switch v := evt.Data[key].(type) {
	case int:
		return v
	case float64:
		return int(v)
	}
	return 0
}
