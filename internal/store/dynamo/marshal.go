package dynamo

import (
	"encoding/json"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/jeffwnelson/sodatracker/internal/domain"
)

// Helper to create a DynamoDB string attribute.
func s(val string) types.AttributeValue {
	return &types.AttributeValueMemberS{Value: val}
}

// Helper to create a DynamoDB number attribute.
func n(val int) types.AttributeValue {
	return &types.AttributeValueMemberN{Value: strconv.Itoa(val)}
}

// Helper to create a DynamoDB number attribute from float64.
func nf(val float64) types.AttributeValue {
	return &types.AttributeValueMemberN{Value: strconv.FormatFloat(val, 'f', -1, 64)}
}

// Helper to create a DynamoDB BOOL attribute.
func b(val bool) types.AttributeValue {
	return &types.AttributeValueMemberBOOL{Value: val}
}

// Helper to read a string from an item.
func getS(item map[string]types.AttributeValue, key string) string {
	if v, ok := item[key].(*types.AttributeValueMemberS); ok {
		return v.Value
	}
	return ""
}

// Helper to read an int from an item.
func getN(item map[string]types.AttributeValue, key string) int {
	if v, ok := item[key].(*types.AttributeValueMemberN); ok {
		i, _ := strconv.Atoi(v.Value)
		return i
	}
	return 0
}

// Helper to read a float64 from an item.
func getNF(item map[string]types.AttributeValue, key string) float64 {
	if v, ok := item[key].(*types.AttributeValueMemberN); ok {
		f, _ := strconv.ParseFloat(v.Value, 64)
		return f
	}
	return 0
}

// Helper to read a bool from an item.
func getB(item map[string]types.AttributeValue, key string) bool {
	if v, ok := item[key].(*types.AttributeValueMemberBOOL); ok {
		return v.Value
	}
	return false
}

// Helper to read a time from an item (stored as RFC3339Nano string).
func getTime(item map[string]types.AttributeValue, key string) time.Time {
	str := getS(item, key)
	if str == "" {
		return time.Time{}
	}
	t, _ := time.Parse(time.RFC3339Nano, str)
	return t
}

// Helper to read an optional time pointer.
func getTimePtr(item map[string]types.AttributeValue, key string) *time.Time {
	str := getS(item, key)
	if str == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339Nano, str)
	if err != nil {
		return nil
	}
	return &t
}

// --- Marshal domain types to DynamoDB items ---

func marshalProduct(p *domain.Product) map[string]types.AttributeValue {
	return map[string]types.AttributeValue{
		"PK":         s(productPK(p.ID)),
		"SK":         s(productSK(p.ID)),
		"GSI1PK":     s(productGSI1PK(p.UPC)),
		"GSI1SK":     s(productGSI1SK(p.ID)),
		"_type":      s("Product"),
		"id":         s(p.ID),
		"upc":        s(p.UPC),
		"name":       s(p.Name),
		"costPerCan": nf(p.CostPerCan),
		"imageUrl":   s(p.ImageURL),
	}
}

func unmarshalProduct(item map[string]types.AttributeValue) domain.Product {
	return domain.Product{
		ID:         getS(item, "id"),
		UPC:        getS(item, "upc"),
		Name:       getS(item, "name"),
		CostPerCan: getNF(item, "costPerCan"),
		ImageURL:   getS(item, "imageUrl"),
	}
}

func marshalMachine(m *domain.Machine) map[string]types.AttributeValue {
	return map[string]types.AttributeValue{
		"PK":        s(machinePK(m.ID)),
		"SK":        s(machineSK(m.ID)),
		"GSI1PK":    s(machineGSI1PK()),
		"GSI1SK":    s(machineGSI1SK(m.Name, m.ID)),
		"_type":     s("Machine"),
		"id":        s(m.ID),
		"name":      s(m.Name),
		"location":  s(m.Location),
		"slotCount": n(m.SlotCount),
	}
}

func unmarshalMachine(item map[string]types.AttributeValue) domain.Machine {
	return domain.Machine{
		ID:        getS(item, "id"),
		Name:      getS(item, "name"),
		Location:  getS(item, "location"),
		SlotCount: getN(item, "slotCount"),
	}
}

func marshalSlot(machineID string, sl *domain.Slot) map[string]types.AttributeValue {
	return map[string]types.AttributeValue{
		"PK":           s(slotPK(machineID)),
		"SK":           s(slotSK(sl.Position)),
		"_type":        s("Slot"),
		"machineId":    s(machineID),
		"position":     n(sl.Position),
		"productId":    s(sl.ProductID),
		"gpioPin":      n(sl.GPIOPin),
		"capacity":     n(sl.Capacity),
		"lowThreshold": n(sl.LowThreshold),
	}
}

func unmarshalSlot(item map[string]types.AttributeValue) domain.Slot {
	return domain.Slot{
		MachineID:    getS(item, "machineId"),
		Position:     getN(item, "position"),
		ProductID:    getS(item, "productId"),
		GPIOPin:      getN(item, "gpioPin"),
		Capacity:     getN(item, "capacity"),
		LowThreshold: getN(item, "lowThreshold"),
	}
}

func marshalDispense(d *domain.Dispense) map[string]types.AttributeValue {
	return map[string]types.AttributeValue{
		"PK":           s(dispensePK(d.MachineID)),
		"SK":           s(dispenseSK(d.Timestamp, d.ID)),
		"GSI1PK":       s(dispenseGSI1PK(d.MachineID, d.SlotPosition)),
		"GSI1SK":       s(dispenseGSI1SK(d.Timestamp, d.ID)),
		"_type":        s("Dispense"),
		"id":           s(d.ID),
		"machineId":    s(d.MachineID),
		"productId":    s(d.ProductID),
		"slotPosition": n(d.SlotPosition),
		"sourceType":   s(d.SourceType),
		"timestamp":    s(d.Timestamp.UTC().Format(time.RFC3339Nano)),
	}
}

