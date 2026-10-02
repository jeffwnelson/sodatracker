package domain

import "time"

type Product struct {
	ID          string  `json:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
	UPC         string  `json:"upc,omitempty" example:"049000042566"`
	Name        string  `json:"name" example:"Coca-Cola"`
	Description string  `json:"description,omitempty" example:"Classic cola"`
	CostPerCan  float64 `json:"costPerCan,omitempty" example:"0.35"`
	ImageURL    string  `json:"imageUrl,omitempty" example:"https://example.com/coke.png"`
	// Color is a "#rrggbb" hex used for the product's dot and chart series.
	// Empty means the dashboard picks one from its default palette.
	Color string `json:"color,omitempty" example:"#e4002b"`
	// SizeOz is the can's volume in US fluid ounces; createProduct defaults
	// it to 12. CaffeineMg is per can; 0 for caffeine-free or unknown.
	SizeOz     float64 `json:"sizeOz" example:"12"`
	CaffeineMg float64 `json:"caffeineMg" example:"34"`
}

type Machine struct {
	ID        string `json:"id" example:"550e8400-e29b-41d4-a716-446655440001"`
	Name      string `json:"name" example:"Break Room"`
	Location  string `json:"location" example:"Building A, Floor 2"`
	SlotCount int    `json:"slotCount" example:"8"`
}

type Slot struct {
	MachineID    string `json:"machineId" example:"550e8400-e29b-41d4-a716-446655440001"`
	Position     int    `json:"position" example:"1"`
	ProductID    string `json:"productId,omitempty" example:"550e8400-e29b-41d4-a716-446655440000"`
	GPIOPin      int    `json:"gpioPin,omitempty" example:"17"`
	Capacity     int    `json:"capacity" example:"12"`
	LowThreshold int    `json:"lowThreshold" example:"3"`
}

type Dispense struct {
	ID           string    `json:"id" example:"550e8400-e29b-41d4-a716-446655440002"`
	MachineID    string    `json:"machineId" example:"550e8400-e29b-41d4-a716-446655440001"`
	ProductID    string    `json:"productId" example:"550e8400-e29b-41d4-a716-446655440000"`
	SlotPosition int       `json:"slotPosition" example:"1"`
	SourceType   string    `json:"sourceType" example:"gpio"`
	Timestamp    time.Time `json:"timestamp" example:"2026-01-15T10:30:00Z"`
}

type Restock struct {
	ID        string        `json:"id" example:"550e8400-e29b-41d4-a716-446655440003"`
	MachineID string        `json:"machineId" example:"550e8400-e29b-41d4-a716-446655440001"`
	UserID    string        `json:"userId,omitempty" example:"550e8400-e29b-41d4-a716-446655440004"`
	Slots     []RestockSlot `json:"slots"`
	Timestamp time.Time     `json:"timestamp" example:"2026-01-15T12:00:00Z"`
}

type RestockSlot struct {
	Position int `json:"position" example:"1"`
	Quantity int `json:"quantity" example:"12"`
}

// SlotStatus represents the computed stock status of a slot.
type SlotStatus struct {
	Slot         Slot     `json:"slot"`
	Product      *Product `json:"product,omitempty"`
	CurrentStock int      `json:"currentStock" example:"8"`
	Status       string   `json:"status" example:"ok"` // "ok", "low", "empty"
}

// MachineStatus represents the full status of a machine including all slot statuses.
type MachineStatus struct {
	Machine Machine      `json:"machine"`
	Slots   []SlotStatus `json:"slots"`
	Agent   AgentStatus  `json:"agent"`
}

// AgentSession is one unbroken run of Pi agent heartbeats: no two
// consecutive beats more than the offline threshold apart.
type AgentSession struct {
	ID        int64     `json:"id"`
	MachineID string    `json:"machineId"`
	StartedAt time.Time `json:"startedAt"`
	LastSeen  time.Time `json:"lastSeen"`
}

// AgentStatus is what the server knows about the machine's Pi agent from its
// heartbeats. LastSeen is nil when the agent has never sent one.
type AgentStatus struct {
	Online   bool       `json:"online"`
	LastSeen *time.Time `json:"lastSeen"`
}

type User struct {
	ID           string `json:"id" example:"550e8400-e29b-41d4-a716-446655440004"`
	Username     string `json:"username" example:"admin"`
	PasswordHash string `json:"-"`
	Role         string `json:"role" example:"admin"`
}

type AlertConfig struct {
	ID           string `json:"id" example:"550e8400-e29b-41d4-a716-446655440005"`
	MachineID    string `json:"machineId" example:"550e8400-e29b-41d4-a716-446655440001"`
	SlotPosition int    `json:"slotPosition" example:"0"` // 0 = machine-wide default
	LowThreshold int    `json:"lowThreshold" example:"3"` // -1 = use slot's own threshold
	Enabled      bool   `json:"enabled" example:"true"`
}

type AlertEvent struct {
	ID           string     `json:"id" example:"550e8400-e29b-41d4-a716-446655440006"`
	MachineID    string     `json:"machineId" example:"550e8400-e29b-41d4-a716-446655440001"`
	SlotPosition int        `json:"slotPosition" example:"1"`
	ProductID    string     `json:"productId" example:"550e8400-e29b-41d4-a716-446655440000"`
	AlertType    string     `json:"alertType" example:"low_stock"` // "low_stock", "out_of_stock", "quiet_slot" or "quiet_machine" (slotPosition 0)
	CurrentStock int        `json:"currentStock" example:"2"`
	Threshold    int        `json:"threshold" example:"3"`
	Cleared      bool       `json:"cleared" example:"false"`
	Timestamp    time.Time  `json:"timestamp" example:"2026-01-15T10:30:00Z"`
	ClearedAt    *time.Time `json:"clearedAt,omitempty"`
}

type StockPrediction struct {
	MachineID         string  `json:"machineId" example:"550e8400-e29b-41d4-a716-446655440001"`
	SlotPosition      int     `json:"slotPosition" example:"1"`
	ProductName       string  `json:"productName,omitempty" example:"Coca-Cola"`
	CurrentStock      int     `json:"currentStock" example:"8"`
	DispensesPerDay   float64 `json:"dispensesPerDay" example:"2.5"`
	EstimatedDaysLeft float64 `json:"estimatedDaysLeft" example:"3.2"` // -1 = no data
	PredictedStockout *string `json:"predictedStockout,omitempty"`
}
