package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"saaslb-backend/internal/domain"
	"saaslb-backend/internal/store"
)

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func writeMessage(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func statusFor(err error) int {
	switch {
	case errors.Is(err, store.ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, domain.ErrEmptyTarget),
		errors.Is(err, domain.ErrNeedRealSite),
		errors.Is(err, domain.ErrUnreadable),
		errors.Is(err, domain.ErrWholeDollars),
		errors.Is(err, domain.ErrBidTooHigh),
		errors.Is(err, domain.ErrNeedOneDollar),
		errors.Is(err, domain.ErrTooManyCats),
		errors.Is(err, domain.ErrNeedCategory),
		errors.Is(err, domain.ErrUnknownCat):
		return http.StatusBadRequest
	default:
		var low *domain.BidTooLowError
		if errors.As(err, &low) {
			return http.StatusBadRequest
		}
		return http.StatusInternalServerError
	}
}
