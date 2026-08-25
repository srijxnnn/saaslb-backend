package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"saaslb-backend/internal/domain"
	"saaslb-backend/internal/store"
)

func (s *Server) listProducts(w http.ResponseWriter, r *http.Request) {
	products, err := s.db.ListProducts(r.Context())
	if err != nil {
		writeMessage(w, http.StatusInternalServerError, "Could not load the board.")
		return
	}

	category := strings.TrimSpace(r.URL.Query().Get("category"))
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	filtered := make([]domain.Product, 0, len(products))

	for _, product := range products {
		if category != "" && !contains(product.Categories, category) {
			continue
		}
		if query != "" && !matchesQuery(product, query) {
			continue
		}
		filtered = append(filtered, product)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"products": filtered,
	})
}

func (s *Server) getProduct(w http.ResponseWriter, r *http.Request) {
	product, err := s.db.GetProduct(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, statusFor(err), err)
		return
	}

	products, err := s.db.ListProducts(r.Context())
	if err != nil {
		writeMessage(w, http.StatusInternalServerError, "Could not load the board.")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"product": product,
		"rank":    domain.RankOf(product.ID, products),
	})
}

func (s *Server) refreshProductMeta(w http.ResponseWriter, r *http.Request) {
	product, err := s.db.RefreshListingMeta(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		if errors.Is(err, store.ErrMetaCooldown) {
			writeMessage(w, http.StatusTooManyRequests, "Already refreshed that listing a moment ago.")
			return
		}
		if errors.Is(err, store.ErrNotFound) {
			writeMessage(w, http.StatusNotFound, "Listing not found.")
			return
		}
		if errors.Is(err, store.ErrSiteUnreadable) {
			writeMessage(w, http.StatusBadGateway, "Could not read that site.")
			return
		}
		writeMessage(w, http.StatusInternalServerError, "Could not refresh that listing.")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"product": product})
}

type clickRequest struct {
	VisitorID string `json:"visitorId"`
}

func (s *Server) recordClick(w http.ResponseWriter, r *http.Request) {
	var req clickRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeMessage(w, http.StatusBadRequest, "Could not read that click.")
		return
	}

	visitorID, err := domain.NormalizeVisitorID(req.VisitorID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	clicks, counted, err := s.db.IncrementClicks(r.Context(), chi.URLParam(r, "id"), visitorID)
	if err != nil {
		if err == store.ErrNotFound {
			writeMessage(w, http.StatusNotFound, "Listing not found.")
			return
		}
		writeMessage(w, http.StatusInternalServerError, "Could not record that click.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"clicks": clicks, "counted": counted})
}

func contains(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}

func matchesQuery(product domain.Product, query string) bool {
	needle := strings.ToLower(query)
	haystack := strings.ToLower(strings.Join(append([]string{
		product.Name,
		product.Tagline,
		product.WebsiteURL,
		product.ListingKey,
	}, product.Categories...), " "))
	return strings.Contains(haystack, needle)
}
