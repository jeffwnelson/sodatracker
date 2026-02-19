package api

import "github.com/jeffwnelson/sodatracker/internal/domain"

// ErrorResponse represents an error response.
type ErrorResponse struct {
	Error string `json:"error" example:"something went wrong"`
}

// HealthResponse represents the health check response.
type HealthResponse struct {
	Status string `json:"status" example:"ok"`
}

// LoginRequest represents the login request body.
type LoginRequest struct {
	Username string `json:"username" example:"admin"`
	Password string `json:"password" example:"dev-admin-password"`
}

// LoginResponse represents the login response body.
type LoginResponse struct {
	Token string       `json:"token" example:"eyJhbGciOiJIUzI1NiIs..."`
	User  *domain.User `json:"user"`
}

// DeleteResponse represents a successful delete response.
type DeleteResponse struct {
	Deleted string `json:"deleted" example:"550e8400-e29b-41d4-a716-446655440000"`
}

// CostResponse represents the machine cost response.
type CostResponse struct {
	Cost float64 `json:"cost" example:"42.50"`
}
