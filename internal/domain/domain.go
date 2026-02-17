package domain

import "time"

type Product struct {
	ID         string  `json:"id"`
	UPC        string  `json:"upc"`
	Name       string  `json:"name"`
	CostPerCan float64 `json:"costPerCan,omitempty"`
	ImageURL   string  `json:"imageUrl,omitempty"`
}

type Machine struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Location  string `json:"location"`
	SlotCount int    `json:"slotCount"`
}

type Slot struct {
	MachineID    string `json:"machineId"`
	Position     int    `json:"position"`
	ProductID    string `json:"productId,omitempty"`
	GPIOPin      int    `json:"gpioPin,omitempty"`
	Capacity     int    `json:"capacity"`
	LowThreshold int   `json:"lowThreshold"`
}

type Dispense struct {
	ID           string    `json:"id"`
	MachineID    string    `json:"machineId"`
	ProductID    string    `json:"productId"`
	SlotPosition int       `json:"slotPosition"`
	SourceType   string    `json:"sourceType"`
	Timestamp    time.Time `json:"timestamp"`
}

type Restock struct {
	ID        string        `json:"id"`
	MachineID string        `json:"machineId"`
	UserID    string        `json:"userId,omitempty"`
	Slots     []RestockSlot `json:"slots"`
	Timestamp time.Time     `json:"timestamp"`
}

type RestockSlot struct {
	Position int `json:"position"`
	Quantity int `json:"quantity"`
}

// SlotStatus represents the computed stock status of a slot.
type SlotStatus struct {
	Slot         Slot     `json:"slot"`
	Product      *Product `json:"product,omitempty"`
	CurrentStock int      `json:"currentStock"`
	Status       string   `json:"status"` // "ok", "low", "empty"
}

// MachineStatus represents the full status of a machine including all slot statuses.
type MachineStatus struct {
	Machine Machine      `json:"machine"`
	Slots   []SlotStatus `json:"slots"`
}

type User struct {
	ID           string `json:"id"`
	Username     string `json:"username"`
	PasswordHash string `json:"-"`
	Role         string `json:"role"`
}
