package httpapi

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"saaslb-backend/internal/config"
	"saaslb-backend/internal/dodo"
	"saaslb-backend/internal/store"
)

type Server struct {
	cfg  config.Config
	db   *store.Store
	dodo *dodo.Client
}

func New(cfg config.Config, db *store.Store, payments *dodo.Client) http.Handler {
	s := &Server{cfg: cfg, db: db, dodo: payments}

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{cfg.FrontendURL, "http://localhost:5173", "http://127.0.0.1:5173"},
		AllowedMethods:   []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Content-Type", "Authorization"},
		AllowCredentials: false,
		MaxAge:           300,
	}))

	r.Get("/api/health", s.health)
	r.Get("/api/categories", s.categories)
	r.Get("/api/stats", s.stats)
	r.Post("/api/presence", s.presence)
	r.Get("/api/products", s.listProducts)
	r.Get("/api/activity", s.listActivity)
	r.Get("/api/products/{id}", s.getProduct)
	r.Post("/api/products/{id}/clicks", s.recordClick)
	r.Post("/api/products/{id}/meta", s.refreshProductMeta)
	r.Post("/api/checkouts", s.createCheckout)
	r.Get("/api/checkouts/{id}", s.getCheckout)
	r.Post("/api/webhooks/dodo", s.dodoWebhook)

	return r
}
