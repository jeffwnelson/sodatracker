package dynamo

import (
	"fmt"
	"time"
)

// PK/SK patterns for single-table design.

func productPK(id string) string   { return "PRODUCT#" + id }
func productSK(id string) string   { return "PRODUCT#" + id }
func productGSI1PK(upc string) string { return "UPC#" + upc }
func productGSI1SK(id string) string  { return "PRODUCT#" + id }

func machinePK(id string) string    { return "MACHINE#" + id }
func machineSK(id string) string    { return "MACHINE#" + id }
func machineGSI1PK() string        { return "MACHINES" }
func machineGSI1SK(name, id string) string { return fmt.Sprintf("MACHINE#%s#%s", name, id) }

func slotPK(machineID string) string         { return "MACHINE#" + machineID }
func slotSK(position int) string             { return fmt.Sprintf("SLOT#%04d", position) }

func dispensePK(machineID string) string     { return "MACHINE#" + machineID }
func dispenseSK(ts time.Time, id string) string {
	return fmt.Sprintf("DISPENSE#%s#%s", ts.UTC().Format(time.RFC3339Nano), id)
}
func dispenseGSI1PK(machineID string, slotPos int) string {
	return fmt.Sprintf("MACHINESLOT#%s#%04d", machineID, slotPos)
}
func dispenseGSI1SK(ts time.Time, id string) string {
	return fmt.Sprintf("DISPENSE#%s#%s", ts.UTC().Format(time.RFC3339Nano), id)
}

func restockPK(machineID string) string      { return "MACHINE#" + machineID }
func restockSK(ts time.Time, id string) string {
	return fmt.Sprintf("RESTOCK#%s#%s", ts.UTC().Format(time.RFC3339Nano), id)
}

func userPK(id string) string                { return "USER#" + id }
func userSK(id string) string                { return "USER#" + id }
func userGSI1PK(username string) string      { return "USERNAME#" + username }
func userGSI1SK(id string) string            { return "USER#" + id }

func alertConfigPK(machineID string) string  { return "MACHINE#" + machineID }
func alertConfigSK(position int) string      { return fmt.Sprintf("ALERTCFG#%04d", position) }

func alertEventPK(machineID string) string   { return "MACHINE#" + machineID }
func alertEventSK(ts time.Time, id string) string {
	return fmt.Sprintf("ALERTEVT#%s#%s", ts.UTC().Format(time.RFC3339Nano), id)
}
func activeAlertGSI1PK(machineID string, slotPos int) string {
	return fmt.Sprintf("ACTIVEALERT#%s#%04d", machineID, slotPos)
}
func clearedAlertGSI1PK(machineID string, slotPos int) string {
	return fmt.Sprintf("CLEARED#%s#%04d", machineID, slotPos)
}
func alertEventGSI1SK(alertType, id string) string {
	return fmt.Sprintf("%s#%s", alertType, id)
}
