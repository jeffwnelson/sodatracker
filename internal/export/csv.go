package export

import (
	"encoding/csv"
	"fmt"
	"io"

	"github.com/jeffwnelson/sodatracker/internal/domain"
)

// WriteDispensesCSV writes dispense data as CSV to w.
func WriteDispensesCSV(w io.Writer, dispenses []domain.Dispense, products map[string]domain.Product) error {
	cw := csv.NewWriter(w)
	defer cw.Flush()

	header := []string{"Timestamp", "Machine ID", "Slot", "Product Name", "Product UPC", "Source", "Cost"}
	if err := cw.Write(header); err != nil {
		return err
	}

	for _, d := range dispenses {
		p := products[d.ProductID]
		row := []string{
			d.Timestamp.UTC().Format("2006-01-02T15:04:05Z"),
			d.MachineID,
			fmt.Sprintf("%d", d.SlotPosition),
			p.Name,
			p.UPC,
			d.SourceType,
			fmt.Sprintf("%.2f", p.CostPerCan),
		}
		if err := cw.Write(row); err != nil {
			return err
		}
	}
	return cw.Error()
}
