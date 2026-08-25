package httpapi

import (
	"net/http"
	"strconv"

	"saaslb-backend/internal/domain"
)

func (s *Server) listActivity(w http.ResponseWriter, r *http.Request) {
	limit := domain.ActivityDefaultLimit
	if raw := r.URL.Query().Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n <= 0 {
			writeMessage(w, http.StatusBadRequest, "Limit has to be a positive number.")
			return
		}
		limit = n
	}

	events, err := s.db.ListRecentActivity(r.Context(), limit)
	if err != nil {
		writeMessage(w, http.StatusInternalServerError, "Could not load recent activity.")
		return
	}
	if events == nil {
		events = []domain.ActivityEvent{}
	}

	writeJSON(w, http.StatusOK, map[string]any{"events": events})
}
