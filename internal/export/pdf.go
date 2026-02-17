package export

import (
	"fmt"
	"io"

	"github.com/go-pdf/fpdf"
	"github.com/jeffwnelson/sodatracker/internal/domain"
)

// WriteDispensesPDF writes dispense data as a PDF table to w.
func WriteDispensesPDF(w io.Writer, dispenses []domain.Dispense, products map[string]domain.Product, machineName string) error {
	pdf := fpdf.New("L", "mm", "A4", "")
	pdf.SetAutoPageBreak(true, 15)
	pdf.AddPage()

	// Title
	pdf.SetFont("Helvetica", "B", 16)
	pdf.Cell(0, 10, fmt.Sprintf("Dispense Report - %s", machineName))
	pdf.Ln(8)

	// Date range subtitle
	if len(dispenses) > 0 {
		first := dispenses[0].Timestamp.UTC().Format("2006-01-02")
		last := dispenses[len(dispenses)-1].Timestamp.UTC().Format("2006-01-02")
		pdf.SetFont("Helvetica", "", 10)
		pdf.Cell(0, 8, fmt.Sprintf("Period: %s to %s", first, last))
		pdf.Ln(10)
	}

	// Table header
	pdf.SetFont("Helvetica", "B", 9)
	colWidths := []float64{45, 55, 15, 50, 40, 30, 25}
	headers := []string{"Timestamp", "Machine ID", "Slot", "Product Name", "Product UPC", "Source", "Cost"}
	for i, h := range headers {
		pdf.CellFormat(colWidths[i], 8, h, "1", 0, "C", false, 0, "")
	}
	pdf.Ln(-1)

	// Table rows
	pdf.SetFont("Helvetica", "", 8)
	var totalCost float64
	for _, d := range dispenses {
		p := products[d.ProductID]
		totalCost += p.CostPerCan
		row := []string{
			d.Timestamp.UTC().Format("2006-01-02 15:04:05"),
			truncate(d.MachineID, 20),
			fmt.Sprintf("%d", d.SlotPosition),
			truncate(p.Name, 20),
			p.UPC,
			d.SourceType,
			fmt.Sprintf("$%.2f", p.CostPerCan),
		}
		for i, cell := range row {
			pdf.CellFormat(colWidths[i], 7, cell, "1", 0, "", false, 0, "")
		}
		pdf.Ln(-1)
	}

	// Summary footer
	pdf.Ln(5)
	pdf.SetFont("Helvetica", "B", 10)
	pdf.Cell(0, 8, fmt.Sprintf("Total Dispenses: %d    Total Cost: $%.2f", len(dispenses), totalCost))

	return pdf.Output(w)
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "~"
}
