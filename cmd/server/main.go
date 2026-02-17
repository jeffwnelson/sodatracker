package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/google/uuid"
	"github.com/jeffwnelson/sodatracker/internal/api"
	"github.com/jeffwnelson/sodatracker/internal/config"
	"github.com/jeffwnelson/sodatracker/internal/domain"
	"github.com/jeffwnelson/sodatracker/internal/service"
	"github.com/jeffwnelson/sodatracker/internal/store/sqlite"
)

func main() {
	cfg := config.Load()

	st, err := sqlite.New(cfg.DBPath)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer st.Close()

	// Seed admin user if not exists
	seedAdminUser(st, cfg.AdminPassword)

	svc := service.New(st, cfg.JWTSecret)
	router := api.NewRouter(st, svc, cfg.APIKey, cfg.JWTSecret, cfg.StaticDir)

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Printf("SodaTracker API listening on :%d", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	<-done
	log.Println("shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("shutdown error: %v", err)
	}
	log.Println("server stopped")
}

func seedAdminUser(st *sqlite.SQLiteStore, password string) {
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
