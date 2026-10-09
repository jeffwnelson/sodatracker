package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jeffwnelson/sodatracker/internal/domain"
	"github.com/jeffwnelson/sodatracker/internal/store"
	_ "modernc.org/sqlite"
)

type SQLiteStore struct {
	db *sql.DB
}

func New(dbPath string) (*SQLiteStore, error) {
	// modernc.org/sqlite takes pragmas as _pragma=name(value), applied to
	// every pooled connection. (mattn/go-sqlite3's _journal_mode and
	// _busy_timeout are silently ignored here, which until v0.11 left the
	// database in rollback-journal mode with no busy timeout.)
	db, err := sql.Open("sqlite", dbPath+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	s := &SQLiteStore{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return s, nil
}

func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

func (s *SQLiteStore) migrate() error {
	migrations := []string{
		`CREATE TABLE IF NOT EXISTS products (
			id TEXT PRIMARY KEY,
			upc TEXT DEFAULT '',
			name TEXT NOT NULL,
			cost_per_can REAL DEFAULT 0,
			image_url TEXT DEFAULT '',
			description TEXT DEFAULT '',
			color TEXT DEFAULT ''
		)`,
		`CREATE TABLE IF NOT EXISTS machines (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			location TEXT DEFAULT '',
			slot_count INTEGER NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS slots (
			machine_id TEXT NOT NULL,
			position INTEGER NOT NULL,
			product_id TEXT DEFAULT '',
			gpio_pin INTEGER DEFAULT 0,
			capacity INTEGER NOT NULL DEFAULT 10,
			low_threshold INTEGER NOT NULL DEFAULT 2,
			PRIMARY KEY (machine_id, position)
		)`,
		`CREATE TABLE IF NOT EXISTS dispenses (
			id TEXT PRIMARY KEY,
			machine_id TEXT NOT NULL,
			product_id TEXT NOT NULL,
			slot_position INTEGER NOT NULL,
			source_type TEXT NOT NULL DEFAULT 'gpio',
			timestamp DATETIME NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_dispenses_machine_ts ON dispenses(machine_id, timestamp)`,
		`CREATE INDEX IF NOT EXISTS idx_dispenses_machine_slot_ts ON dispenses(machine_id, slot_position, timestamp)`,
		`CREATE TABLE IF NOT EXISTS restocks (
			id TEXT PRIMARY KEY,
			machine_id TEXT NOT NULL,
			user_id TEXT DEFAULT '',
			slots_json TEXT NOT NULL,
			timestamp DATETIME NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_restocks_machine_ts ON restocks(machine_id, timestamp)`,
		`CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY,
			username TEXT UNIQUE NOT NULL,
			password_hash TEXT NOT NULL,
			role TEXT NOT NULL DEFAULT 'admin'
		)`,
		`CREATE TABLE IF NOT EXISTS alert_configs (
			id TEXT PRIMARY KEY,
			machine_id TEXT NOT NULL,
			slot_position INTEGER NOT NULL DEFAULT 0,
			low_threshold INTEGER NOT NULL DEFAULT -1,
			enabled INTEGER NOT NULL DEFAULT 1,
			UNIQUE(machine_id, slot_position)
		)`,
		`CREATE TABLE IF NOT EXISTS alert_events (
			id TEXT PRIMARY KEY,
			machine_id TEXT NOT NULL,
			slot_position INTEGER NOT NULL,
			product_id TEXT NOT NULL DEFAULT '',
			alert_type TEXT NOT NULL,
			current_stock INTEGER NOT NULL DEFAULT 0,
			threshold INTEGER NOT NULL DEFAULT 0,
			cleared INTEGER NOT NULL DEFAULT 0,
			timestamp DATETIME NOT NULL,
			cleared_at DATETIME
		)`,
		`CREATE INDEX IF NOT EXISTS idx_alert_events_machine_cleared_ts ON alert_events(machine_id, cleared, timestamp)`,
		`CREATE TABLE IF NOT EXISTS agent_sessions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			machine_id TEXT NOT NULL,
			started_at DATETIME NOT NULL,
			last_seen DATETIME NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_agent_sessions_machine ON agent_sessions(machine_id, started_at)`,
		`CREATE TABLE IF NOT EXISTS settings (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL,
			secret INTEGER NOT NULL DEFAULT 0
		)`,
		// The not-registering (quiet) alerts were removed in v0.17; close any
		// still open and drop their settings. A beam-break sensor will take
		// their place.
		`UPDATE alert_events SET cleared = 1, cleared_at = datetime('now')
		 WHERE cleared = 0 AND alert_type IN ('quiet_slot', 'quiet_machine')`,
		`DELETE FROM settings WHERE key LIKE 'alerts.%'`,
	}
	for _, m := range migrations {
		if _, err := s.db.Exec(m); err != nil {
			return fmt.Errorf("exec migration: %w", err)
		}
	}

	// Schema evolution for existing databases: add description column if missing.
	s.db.Exec(`ALTER TABLE products ADD COLUMN description TEXT DEFAULT ''`)

	// Schema evolution: remove UNIQUE constraint on upc for existing databases.
	if err := s.migrateProductsDropUPCUnique(); err != nil {
		return fmt.Errorf("migrate products upc: %w", err)
	}

	// Schema evolution: product color. After the UPC migration, which rebuilds
	// the table without it; errors here just mean the column already exists.
	s.db.Exec(`ALTER TABLE products ADD COLUMN color TEXT DEFAULT ''`)
	// Schema evolution: can size and caffeine, for the Highlights totals.
	s.db.Exec(`ALTER TABLE products ADD COLUMN size_oz REAL DEFAULT 12`)
	s.db.Exec(`ALTER TABLE products ADD COLUMN caffeine_mg REAL DEFAULT 0`)

	return nil
}

// migrateProductsDropUPCUnique recreates the products table without the UNIQUE
// constraint on upc, if the constraint still exists. This is idempotent.
func (s *SQLiteStore) migrateProductsDropUPCUnique() error {
	var tableSql string
	err := s.db.QueryRow("SELECT sql FROM sqlite_master WHERE type='table' AND name='products'").Scan(&tableSql)
	if err != nil {
		return nil // table doesn't exist, skip
	}
	if !strings.Contains(tableSql, "UNIQUE") {
		return nil // already migrated
	}

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmts := []string{
		`CREATE TABLE products_new (
			id TEXT PRIMARY KEY,
			upc TEXT DEFAULT '',
			name TEXT NOT NULL,
			cost_per_can REAL DEFAULT 0,
			image_url TEXT DEFAULT '',
			description TEXT DEFAULT ''
		)`,
		`INSERT INTO products_new (id, upc, name, cost_per_can, image_url, description)
		 SELECT id, upc, name, cost_per_can, image_url, COALESCE(description, '') FROM products`,
		`DROP TABLE products`,
		`ALTER TABLE products_new RENAME TO products`,
	}
	for _, stmt := range stmts {
		if _, err := tx.Exec(stmt); err != nil {
			return fmt.Errorf("products upc migration: %w", err)
		}
	}
	return tx.Commit()
}

// --- Products ---

func (s *SQLiteStore) CreateProduct(ctx context.Context, p *domain.Product) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO products (id, upc, name, cost_per_can, image_url, description, color, size_oz, caffeine_mg) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET upc=excluded.upc, name=excluded.name, cost_per_can=excluded.cost_per_can, image_url=excluded.image_url, description=excluded.description, color=excluded.color, size_oz=excluded.size_oz, caffeine_mg=excluded.caffeine_mg`,
		p.ID, p.UPC, p.Name, p.CostPerCan, p.ImageURL, p.Description, p.Color, p.SizeOz, p.CaffeineMg)
	return err
}

func (s *SQLiteStore) GetProduct(ctx context.Context, id string) (*domain.Product, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id, upc, name, cost_per_can, image_url, description, COALESCE(color, ''), COALESCE(size_oz, 12), COALESCE(caffeine_mg, 0) FROM products WHERE id = ?`, id)
	return scanProduct(row)
}

