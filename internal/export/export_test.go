package export

import (
	"bytes"
	"encoding/csv"
	"strings"
	"testing"
	"time"

	"github.com/jeffwnelson/sodatracker/internal/domain"
)

func testData() ([]domain.Dispense, map[string]domain.Product) {
	products := map[string]domain.Product{
		"p1": {ID: "p1", UPC: "111222333444", Name: "Cola", CostPerCan: 0.50},
		"p2": {ID: "p2", UPC: "555666777888", Name: "Sprite", CostPerCan: 0.45},
	}
	dispenses := []domain.Dispense{
		{ID: "d1", MachineID: "m1", ProductID: "p1", SlotPosition: 1, SourceType: "gpio", Timestamp: time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)},
		{ID: "d2", MachineID: "m1", ProductID: "p2", SlotPosition: 2, SourceType: "gpio", Timestamp: time.Date(2026, 1, 15, 11, 0, 0, 0, time.UTC)},
		{ID: "d3", MachineID: "m1", ProductID: "p1", SlotPosition: 1, SourceType: "manual", Timestamp: time.Date(2026, 1, 16, 9, 0, 0, 0, time.UTC)},
	}
	return dispenses, products
}

func TestWriteDispensesCSV(t *testing.T) {
	dispenses, products := testData()
	var buf bytes.Buffer
	if err := WriteDispensesCSV(&buf, dispenses, products); err != nil {
		t.Fatalf("WriteDispensesCSV: %v", err)
	}

	r := csv.NewReader(strings.NewReader(buf.String()))
	records, err := r.ReadAll()
	if err != nil {
		t.Fatalf("parse CSV: %v", err)
	}

	// Header + 3 data rows
	if len(records) != 4 {
		t.Fatalf("expected 4 rows (header + 3 data), got %d", len(records))
	}

	// Check header
	if records[0][0] != "Timestamp" || records[0][3] != "Product Name" || records[0][6] != "Cost" {
		t.Errorf("unexpected header: %v", records[0])
	}

	// Check first data row
	if records[1][3] != "Cola" {
		t.Errorf("expected Cola, got %s", records[1][3])
	}
	if records[1][6] != "0.50" {
		t.Errorf("expected cost 0.50, got %s", records[1][6])
	}

	// Check second row product
	if records[2][3] != "Sprite" {
		t.Errorf("expected Sprite, got %s", records[2][3])
	}
}

func TestWriteDispensesPDF(t *testing.T) {
	dispenses, products := testData()
	var buf bytes.Buffer
	if err := WriteDispensesPDF(&buf, dispenses, products, "Test Machine"); err != nil {
		t.Fatalf("WriteDispensesPDF: %v", err)
	}

	// Verify non-empty and valid PDF header
	if buf.Len() == 0 {
		t.Fatal("expected non-empty PDF output")
	}
	if !strings.HasPrefix(buf.String(), "%PDF-") {
		t.Errorf("expected PDF header, got: %q", buf.String()[:20])
	}
}

func TestEmptyExport(t *testing.T) {
	empty := []domain.Dispense{}
	products := map[string]domain.Product{}

	// CSV with zero dispenses
	var csvBuf bytes.Buffer
	if err := WriteDispensesCSV(&csvBuf, empty, products); err != nil {
		t.Fatalf("empty CSV: %v", err)
	}
	r := csv.NewReader(strings.NewReader(csvBuf.String()))
	records, err := r.ReadAll()
	if err != nil {
		t.Fatalf("parse empty CSV: %v", err)
	}
	if len(records) != 1 {
		t.Errorf("expected 1 row (header only), got %d", len(records))
	}

	// PDF with zero dispenses
	var pdfBuf bytes.Buffer
	if err := WriteDispensesPDF(&pdfBuf, empty, products, "Empty Machine"); err != nil {
		t.Fatalf("empty PDF: %v", err)
	}
	if pdfBuf.Len() == 0 {
		t.Fatal("expected non-empty PDF output for empty dispenses")
	}
	if !strings.HasPrefix(pdfBuf.String(), "%PDF-") {
		t.Error("expected valid PDF header for empty dispenses")
	}
}
