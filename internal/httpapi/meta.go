package httpapi

import (
	"context"
	"net/http"
	"time"

	"saaslb-backend/internal/domain"
)

func (s *Server) withPeriod(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := s.db.EnsurePeriod(r.Context(), domain.CurrentPeriodKey(time.Now())); err != nil {
			writeMessage(w, http.StatusInternalServerError, "Could not roll the month.")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":              true,
		"paymentsMode":    s.cfg.PaymentsMode,
		"dodoConfigured":  s.dodo != nil,
		"dodoEnvironment": s.cfg.DodoEnvironment,
	})
}

func (s *Server) period(w http.ResponseWriter, _ *http.Request) {
	now := time.Now()
	writeJSON(w, http.StatusOK, map[string]any{
		"period":   domain.CurrentPeriodKey(now),
		"resetsAt": domain.NextMonthStart(now).UTC(),
	})
}

func (s *Server) categories(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"categories": domain.Categories})
}

func (s *Server) currentPeriod() string {
	return domain.CurrentPeriodKey(time.Now())
}

func (s *Server) products(ctx context.Context) ([]domain.Product, error) {
	return s.db.ListProducts(ctx)
}