func (s *SQLiteStore) GetProductByUPC(ctx context.Context, upc string) (*domain.Product, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id, upc, name, cost_per_can, image_url, description, COALESCE(color, ''), COALESCE(size_oz, 12), COALESCE(caffeine_mg, 0) FROM products WHERE upc = ?`, upc)
	return scanProduct(row)
}

func (s *SQLiteStore) ListProducts(ctx context.Context) ([]domain.Product, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, upc, name, cost_per_can, image_url, description, COALESCE(color, ''), COALESCE(size_oz, 12), COALESCE(caffeine_mg, 0) FROM products ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var products []domain.Product
	for rows.Next() {
		var p domain.Product
		if err := rows.Scan(&p.ID, &p.UPC, &p.Name, &p.CostPerCan, &p.ImageURL, &p.Description, &p.Color, &p.SizeOz, &p.CaffeineMg); err != nil {
			return nil, err
		}
		products = append(products, p)
	}
	return products, rows.Err()
}

func (s *SQLiteStore) DeleteProduct(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM products WHERE id = ?`, id)
	return err
}

func scanProduct(row *sql.Row) (*domain.Product, error) {
	var p domain.Product
	if err := row.Scan(&p.ID, &p.UPC, &p.Name, &p.CostPerCan, &p.ImageURL, &p.Description, &p.Color, &p.SizeOz, &p.CaffeineMg); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &p, nil
}

