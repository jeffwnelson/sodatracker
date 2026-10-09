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
	"mime"
	"net/http"
	"path/filepath"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	httpSwagger "github.com/swaggo/http-swagger/v2"

	"github.com/jeffwnelson/sodatracker/internal/auth"
	"github.com/jeffwnelson/sodatracker/internal/backup"
	"github.com/jeffwnelson/sodatracker/internal/events"
	"github.com/jeffwnelson/sodatracker/internal/service"
	"github.com/jeffwnelson/sodatracker/internal/slackcmd"
	"github.com/jeffwnelson/sodatracker/internal/slackconf"
	"github.com/jeffwnelson/sodatracker/internal/store"

	_ "github.com/jeffwnelson/sodatracker/internal/api/docs"
)

// RouterConfig carries the router's settings. It is a struct rather than
// positional arguments because these are all strings and transposing two of
// them would compile cleanly while breaking auth.
type RouterConfig struct {
	APIKey             string
	JWTSecret          string
	StaticDir          string
	SlackSigningSecret string

	// Bus feeds GET /events. Without one the route isn't registered.
	Bus events.EventBus
	// StreamsDone, when closed, ends every open event stream. The server
	// closes it on shutdown, which would otherwise wait on them.
	StreamsDone <-chan struct{}
	// Backups serves /admin/backup. Without it those routes aren't registered.
	Backups *backup.Manager
	// Slack serves /admin/slack. Without it those routes aren't registered.
	Slack *slackconf.Manager
}

// Go's mime table lacks the web app manifest, which it would serve as
// text/plain.
func init() {
	mime.AddExtensionType(".webmanifest", "application/manifest+json")
}

func NewRouter(st store.Store, svc *service.Service, cfg RouterConfig) *chi.Mux {
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

	h := &handlers{
		store:              st,
		svc:                svc,
		jwtSecret:          cfg.JWTSecret,
		slackSigningSecret: cfg.SlackSigningSecret,
		slackCmds:          slackcmd.New(st, svc),
		streamsDone:        cfg.StreamsDone,
		backups:            cfg.Backups,
		slack:              cfg.Slack,
	}
	if cfg.Bus != nil {
		h.broker = newBroker(cfg.Bus)
	}

	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/health", h.health)

		// Public
		r.Get("/machines", h.listMachines)
		r.Get("/machines/{id}/status", h.getMachineStatus)

		// Auth
		r.Post("/auth/login", h.login)

		// Slack slash commands. Registered only when a signing secret is set,
		// since the handler authenticates by Slack's signature alone — an
		// unsigned route here would be an unauthenticated read of the machine.
		if cfg.SlackSigningSecret != "" {
			r.Post("/integrations/slack/commands", h.slackCommand)
		}

		// Pi agent (API key only)
		r.Group(func(r chi.Router) {
			r.Use(auth.APIKeyMiddleware(cfg.APIKey))
			r.Post("/dispenses", h.createDispense)
			r.Post("/agents/heartbeat", h.agentHeartbeat)
		})

		// Admin (API key or JWT)
		r.Group(func(r chi.Router) {
			r.Use(auth.APIKeyOrJWTMiddleware(cfg.APIKey, cfg.JWTSecret))
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

			if h.broker != nil {
				r.Get("/events", h.streamEvents)
			}
		})

		// JWT-only admin endpoints
		r.Group(func(r chi.Router) {
			r.Use(auth.APIKeyOrJWTMiddleware(cfg.APIKey, cfg.JWTSecret))
			r.Get("/admin/stats", h.getAdminStats)
			r.Get("/admin/cost", h.getMachineCost)
			r.Get("/reports/dispenses", h.exportDispenses)

			// Analytics dashboards
			r.Get("/analytics/consumption", h.getConsumptionAnalytics)
			r.Get("/analytics/products", h.getProductAnalytics)
			r.Get("/analytics/inventory", h.getInventoryAnalytics)
			r.Get("/analytics/spend", h.getSpendAnalytics)
			r.Get("/analytics/highlights", h.getHighlightsAnalytics)
			r.Get("/analytics/uptime", h.getUptimeAnalytics)

			if h.slack != nil {
				r.Get("/admin/slack", h.getSlackSettings)
				r.Put("/admin/slack", h.updateSlackSettings)
				r.Post("/admin/slack/test", h.testSlack)
			}

			if h.backups != nil {
				r.Get("/admin/backup", h.getBackupStatus)
				r.Post("/admin/backup", h.runBackup)
				r.Put("/admin/backup/config", h.configureBackup)
				r.Get("/admin/backup/download", h.downloadBackup)
			}
		})

		// JWT-only user endpoints
		r.Group(func(r chi.Router) {
			r.Use(auth.JWTMiddleware(cfg.JWTSecret))
			r.Delete("/admin/reset", h.resetAllData)
		})
	})

	// Swagger UI
	r.Get("/swagger/*", httpSwagger.WrapHandler)

	// Serve frontend static files if configured
	if cfg.StaticDir != "" {
		root := http.Dir(cfg.StaticDir)
		fileServer := http.FileServer(root)
		index := filepath.Join(cfg.StaticDir, "index.html")
		r.Get("/*", func(w http.ResponseWriter, r *http.Request) {
			// Files are served as they are. Anything else, a client-side
			// route or a directory (which FileServer would list), gets the
			// SPA's index.html.
			if f, err := root.Open(r.URL.Path); err == nil {
				st, err := f.Stat()
				f.Close()
				if err == nil && !st.IsDir() {
					fileServer.ServeHTTP(w, r)
					return
				}
			}
			http.ServeFile(w, r, index)
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
