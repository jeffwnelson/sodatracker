package service

import (
	"context"
	"fmt"
	"sort"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/jeffwnelson/sodatracker/internal/auth"
	"github.com/jeffwnelson/sodatracker/internal/domain"
	"github.com/jeffwnelson/sodatracker/internal/store"
)

// Service contains the core business logic for SodaTracker.
type Service struct {
	store     store.Store
	jwtSecret string
}

func New(s store.Store, jwtSecret string) *Service {
	return &Service{store: s, jwtSecret: jwtSecret}
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

// RegisterDispense records a new dispense event.
func (s *Service) RegisterDispense(ctx context.Context, d *domain.Dispense) error {
	return s.store.CreateDispense(ctx, d)
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

// DispenseStats holds aggregated dispense analytics.
type DispenseStats struct {
	TotalCount  int               `json:"totalCount"`
	TotalCost   float64           `json:"totalCost"`
	TimeSeries  []TimeSeriesPoint `json:"timeSeries"`
	TopProducts []ProductRanking  `json:"topProducts"`
}

type TimeSeriesPoint struct {
	Hour  string `json:"hour"` // RFC3339 truncated to hour
	Count int    `json:"count"`
}

type ProductRanking struct {
	ProductID   string  `json:"productId"`
	ProductName string  `json:"productName"`
	Count       int     `json:"count"`
	TotalCost   float64 `json:"totalCost"`
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

	stats := &DispenseStats{
		TimeSeries:  []TimeSeriesPoint{},
		TopProducts: []ProductRanking{},
	}

	hourCounts := make(map[string]int)
	productCounts := make(map[string]*ProductRanking)

	for _, d := range dispenses {
		stats.TotalCount++

		p := productMap[d.ProductID]
		stats.TotalCost += p.CostPerCan

		// Hourly bucketing
		hourKey := d.Timestamp.UTC().Truncate(time.Hour).Format(time.RFC3339)
		hourCounts[hourKey]++

		// Product ranking
		pr, ok := productCounts[d.ProductID]
		if !ok {
			pr = &ProductRanking{
				ProductID:   d.ProductID,
				ProductName: p.Name,
			}
			productCounts[d.ProductID] = pr
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
	productMap := make(map[string]float64)
	for _, p := range products {
		productMap[p.ID] = p.CostPerCan
	}

	var total float64
	for _, d := range dispenses {
		total += productMap[d.ProductID]
	}
	return total, nil
}