// --- Machines ---

func (s *SQLiteStore) CreateMachine(ctx context.Context, m *domain.Machine) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO machines (id, name, location, slot_count) VALUES (?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET name=excluded.name, location=excluded.location, slot_count=excluded.slot_count`,
		m.ID, m.Name, m.Location, m.SlotCount)
	return err
}

func (s *SQLiteStore) GetMachine(ctx context.Context, id string) (*domain.Machine, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id, name, location, slot_count FROM machines WHERE id = ?`, id)
	var m domain.Machine
	if err := row.Scan(&m.ID, &m.Name, &m.Location, &m.SlotCount); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &m, nil
}

func (s *SQLiteStore) ListMachines(ctx context.Context) ([]domain.Machine, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, name, location, slot_count FROM machines ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var machines []domain.Machine
	for rows.Next() {
		var m domain.Machine
		if err := rows.Scan(&m.ID, &m.Name, &m.Location, &m.SlotCount); err != nil {
			return nil, err
		}
		machines = append(machines, m)
	}
	return machines, rows.Err()
}

func (s *SQLiteStore) DeleteMachine(ctx context.Context, id string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	tables := []string{
		`DELETE FROM alert_events WHERE machine_id = ?`,
		`DELETE FROM agent_sessions WHERE machine_id = ?`,
		`DELETE FROM alert_configs WHERE machine_id = ?`,
		`DELETE FROM restocks WHERE machine_id = ?`,
		`DELETE FROM dispenses WHERE machine_id = ?`,
		`DELETE FROM slots WHERE machine_id = ?`,
		`DELETE FROM machines WHERE id = ?`,
	}
	for _, stmt := range tables {
		if _, err := tx.ExecContext(ctx, stmt, id); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// --- Slots ---

func (s *SQLiteStore) SetSlots(ctx context.Context, machineID string, slots []domain.Slot) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DELETE FROM slots WHERE machine_id = ?`, machineID); err != nil {
		return err
	}
	for _, sl := range slots {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO slots (machine_id, position, product_id, gpio_pin, capacity, low_threshold) VALUES (?, ?, ?, ?, ?, ?)`,
			machineID, sl.Position, sl.ProductID, sl.GPIOPin, sl.Capacity, sl.LowThreshold); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *SQLiteStore) GetSlots(ctx context.Context, machineID string) ([]domain.Slot, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT machine_id, position, product_id, gpio_pin, capacity, low_threshold FROM slots WHERE machine_id = ? ORDER BY position`,
		machineID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var slots []domain.Slot
	for rows.Next() {
		var sl domain.Slot
		if err := rows.Scan(&sl.MachineID, &sl.Position, &sl.ProductID, &sl.GPIOPin, &sl.Capacity, &sl.LowThreshold); err != nil {
			return nil, err
		}
		slots = append(slots, sl)
	}
	return slots, rows.Err()
}

// --- Dispenses ---

// Times are stored as text and compared as text, so they are always written
// in UTC; a mix of offsets would sort and filter wrongly.
func (s *SQLiteStore) CreateDispense(ctx context.Context, d *domain.Dispense) error {
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO dispenses (id, machine_id, product_id, slot_position, source_type, timestamp) VALUES (?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO NOTHING`,
		d.ID, d.MachineID, d.ProductID, d.SlotPosition, d.SourceType, d.Timestamp.UTC())
	if err != nil {
		return err
	}
	if n, err := res.RowsAffected(); err == nil && n == 0 {
		return store.ErrDuplicate
	}
	return nil
}

