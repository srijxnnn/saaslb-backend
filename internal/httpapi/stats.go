package httpapi

import (
	"encoding/json"
	"net/http"
	"time"

	"saaslb-backend/internal/domain"
	"saaslb-backend/internal/store"
)

type presenceRequest struct {
	VisitorID string `json:"visitorId"`
	Visit     bool   `json:"visit"`
}

func (s *Server) stats(w http.ResponseWriter, r *http.Request) {
	stats, err := s.db.SiteStats(r.Context())
	if err != nil {
		writeMessage(w, http.StatusInternalServerError, "Could not load visits.")
		return
	}
	writeJSON(w, http.StatusOK, statsJSON(stats))
}

func (s *Server) presence(w http.ResponseWriter, r *http.Request) {
	var req presenceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeMessage(w, http.StatusBadRequest, "Could not read that ping.")
		return
	}

	visitorID, err := domain.NormalizeVisitorID(req.VisitorID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	stats, err := s.db.TouchPresence(r.Context(), visitorID, req.Visit)
	if err != nil {
		writeMessage(w, http.StatusInternalServerError, "Could not record that visit.")
		return
	}

	writeJSON(w, http.StatusOK, statsJSON(stats))
}

func statsJSON(stats store.SiteStats) map[string]any {
	payload := map[string]any{
		"online": stats.Online,
		"visits": stats.Visits,
	}
	if !stats.Since.IsZero() {
		payload["since"] = stats.Since.UTC().Format(time.RFC3339)
	}
	return payload
}
