package httpapi

import (
	"context"
	"net/http"

	"saaslb-backend/internal/domain"
)

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":              true,
		"paymentsMode":    s.cfg.PaymentsMode,
		"dodoConfigured":  s.dodo != nil,
		"dodoEnvironment": s.cfg.DodoEnvironment,
	})
}

func (s *Server) categories(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"categories": domain.Categories})
}

func (s *Server) products(ctx context.Context) ([]domain.Product, error) {
	return s.db.ListProducts(ctx)
}
