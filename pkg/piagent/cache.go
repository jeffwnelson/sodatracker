package piagent

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// CachedEvent represents a dispense event stored locally for retry.
type CachedEvent struct {
	ID        int64
	MachineID string
	Slot      int
	Timestamp time.Time
}

// Cache provides local SQLite storage for failed dispense events.
type Cache struct {
	db *sql.DB
}

func NewCache(dbPath string) (*Cache, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open cache db: %w", err)
	}
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

func (c *Cache) Pending() ([]CachedEvent, error) {
	rows, err := c.db.Query(`SELECT id, payload FROM pending_events ORDER BY created_at ASC LIMIT 50`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var events []CachedEvent
	for rows.Next() {
		var id int64
		var payload string
		if err := rows.Scan(&id, &payload); err != nil {
			return nil, err
		}
		var evt CachedEvent
		if err := json.Unmarshal([]byte(payload), &evt); err != nil {
			continue
		}
		evt.ID = id
		events = append(events, evt)
	}
	return events, rows.Err()
}

func (c *Cache) Remove(id int64) error {
	_, err := c.db.Exec(`DELETE FROM pending_events WHERE id = ?`, id)
	return err
}

func (c *Cache) Close() error {
	return c.db.Close()
}
