package service

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/google/uuid"
	"github.com/jeffwnelson/sodatracker/internal/auth"
	"github.com/jeffwnelson/sodatracker/internal/domain"
	"github.com/jeffwnelson/sodatracker/internal/events"
	"github.com/jeffwnelson/sodatracker/internal/store"
)

// Service contains the core business logic for SodaTracker.
type Service struct {
	store     store.Store
	jwtSecret string
	bus       events.EventBus
}

func New(s store.Store, jwtSecret string, bus events.EventBus) *Service {
	return &Service{store: s, jwtSecret: jwtSecret, bus: bus}
}

// GetMachineStatus computes the current stock status for every slot in a machine.
// Stock = restock quantity - dispenses since last restock.
func (s *Service) GetMachineStatus(ctx context.Context, machineID string) (*domain.MachineStatus, error) {
	machine, err := s.store.GetMachine(ctx, machineID)
	if err != nil {
		return nil, err
	}
	if machine == nil {
		return nil, nil
	}

	slots, err := s.store.GetSlots(ctx, machineID)
	if err != nil {
		return nil, err
	}

	statuses := make([]domain.SlotStatus, len(slots))
	for i, slot := range slots {
		ss := domain.SlotStatus{Slot: slot}

		if slot.ProductID != "" {
			p, err := s.store.GetProduct(ctx, slot.ProductID)
			if err != nil {
				return nil, err
			}
			ss.Product = p
		}

		// Compute stock: restock quantity - dispenses since restock
		restock, restockSlot, err := s.store.GetLastRestock(ctx, machineID, slot.Position)
		if err != nil {
			return nil, err
		}

		if restock != nil && restockSlot != nil {
			dispensed, err := s.store.CountDispensesSince(ctx, machineID, slot.Position, restock.Timestamp)
			if err != nil {
				return nil, err
			}
			ss.CurrentStock = restockSlot.Quantity - dispensed
			if ss.CurrentStock < 0 {
				ss.CurrentStock = 0
			}
		}

		switch {
		case ss.CurrentStock <= 0:
			ss.Status = "empty"
		case ss.CurrentStock <= slot.LowThreshold:
			ss.Status = "low"
		default:
			ss.Status = "ok"
		}

		statuses[i] = ss
	}

	return &domain.MachineStatus{
		Machine: *machine,
		Slots:   statuses,
	}, nil
}

// RegisterDispense records a new dispense event and publishes it on the event bus.
func (s *Service) RegisterDispense(ctx context.Context, d *domain.Dispense) error {
	if err := s.store.CreateDispense(ctx, d); err != nil {
		return err
	}
	if s.bus != nil {
		s.bus.Publish(ctx, events.Event{
			Type:      events.EventDispense,
			Timestamp: d.Timestamp.UTC().Format(time.RFC3339),
			Data: map[string]any{
				"machineId":    d.MachineID,
				"slotPosition": d.SlotPosition,
				"productId":    d.ProductID,
				"dispenseId":   d.ID,
			},
		})
	}
	return nil
}

// ListDispenses returns recent dispenses for a machine.
func (s *Service) ListDispenses(ctx context.Context, machineID string, since time.Time, limit int) ([]domain.Dispense, error) {
	return s.store.ListDispenses(ctx, machineID, since, limit)
}

// Login authenticates a user and returns a JWT token.
func (s *Service) Login(ctx context.Context, username, password string) (string, *domain.User, error) {
	user, err := s.store.GetUserByUsername(ctx, username)
	if err != nil {
		return "", nil, err
	}
	if user == nil {
		return "", nil, fmt.Errorf("invalid credentials")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return "", nil, fmt.Errorf("invalid credentials")
	}
	token, err := auth.GenerateToken(s.jwtSecret, user.ID, user.Username, user.Role)
	if err != nil {
		return "", nil, err
	}
	return token, user, nil
}

// ListRestocks returns recent restocks for a machine.
func (s *Service) ListRestocks(ctx context.Context, machineID string, limit int) ([]domain.Restock, error) {
	return s.store.ListRestocks(ctx, machineID, limit)
}