func (s *SQLiteStore) ListDispenses(ctx context.Context, machineID string, since time.Time, limit int) ([]domain.Dispense, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, machine_id, product_id, slot_position, source_type, timestamp
		 FROM dispenses WHERE machine_id = ? AND timestamp >= ? ORDER BY timestamp DESC LIMIT ?`,
		machineID, since, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var dispenses []domain.Dispense
	for rows.Next() {
		var d domain.Dispense
		if err := rows.Scan(&d.ID, &d.MachineID, &d.ProductID, &d.SlotPosition, &d.SourceType, &d.Timestamp); err != nil {
			return nil, err
		}
		dispenses = append(dispenses, d)
	}
	return dispenses, rows.Err()
}

func (s *SQLiteStore) ListDispensesBetween(ctx context.Context, machineID string, from, to time.Time) ([]domain.Dispense, error) {
	query := `SELECT id, machine_id, product_id, slot_position, source_type, timestamp
		 FROM dispenses WHERE machine_id = ? AND timestamp >= ?`
	args := []any{machineID, from.UTC()}
	if !to.IsZero() {
		query += ` AND timestamp < ?`
		args = append(args, to.UTC())
	}
	rows, err := s.db.QueryContext(ctx, query+` ORDER BY timestamp`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var dispenses []domain.Dispense
	for rows.Next() {
		var d domain.Dispense
		if err := rows.Scan(&d.ID, &d.MachineID, &d.ProductID, &d.SlotPosition, &d.SourceType, &d.Timestamp); err != nil {
			return nil, err
		}
		dispenses = append(dispenses, d)
	}
	return dispenses, rows.Err()
}

func (s *SQLiteStore) FirstActivity(ctx context.Context, machineID string) (time.Time, error) {
	// One indexed lookup per table; MIN() would return untyped text.
	var first time.Time
	for _, table := range []string{"dispenses", "restocks"} {
		var t time.Time
		err := s.db.QueryRowContext(ctx,
			`SELECT timestamp FROM `+table+` WHERE machine_id = ? ORDER BY timestamp LIMIT 1`,
			machineID).Scan(&t)
		if err == sql.ErrNoRows {
			continue
		}
		if err != nil {
			return time.Time{}, err
		}
		if first.IsZero() || t.Before(first) {
			first = t
		}
	}
	return first, nil
}

func (s *SQLiteStore) CountDispensesSince(ctx context.Context, machineID string, slotPosition int, since time.Time) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM dispenses WHERE machine_id = ? AND slot_position = ? AND timestamp >= ?`,
		machineID, slotPosition, since).Scan(&count)
	return count, err
}

// --- Restocks ---

func (s *SQLiteStore) CreateRestock(ctx context.Context, r *domain.Restock) error {
	slotsJSON, err := json.Marshal(r.Slots)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO restocks (id, machine_id, user_id, slots_json, timestamp) VALUES (?, ?, ?, ?, ?)`,
		r.ID, r.MachineID, r.UserID, string(slotsJSON), r.Timestamp.UTC())
	return err
}

func (s *SQLiteStore) GetLastRestock(ctx context.Context, machineID string, slotPosition int) (*domain.Restock, *domain.RestockSlot, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, machine_id, user_id, slots_json, timestamp FROM restocks
		 WHERE machine_id = ? ORDER BY timestamp DESC`,
		machineID)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	for rows.Next() {
		r, err := scanRestock(rows)
		if err != nil {
			return nil, nil, err
		}
		for i := range r.Slots {
			if r.Slots[i].Position == slotPosition {
				return r, &r.Slots[i], nil
			}
		}
	}
	return nil, nil, rows.Err()
}

func (s *SQLiteStore) ListRestocks(ctx context.Context, machineID string, limit int) ([]domain.Restock, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, machine_id, user_id, slots_json, timestamp FROM restocks
		 WHERE machine_id = ? ORDER BY timestamp DESC LIMIT ?`,
		machineID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var restocks []domain.Restock
	for rows.Next() {
		r, err := scanRestock(rows)
		if err != nil {
			return nil, err
		}
		restocks = append(restocks, *r)
	}
	return restocks, rows.Err()
}

// --- Users ---

func (s *SQLiteStore) CreateUser(ctx context.Context, u *domain.User) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO users (id, username, password_hash, role) VALUES (?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET username=excluded.username, password_hash=excluded.password_hash, role=excluded.role`,
		u.ID, u.Username, u.PasswordHash, u.Role)
	return err
}

