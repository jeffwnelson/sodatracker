package piagent

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

// CachedEvent is a dispense waiting to be delivered to the API. EventID is
// sent as the dispense's id, so a resend after a lost response is recognized
// by the server rather than counted again.
type CachedEvent struct {
	ID        int64 // row id in the cache
	EventID   string
	MachineID string
	Slot      int
	Timestamp time.Time
}

// Cache is the agent's outbox: every dispense is written here before it is
// sent and removed once the API has it, so a crash or outage loses nothing.
type Cache struct {
	db *sql.DB
}

func NewCache(dbPath string) (*Cache, error) {
	db, err := sql.Open("sqlite", dbPath+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, fmt.Errorf("open cache db: %w", err)
	}
	// The poll loop writes and the sender reads and deletes; one connection
	// serializes them instead of trading SQLITE_BUSY errors.
	db.SetMaxOpenConns(1)
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS pending_events (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		payload TEXT NOT NULL,
		created_at DATETIME NOT NULL
	)`)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("create cache table: %w", err)
	}
	return &Cache{db: db}, nil
}

func (c *Cache) Store(evt CachedEvent) error {
	payload, _ := json.Marshal(evt)
	_, err := c.db.Exec(`INSERT INTO pending_events (payload, created_at) VALUES (?, ?)`,
		string(payload), time.Now().UTC())
	return err
}

// Pending returns up to limit events, oldest first. Rows that can't be parsed
// are deleted, since they could never be sent and would otherwise hold the
// queue. Rows cached before events had IDs are given one, saved before it is
// first sent, so their retries are idempotent too.
func (c *Cache) Pending(limit int) ([]CachedEvent, error) {
	rows, err := c.db.Query(`SELECT id, payload FROM pending_events ORDER BY id LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	var events []CachedEvent
	var bad []int64
	for rows.Next() {
		var id int64
		var payload string
		if err := rows.Scan(&id, &payload); err != nil {
			rows.Close()
			return nil, err
		}
		var evt CachedEvent
		if err := json.Unmarshal([]byte(payload), &evt); err != nil {
			slog.Error("dropping unreadable cached event", "rowID", id, "payload", payload, "error", err)
			bad = append(bad, id)
			continue
		}
		evt.ID = id
		events = append(events, evt)
	}
	err = rows.Err()
	rows.Close() // release the one connection before the writes below
	if err != nil {
		return nil, err
	}

	for _, id := range bad {
		if err := c.Remove(id); err != nil {
			return nil, err
		}
	}
	for i := range events {
		if events[i].EventID != "" {
			continue
		}
		events[i].EventID = uuid.NewString()
		payload, _ := json.Marshal(events[i])
		if _, err := c.db.Exec(`UPDATE pending_events SET payload = ? WHERE id = ?`, string(payload), events[i].ID); err != nil {
			return nil, err
		}
	}
	return events, nil
}

func (c *Cache) Remove(id int64) error {
	_, err := c.db.Exec(`DELETE FROM pending_events WHERE id = ?`, id)
	return err
}

func (c *Cache) Close() error {
	return c.db.Close()
}