// CreateRestock records a restock and clears alerts for restocked slots.
func (s *Service) CreateRestock(ctx context.Context, r *domain.Restock) error {
	if err := s.store.CreateRestock(ctx, r); err != nil {
		return err
	}
	for _, slot := range r.Slots {
		s.store.ClearAlertEvents(ctx, r.MachineID, slot.Position)
	}
	if s.bus != nil {
		for _, slot := range r.Slots {
			s.bus.Publish(ctx, events.Event{
				Type:      events.EventStockCleared,
				Timestamp: r.Timestamp.UTC().Format(time.RFC3339),
				Data: map[string]any{
					"machineId":    r.MachineID,
					"slotPosition": slot.Position,
					"quantity":     slot.Quantity,
				},
			})
		}
	}
	return nil
}

// EvaluateStockAlert checks stock for a slot and fires alerts if needed.
func (s *Service) EvaluateStockAlert(ctx context.Context, machineID string, slotPosition int) error {
	// Get slot config
	slots, err := s.store.GetSlots(ctx, machineID)
	if err != nil {
		return err
	}
	var slot *domain.Slot
	for i := range slots {
		if slots[i].Position == slotPosition {
			slot = &slots[i]
			break
		}
	}
	if slot == nil {
		return nil
	}

	// Compute current stock
	restock, restockSlot, err := s.store.GetLastRestock(ctx, machineID, slotPosition)
	if err != nil {
		return err
	}
	currentStock := 0
	if restock != nil && restockSlot != nil {
		dispensed, err := s.store.CountDispensesSince(ctx, machineID, slotPosition, restock.Timestamp)
		if err != nil {
			return err
		}
		currentStock = restockSlot.Quantity - dispensed
		if currentStock < 0 {
			currentStock = 0
		}
	}

	// Determine threshold: check alert configs (slot-specific > machine-wide)
	threshold := slot.LowThreshold
	configs, err := s.store.ListAlertConfigs(ctx, machineID)
	if err != nil {
		return err
	}
	var slotConfig, machineConfig *domain.AlertConfig
	for i := range configs {
		if !configs[i].Enabled {
			continue
		}
		if configs[i].SlotPosition == slotPosition {
			slotConfig = &configs[i]
		} else if configs[i].SlotPosition == 0 {
			machineConfig = &configs[i]
		}
	}
	if slotConfig != nil && slotConfig.LowThreshold >= 0 {
		threshold = slotConfig.LowThreshold
	} else if machineConfig != nil && machineConfig.LowThreshold >= 0 {
		threshold = machineConfig.LowThreshold
	}

	productID := slot.ProductID

	// Check out_of_stock
	if currentStock <= 0 {
		existing, err := s.store.GetActiveAlert(ctx, machineID, slotPosition, "out_of_stock")
		if err != nil {
			return err
		}
		if existing == nil {
			ae := &domain.AlertEvent{
				ID:           uuid.NewString(),
				MachineID:    machineID,
				SlotPosition: slotPosition,
				ProductID:    productID,
				AlertType:    "out_of_stock",
				CurrentStock: currentStock,
				Threshold:    threshold,
				Timestamp:    time.Now().UTC(),
			}
			if err := s.store.CreateAlertEvent(ctx, ae); err != nil {
				return err
			}
			if s.bus != nil {
				s.bus.Publish(ctx, events.Event{
					Type:      events.EventOutOfStock,
					Timestamp: ae.Timestamp.UTC().Format(time.RFC3339),
					Data: map[string]any{
						"machineId":    machineID,
						"slotPosition": slotPosition,
						"productId":    productID,
						"currentStock": currentStock,
					},
				})
			}
		}
		return nil
	}

	// Check low_stock
	if currentStock <= threshold {
		existing, err := s.store.GetActiveAlert(ctx, machineID, slotPosition, "low_stock")
		if err != nil {
			return err
		}
		if existing == nil {
			ae := &domain.AlertEvent{
				ID:           uuid.NewString(),
				MachineID:    machineID,
				SlotPosition: slotPosition,
				ProductID:    productID,
				AlertType:    "low_stock",
				CurrentStock: currentStock,
				Threshold:    threshold,
				Timestamp:    time.Now().UTC(),
			}
			if err := s.store.CreateAlertEvent(ctx, ae); err != nil {
				return err
			}
			if s.bus != nil {
				s.bus.Publish(ctx, events.Event{
					Type:      events.EventLowStock,
					Timestamp: ae.Timestamp.UTC().Format(time.RFC3339),
					Data: map[string]any{
						"machineId":    machineID,
						"slotPosition": slotPosition,
						"productId":    productID,
						"currentStock": currentStock,
						"threshold":    threshold,
					},
				})
			}
		}
	}

	return nil
}