func (s *SQLiteStore) GetUserByUsername(ctx context.Context, username string) (*domain.User, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id, username, password_hash, role FROM users WHERE username = ?`, username)
	var u domain.User
	if err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &u, nil
}

// --- Alert Configs ---

func (s *SQLiteStore) CreateAlertConfig(ctx context.Context, ac *domain.AlertConfig) error {
	enabled := 0
	if ac.Enabled {
		enabled = 1
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO alert_configs (id, machine_id, slot_position, low_threshold, enabled) VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(machine_id, slot_position) DO UPDATE SET low_threshold=excluded.low_threshold, enabled=excluded.enabled, id=excluded.id`,
		ac.ID, ac.MachineID, ac.SlotPosition, ac.LowThreshold, enabled)
	return err
}

func (s *SQLiteStore) ListAlertConfigs(ctx context.Context, machineID string) ([]domain.AlertConfig, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, machine_id, slot_position, low_threshold, enabled FROM alert_configs WHERE machine_id = ? ORDER BY slot_position`,
		machineID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var configs []domain.AlertConfig
	for rows.Next() {
		var ac domain.AlertConfig
		var enabled int
		if err := rows.Scan(&ac.ID, &ac.MachineID, &ac.SlotPosition, &ac.LowThreshold, &enabled); err != nil {
			return nil, err
		}
		ac.Enabled = enabled != 0
		configs = append(configs, ac)
	}
	return configs, rows.Err()
}

func (s *SQLiteStore) UpdateAlertConfig(ctx context.Context, ac *domain.AlertConfig) error {
	enabled := 0
	if ac.Enabled {
		enabled = 1
	}
	_, err := s.db.ExecContext(ctx,
		`UPDATE alert_configs SET low_threshold = ?, enabled = ? WHERE id = ?`,
		ac.LowThreshold, enabled, ac.ID)
	return err
}

func (s *SQLiteStore) DeleteAlertConfig(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM alert_configs WHERE id = ?`, id)
	return err
}

// --- Alert Events ---

func (s *SQLiteStore) CreateAlertEvent(ctx context.Context, ae *domain.AlertEvent) error {
	cleared := 0
	if ae.Cleared {
		cleared = 1
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO alert_events (id, machine_id, slot_position, product_id, alert_type, current_stock, threshold, cleared, timestamp, cleared_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		ae.ID, ae.MachineID, ae.SlotPosition, ae.ProductID, ae.AlertType, ae.CurrentStock, ae.Threshold, cleared, ae.Timestamp, ae.ClearedAt)
	return err
}

func (s *SQLiteStore) ListAlertEvents(ctx context.Context, machineID string, activeOnly bool, limit int) ([]domain.AlertEvent, error) {
	query := `SELECT id, machine_id, slot_position, product_id, alert_type, current_stock, threshold, cleared, timestamp, cleared_at
		 FROM alert_events WHERE machine_id = ?`
	args := []any{machineID}
	if activeOnly {
		query += ` AND cleared = 0`
	}
	query += ` ORDER BY timestamp DESC LIMIT ?`
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var events []domain.AlertEvent
	for rows.Next() {
		var ae domain.AlertEvent
		var cleared int
		if err := rows.Scan(&ae.ID, &ae.MachineID, &ae.SlotPosition, &ae.ProductID, &ae.AlertType, &ae.CurrentStock, &ae.Threshold, &cleared, &ae.Timestamp, &ae.ClearedAt); err != nil {
			return nil, err
		}
		ae.Cleared = cleared != 0
		events = append(events, ae)
	}
	return events, rows.Err()
}

func (s *SQLiteStore) ClearAlertEvents(ctx context.Context, machineID string, slotPosition int) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE alert_events SET cleared = 1, cleared_at = datetime('now') WHERE machine_id = ? AND slot_position = ? AND cleared = 0`,
		machineID, slotPosition)
	return err
}

func (s *SQLiteStore) GetActiveAlert(ctx context.Context, machineID string, slotPosition int, alertType string) (*domain.AlertEvent, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, machine_id, slot_position, product_id, alert_type, current_stock, threshold, cleared, timestamp, cleared_at
		 FROM alert_events WHERE machine_id = ? AND slot_position = ? AND alert_type = ? AND cleared = 0 LIMIT 1`,
		machineID, slotPosition, alertType)
	var ae domain.AlertEvent
	var cleared int
	if err := row.Scan(&ae.ID, &ae.MachineID, &ae.SlotPosition, &ae.ProductID, &ae.AlertType, &ae.CurrentStock, &ae.Threshold, &cleared, &ae.Timestamp, &ae.ClearedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	ae.Cleared = cleared != 0
	return &ae, nil
}

