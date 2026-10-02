package sqlite

import (
	"path/filepath"
	"testing"
)

// The driver ignores pragma parameters it doesn't recognise, so check that
// these took effect.
func TestPragmasApplied(t *testing.T) {
	s, err := New(filepath.Join(t.TempDir(), "x.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	var mode string
	var timeout int
	if err := s.db.QueryRow("PRAGMA journal_mode").Scan(&mode); err != nil || mode != "wal" {
		t.Errorf("journal_mode = %q, %v; want wal", mode, err)
	}
	if err := s.db.QueryRow("PRAGMA busy_timeout").Scan(&timeout); err != nil || timeout != 5000 {
		t.Errorf("busy_timeout = %d, %v; want 5000", timeout, err)
	}
}