// GetStockPredictions computes estimated days until stock-out for each slot.
func (s *Service) GetStockPredictions(ctx context.Context, machineID string, lookbackDays int) ([]domain.StockPrediction, error) {
	if lookbackDays <= 0 {
		lookbackDays = 14
	}

	status, err := s.GetMachineStatus(ctx, machineID)
	if err != nil {
		return nil, err
	}
	if status == nil {
		return nil, nil
	}

	since := time.Now().UTC().AddDate(0, 0, -lookbackDays)
	var predictions []domain.StockPrediction

	for _, ss := range status.Slots {
		pred := domain.StockPrediction{
			MachineID:         machineID,
			SlotPosition:      ss.Slot.Position,
			CurrentStock:      ss.CurrentStock,
			EstimatedDaysLeft: -1,
		}
		if ss.Product != nil {
			pred.ProductName = ss.Product.Name
		}

		count, err := s.store.CountDispensesSince(ctx, machineID, ss.Slot.Position, since)
		if err != nil {
			return nil, err
		}

		if count > 0 {
			pred.DispensesPerDay = float64(count) / float64(lookbackDays)
			if pred.DispensesPerDay > 0 && ss.CurrentStock > 0 {
				pred.EstimatedDaysLeft = math.Round(float64(ss.CurrentStock)/pred.DispensesPerDay*10) / 10
				stockoutTime := time.Now().UTC().Add(time.Duration(pred.EstimatedDaysLeft*24) * time.Hour)
				stockoutStr := stockoutTime.Format(time.RFC3339)
				pred.PredictedStockout = &stockoutStr
			} else if ss.CurrentStock <= 0 {
				pred.EstimatedDaysLeft = 0
			}
		}

		predictions = append(predictions, pred)
	}

	return predictions, nil
}

// Delete pass-throughs

func (s *Service) DeleteProduct(ctx context.Context, id string) error {
	return s.store.DeleteProduct(ctx, id)
}

func (s *Service) DeleteMachine(ctx context.Context, id string) error {
	return s.store.DeleteMachine(ctx, id)
}

// Alert config pass-throughs

func (s *Service) CreateAlertConfig(ctx context.Context, ac *domain.AlertConfig) error {
	return s.store.CreateAlertConfig(ctx, ac)
}

func (s *Service) ListAlertConfigs(ctx context.Context, machineID string) ([]domain.AlertConfig, error) {
	return s.store.ListAlertConfigs(ctx, machineID)
}

func (s *Service) UpdateAlertConfig(ctx context.Context, ac *domain.AlertConfig) error {
	return s.store.UpdateAlertConfig(ctx, ac)
}

func (s *Service) DeleteAlertConfig(ctx context.Context, id string) error {
	return s.store.DeleteAlertConfig(ctx, id)
}

func (s *Service) ListAlertEvents(ctx context.Context, machineID string, activeOnly bool, limit int) ([]domain.AlertEvent, error) {
	return s.store.ListAlertEvents(ctx, machineID, activeOnly, limit)
}

// DispenseStats holds aggregated dispense analytics.
type DispenseStats struct {
	TotalCount  int               `json:"totalCount" example:"150"`
	TotalCost   float64           `json:"totalCost" example:"52.50"`
	TimeSeries  []TimeSeriesPoint `json:"timeSeries"`
	TopProducts []ProductRanking  `json:"topProducts"`
}

type TimeSeriesPoint struct {
	Hour  string `json:"hour" example:"2026-01-15T10:00:00Z"` // RFC3339 truncated to hour
	Count int    `json:"count" example:"5"`
}

type ProductRanking struct {
	ProductID   string  `json:"productId" example:"550e8400-e29b-41d4-a716-446655440000"`
	ProductName string  `json:"productName" example:"Coca-Cola"`
	Count       int     `json:"count" example:"42"`
	TotalCost   float64 `json:"totalCost" example:"14.70"`
}