// --- Admin ---

// Snapshot uses VACUUM INTO, which copies the database as of one read
// transaction. Copying the file itself could catch it mid-checkpoint, and
// would miss whatever is still in the WAL. Secret settings are then deleted
// from the copy, and it is vacuumed again so their pages don't linger.
func (s *SQLiteStore) Snapshot(ctx context.Context, dest string) error {
	if _, err := s.db.ExecContext(ctx, "VACUUM INTO ?", dest); err != nil {
		return err
	}
	cp, err := sql.Open("sqlite", dest)
	if err != nil {
		return err
	}
	defer cp.Close()
	if _, err := cp.ExecContext(ctx, "DELETE FROM settings WHERE secret = 1"); err != nil {
		return fmt.Errorf("scrub secrets: %w", err)
	}
	if _, err := cp.ExecContext(ctx, "VACUUM"); err != nil {
		return fmt.Errorf("scrub secrets: %w", err)
	}
	return nil
}

// GetSetting returns "" for a setting that was never set.
func (s *SQLiteStore) GetSetting(ctx context.Context, key string) (string, error) {
	var v string
	err := s.db.QueryRowContext(ctx, "SELECT value FROM settings WHERE key = ?", key).Scan(&v)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return v, err
}

func (s *SQLiteStore) SetSetting(ctx context.Context, key, value string, secret bool) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO settings (key, value, secret) VALUES (?, ?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value, secret = excluded.secret`,
		key, value, secret)
	return err
}

func (s *SQLiteStore) ResetAllData(ctx context.Context) error {
	tables := []string{
		"alert_events", "alert_configs", "restocks", "dispenses", "slots", "machines", "products", "users",
	}
	for _, table := range tables {
		if _, err := s.db.ExecContext(ctx, "DELETE FROM "+table); err != nil {
			return fmt.Errorf("clear %s: %w", table, err)
		}
	}
	return nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanRestock(row scanner) (*domain.Restock, error) {
	var r domain.Restock
	var slotsJSON string
	if err := row.Scan(&r.ID, &r.MachineID, &r.UserID, &slotsJSON, &r.Timestamp); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	if err := json.Unmarshal([]byte(slotsJSON), &r.Slots); err != nil {
		return nil, err
	}
	return &r, nil
}

// --- Agent sessions ---

func (s *SQLiteStore) LatestAgentSession(ctx context.Context, machineID string) (*domain.AgentSession, error) {
	var a domain.AgentSession
	err := s.db.QueryRowContext(ctx,
		`SELECT id, machine_id, started_at, last_seen FROM agent_sessions
		 WHERE machine_id = ? ORDER BY started_at DESC, id DESC LIMIT 1`, machineID).
		Scan(&a.ID, &a.MachineID, &a.StartedAt, &a.LastSeen)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (s *SQLiteStore) StartAgentSession(ctx context.Context, machineID string, at time.Time) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO agent_sessions (machine_id, started_at, last_seen) VALUES (?, ?, ?)`,
		machineID, at.UTC(), at.UTC())
	return err
}

func (s *SQLiteStore) TouchAgentSession(ctx context.Context, id int64, at time.Time) error {
	_, err := s.db.ExecContext(ctx, `UPDATE agent_sessions SET last_seen = ? WHERE id = ?`, at.UTC(), id)
	return err
}

func (s *SQLiteStore) ListAgentSessions(ctx context.Context, machineID string) ([]domain.AgentSession, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, machine_id, started_at, last_seen FROM agent_sessions
		 WHERE machine_id = ? ORDER BY started_at, id`, machineID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.AgentSession
	for rows.Next() {
		var a domain.AgentSession
		if err := rows.Scan(&a.ID, &a.MachineID, &a.StartedAt, &a.LastSeen); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
