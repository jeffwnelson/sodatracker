// @title           SodaTracker API
// @version         1.0
// @description     API for tracking soda machine inventory, dispenses, restocks, and alerts.

// @host            localhost:3000
// @BasePath        /api/v1

// @securityDefinitions.apikey  ApiKeyAuth
// @in                          header
// @name                        X-API-Key

// @securityDefinitions.apikey  BearerAuth
// @in                          header
// @name                        Authorization

package api

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	httpSwagger "github.com/swaggo/http-swagger/v2"

	"github.com/jeffwnelson/sodatracker/internal/auth"
	"github.com/jeffwnelson/sodatracker/internal/service"
	"github.com/jeffwnelson/sodatracker/internal/store"

	_ "github.com/jeffwnelson/sodatracker/internal/api/docs"
)

func NewRouter(st store.Store, svc *service.Service, apiKey string, jwtSecret string, staticDir string) *chi.Mux {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(structuredLogger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RealIP)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-API-Key"},
		AllowCredentials: false,
		MaxAge:           300,
	}))

	h := &handlers{store: st, svc: svc, jwtSecret: jwtSecret}

	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/health", h.health)

		// Public
		r.Get("/machines", h.listMachines)
		r.Get("/machines/{id}/status", h.getMachineStatus)

		// Auth
		r.Post("/auth/login", h.login)

		// Pi agent (API key only)
		r.Group(func(r chi.Router) {
			r.Use(auth.APIKeyMiddleware(apiKey))
			r.Post("/dispenses", h.createDispense)
		})

		// Admin (API key or JWT)
		r.Group(func(r chi.Router) {
			r.Use(auth.APIKeyOrJWTMiddleware(apiKey, jwtSecret))
			r.Post("/machines", h.createMachine)
			r.Post("/machines/{id}/slots", h.setSlots)
			r.Post("/machines/{id}/restock", h.createRestock)
			r.Get("/machines/{id}/dispenses", h.listDispenses)
			r.Get("/machines/{id}/restocks", h.listRestocks)
			r.Get("/products", h.listProducts)
			r.Post("/products", h.createProduct)
			r.Delete("/products/{id}", h.deleteProduct)
			r.Delete("/machines/{id}", h.deleteMachine)

			// Alerts
			r.Get("/machines/{id}/alerts", h.listAlertConfigs)
			r.Post("/machines/{id}/alerts", h.createAlertConfig)
			r.Put("/alerts/{id}", h.updateAlertConfig)
			r.Delete("/alerts/{id}", h.deleteAlertConfig)
			r.Get("/machines/{id}/alert-events", h.listAlertEvents)
			r.Get("/machines/{id}/predictions", h.getStockPredictions)
		})

		// JWT-only admin endpoints
		r.Group(func(r chi.Router) {
			r.Use(auth.APIKeyOrJWTMiddleware(apiKey, jwtSecret))
			r.Get("/admin/stats", h.getAdminStats)
			r.Get("/admin/cost", h.getMachineCost)
			r.Get("/reports/dispenses", h.exportDispenses)
		})
	})

	// Swagger UI
	r.Get("/swagger/*", httpSwagger.WrapHandler)

	// Serve frontend static files if configured
	if staticDir != "" {
		fileServer := http.FileServer(http.Dir(staticDir))
		r.Get("/*", func(w http.ResponseWriter, r *http.Request) {
			// Try the file first; fall back to index.html for SPA routing
			if _, err := http.Dir(staticDir).Open(r.URL.Path); err != nil {
				http.ServeFile(w, r, staticDir+"/index.html")
				return
			}
			fileServer.ServeHTTP(w, r)
		})
	}

	return r
}

func structuredLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		start := time.Now()
		next.ServeHTTP(ww, r)
		slog.Info("http request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", ww.Status(),
			"duration", time.Since(start).String(),
			"requestId", middleware.GetReqID(r.Context()),
		)
	})
}