// GetDispenseStats aggregates dispense data for a machine since a given time.
func (s *Service) GetDispenseStats(ctx context.Context, machineID string, since time.Time) (*DispenseStats, error) {
	dispenses, err := s.store.ListDispenses(ctx, machineID, since, 10000)
	if err != nil {
		return nil, err
	}

	// Build product cache for names and costs
	products, err := s.store.ListProducts(ctx)
	if err != nil {
		return nil, err
	}
	productMap := make(map[string]domain.Product)
	for _, p := range products {
		productMap[p.ID] = p
	}

	// Build slot→product fallback for orphaned product IDs (e.g. products recreated with new UUIDs)
	slots, _ := s.store.GetSlots(ctx, machineID)
	slotProductMap := make(map[int]domain.Product)
	for _, sl := range slots {
		if p, ok := productMap[sl.ProductID]; ok {
			slotProductMap[sl.Position] = p
		}
	}

	stats := &DispenseStats{
		TimeSeries:  []TimeSeriesPoint{},
		TopProducts: []ProductRanking{},
	}

	hourCounts := make(map[string]int)
	productCounts := make(map[string]*ProductRanking)

	for _, d := range dispenses {
		stats.TotalCount++

		p := productMap[d.ProductID]
		// Fallback: if product was deleted/recreated, use the slot's current product
		if p.Name == "" {
			if sp, ok := slotProductMap[d.SlotPosition]; ok {
				p = sp
			}
		}
		stats.TotalCost += p.CostPerCan

		// Hourly bucketing
		hourKey := d.Timestamp.UTC().Truncate(time.Hour).Format(time.RFC3339)
		hourCounts[hourKey]++

		// Product ranking — group by resolved product name to merge old+new IDs
		groupKey := d.ProductID
		if p.ID != "" && p.ID != d.ProductID {
			groupKey = p.ID // merge under the current product ID
		}
		pr, ok := productCounts[groupKey]
		if !ok {
			pr = &ProductRanking{
				ProductID:   groupKey,
				ProductName: p.Name,
			}
			productCounts[groupKey] = pr
		}
		pr.Count++
		pr.TotalCost += p.CostPerCan
	}

	// Convert hour map to sorted slice
	for hour, count := range hourCounts {
		stats.TimeSeries = append(stats.TimeSeries, TimeSeriesPoint{Hour: hour, Count: count})
	}
	sort.Slice(stats.TimeSeries, func(i, j int) bool {
		return stats.TimeSeries[i].Hour < stats.TimeSeries[j].Hour
	})

	// Convert product map to sorted slice (descending by count)
	for _, pr := range productCounts {
		stats.TopProducts = append(stats.TopProducts, *pr)
	}
	sort.Slice(stats.TopProducts, func(i, j int) bool {
		return stats.TopProducts[i].Count > stats.TopProducts[j].Count
	})

	return stats, nil
}

// GetMachineCost calculates the total cost of dispenses for a machine since a given time.
func (s *Service) GetMachineCost(ctx context.Context, machineID string, since time.Time) (float64, error) {
	dispenses, err := s.store.ListDispenses(ctx, machineID, since, 10000)
	if err != nil {
		return 0, err
	}

	products, err := s.store.ListProducts(ctx)
	if err != nil {
		return 0, err
	}
	productCostMap := make(map[string]float64)
	productMap := make(map[string]domain.Product)
	for _, p := range products {
		productCostMap[p.ID] = p.CostPerCan
		productMap[p.ID] = p
	}

	// Slot→cost fallback for orphaned product IDs
	slots, _ := s.store.GetSlots(ctx, machineID)
	slotCostMap := make(map[int]float64)
	for _, sl := range slots {
		if p, ok := productMap[sl.ProductID]; ok {
			slotCostMap[sl.Position] = p.CostPerCan
		}
	}

	var total float64
	for _, d := range dispenses {
		if cost, ok := productCostMap[d.ProductID]; ok {
			total += cost
		} else if cost, ok := slotCostMap[d.SlotPosition]; ok {
			total += cost
		}
	}
	return total, nil
}
