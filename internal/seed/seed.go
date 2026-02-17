package seed

import (
	"context"
	"log/slog"

	"golang.org/x/crypto/bcrypt"

	"github.com/google/uuid"
	"github.com/jeffwnelson/sodatracker/internal/domain"
	"github.com/jeffwnelson/sodatracker/internal/store"
)

// AdminUser creates the admin user if it doesn't already exist.
func AdminUser(st store.Store, password string) {
	ctx := context.Background()
	existing, err := st.GetUserByUsername(ctx, "admin")
	if err != nil {
		slog.Warn("failed to check admin user", "error", err)
		return
	}
	if existing != nil {
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		slog.Warn("failed to hash admin password", "error", err)
		return
	}

	u := &domain.User{
		ID:           uuid.NewString(),
		Username:     "admin",
		PasswordHash: string(hash),
		Role:         "admin",
	}
	if err := st.CreateUser(ctx, u); err != nil {
		slog.Warn("failed to seed admin user", "error", err)
		return
	}
	slog.Info("seeded admin user")
}
