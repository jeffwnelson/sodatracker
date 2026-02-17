package seed

import (
	"context"
	"log"

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
		log.Printf("warning: failed to check admin user: %v", err)
		return
	}
	if existing != nil {
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("warning: failed to hash admin password: %v", err)
		return
	}

	u := &domain.User{
		ID:           uuid.NewString(),
		Username:     "admin",
		PasswordHash: string(hash),
		Role:         "admin",
	}
	if err := st.CreateUser(ctx, u); err != nil {
		log.Printf("warning: failed to seed admin user: %v", err)
		return
	}
	log.Println("seeded admin user")
}