func unmarshalDispense(item map[string]types.AttributeValue) domain.Dispense {
	return domain.Dispense{
		ID:           getS(item, "id"),
		MachineID:    getS(item, "machineId"),
		ProductID:    getS(item, "productId"),
		SlotPosition: getN(item, "slotPosition"),
		SourceType:   getS(item, "sourceType"),
		Timestamp:    getTime(item, "timestamp"),
	}
}

func marshalRestock(r *domain.Restock) map[string]types.AttributeValue {
	slotsJSON, _ := json.Marshal(r.Slots)
	return map[string]types.AttributeValue{
		"PK":        s(restockPK(r.MachineID)),
		"SK":        s(restockSK(r.Timestamp, r.ID)),
		"_type":     s("Restock"),
		"id":        s(r.ID),
		"machineId": s(r.MachineID),
		"userId":    s(r.UserID),
		"slotsJson": s(string(slotsJSON)),
		"timestamp": s(r.Timestamp.UTC().Format(time.RFC3339Nano)),
	}
}

func unmarshalRestock(item map[string]types.AttributeValue) (domain.Restock, error) {
	r := domain.Restock{
		ID:        getS(item, "id"),
		MachineID: getS(item, "machineId"),
		UserID:    getS(item, "userId"),
		Timestamp: getTime(item, "timestamp"),
	}
	slotsJSON := getS(item, "slotsJson")
	if slotsJSON != "" {
		if err := json.Unmarshal([]byte(slotsJSON), &r.Slots); err != nil {
			return r, err
		}
	}
	return r, nil
}

func marshalUser(u *domain.User) map[string]types.AttributeValue {
	return map[string]types.AttributeValue{
		"PK":           s(userPK(u.ID)),
		"SK":           s(userSK(u.ID)),
		"GSI1PK":       s(userGSI1PK(u.Username)),
		"GSI1SK":       s(userGSI1SK(u.ID)),
		"_type":        s("User"),
		"id":           s(u.ID),
		"username":     s(u.Username),
		"passwordHash": s(u.PasswordHash),
		"role":         s(u.Role),
	}
}

func unmarshalUser(item map[string]types.AttributeValue) domain.User {
	return domain.User{
		ID:           getS(item, "id"),
		Username:     getS(item, "username"),
		PasswordHash: getS(item, "passwordHash"),
		Role:         getS(item, "role"),
	}
}

func marshalAlertConfig(ac *domain.AlertConfig) map[string]types.AttributeValue {
	return map[string]types.AttributeValue{
		"PK":           s(alertConfigPK(ac.MachineID)),
		"SK":           s(alertConfigSK(ac.SlotPosition)),
		"_type":        s("AlertConfig"),
		"id":           s(ac.ID),
		"machineId":    s(ac.MachineID),
		"slotPosition": n(ac.SlotPosition),
		"lowThreshold": n(ac.LowThreshold),
		"enabled":      b(ac.Enabled),
	}
}

func unmarshalAlertConfig(item map[string]types.AttributeValue) domain.AlertConfig {
	return domain.AlertConfig{
		ID:           getS(item, "id"),
		MachineID:    getS(item, "machineId"),
		SlotPosition: getN(item, "slotPosition"),
		LowThreshold: getN(item, "lowThreshold"),
		Enabled:      getB(item, "enabled"),
	}
}

func marshalAlertEvent(ae *domain.AlertEvent) map[string]types.AttributeValue {
	gsi1pk := activeAlertGSI1PK(ae.MachineID, ae.SlotPosition)
	if ae.Cleared {
		gsi1pk = clearedAlertGSI1PK(ae.MachineID, ae.SlotPosition)
	}

	item := map[string]types.AttributeValue{
		"PK":           s(alertEventPK(ae.MachineID)),
		"SK":           s(alertEventSK(ae.Timestamp, ae.ID)),
		"GSI1PK":       s(gsi1pk),
		"GSI1SK":       s(alertEventGSI1SK(ae.AlertType, ae.ID)),
		"_type":        s("AlertEvent"),
		"id":           s(ae.ID),
		"machineId":    s(ae.MachineID),
		"slotPosition": n(ae.SlotPosition),
		"productId":    s(ae.ProductID),
		"alertType":    s(ae.AlertType),
		"currentStock": n(ae.CurrentStock),
		"threshold":    n(ae.Threshold),
		"cleared":      b(ae.Cleared),
		"timestamp":    s(ae.Timestamp.UTC().Format(time.RFC3339Nano)),
	}
	if ae.ClearedAt != nil {
		item["clearedAt"] = s(ae.ClearedAt.UTC().Format(time.RFC3339Nano))
	}
	return item
}

func unmarshalAlertEvent(item map[string]types.AttributeValue) domain.AlertEvent {
	return domain.AlertEvent{
		ID:           getS(item, "id"),
		MachineID:    getS(item, "machineId"),
		SlotPosition: getN(item, "slotPosition"),
		ProductID:    getS(item, "productId"),
		AlertType:    getS(item, "alertType"),
		CurrentStock: getN(item, "currentStock"),
		Threshold:    getN(item, "threshold"),
		Cleared:      getB(item, "cleared"),
		Timestamp:    getTime(item, "timestamp"),
		ClearedAt:    getTimePtr(item, "clearedAt"),
	}
}
