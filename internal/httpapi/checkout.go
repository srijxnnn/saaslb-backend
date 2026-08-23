package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"saaslb-backend/internal/domain"
	"saaslb-backend/internal/dodo"
	"saaslb-backend/internal/store"
)

type createCheckoutRequest struct {
	Target      string   `json:"target"`
	AmountCents int      `json:"amountCents"`
	Tagline     string   `json:"tagline"`
	Categories  []string `json:"categories"`
}

func (s *Server) createCheckout(w http.ResponseWriter, r *http.Request) {
	var req createCheckoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeMessage(w, http.StatusBadRequest, "Could not read that bid.")
		return
	}

	target, err := domain.ParseListingTarget(req.Target)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	existing, err := s.db.ProductByListingKey(r.Context(), target.Key)
	var existingPtr *domain.Product
	if err == nil {
		existingPtr = &existing
	} else if !errors.Is(err, store.ErrNotFound) {
		writeMessage(w, http.StatusInternalServerError, "Could not look up that listing.")
		return
	}

	paidCents, err := domain.ValidateBid(req.AmountCents, existingPtr)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	categories, err := domain.ValidateCategories(req.Categories)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	checkout := store.Checkout{
		ID:          store.NewID("chk_"),
		ListingKey:  target.Key,
		WebsiteURL:  target.WebsiteURL,
		Name:        target.Name,
		Tagline:     strings.TrimSpace(req.Tagline),
		Categories:  categories,
		AmountCents: req.AmountCents,
		PaidCents:   paidCents,
		Status:      "pending",
		CreatedAt:   time.Now().UTC(),
	}
	if existingPtr != nil {
		id := existingPtr.ID
		checkout.ExistingProductID = &id
	}

	if err := s.db.CreateCheckout(r.Context(), checkout); err != nil {
		writeMessage(w, http.StatusInternalServerError, "Could not start checkout.")
		return
	}

	if s.cfg.PaymentsMode == "simulate" {
		product, err := s.db.FulfillCheckout(r.Context(), checkout.ID, "", s.currentPeriod())
		if err != nil && !errors.Is(err, store.ErrAlreadyProcessed) {
			writeMessage(w, http.StatusInternalServerError, "Could not apply that bid.")
			return
		}
		products, listErr := s.db.ListProducts(r.Context())
		if listErr != nil {
			writeMessage(w, http.StatusInternalServerError, "Could not load the board.")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":          true,
			"checkoutId":  checkout.ID,
			"status":      "paid",
			"paidCents":   paidCents,
			"product":     product,
			"rank":        domain.RankOf(product.ID, products),
			"checkoutUrl": nil,
		})
		return
	}

	if s.dodo == nil {
		writeMessage(w, http.StatusServiceUnavailable, "Dodo Payments is not configured yet.")
		return
	}

	session, err := s.dodo.CreateCheckout(r.Context(), dodo.CreateSessionInput{
		AmountCents: paidCents,
		ReturnURL:   s.cfg.FrontendURL + "/?checkout=" + checkout.ID,
		Metadata: map[string]string{
			"checkout_id":   checkout.ID,
			"listing_key":   checkout.ListingKey,
			"amount_cents":  itoa(checkout.AmountCents),
			"paid_cents":    itoa(checkout.PaidCents),
		},
	})
	if err != nil {
		writeMessage(w, http.StatusBadGateway, "Dodo did not start checkout.")
		return
	}
	if err := s.db.SetCheckoutSession(r.Context(), checkout.ID, session.SessionID); err != nil {
		writeMessage(w, http.StatusInternalServerError, "Saved the bid but lost the session id.")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":          true,
		"checkoutId":  checkout.ID,
		"status":      "pending",
		"paidCents":   paidCents,
		"checkoutUrl": session.CheckoutURL,
	})
}

func (s *Server) getCheckout(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	checkout, err := s.db.GetCheckout(r.Context(), id)
	if err != nil {
		writeError(w, statusFor(err), err)
		return
	}

	if checkout.Status == "pending" && s.dodo != nil && checkout.SessionID != nil {
		if synced, syncErr := s.syncCheckout(r, checkout); syncErr == nil {
			checkout = synced
		}
	}

	payload := map[string]any{
		"checkoutId": checkout.ID,
		"status":     checkout.Status,
		"paidCents":  checkout.PaidCents,
	}

	if checkout.ProductID != nil {
		product, err := s.db.GetProduct(r.Context(), *checkout.ProductID)
		if err == nil {
			products, listErr := s.db.ListProducts(r.Context())
			if listErr == nil {
				payload["product"] = product
				payload["rank"] = domain.RankOf(product.ID, products)
			}
		}
	}

	writeJSON(w, http.StatusOK, payload)
}

func (s *Server) syncCheckout(r *http.Request, checkout store.Checkout) (store.Checkout, error) {
	status, err := s.dodo.GetCheckout(r.Context(), *checkout.SessionID)
	if err != nil || status.PaymentStatus == nil {
		return checkout, err
	}

	paymentID := ""
	if status.PaymentID != nil {
		paymentID = *status.PaymentID
	}

	switch *status.PaymentStatus {
	case "succeeded":
		if _, err := s.db.FulfillCheckout(r.Context(), checkout.ID, paymentID, s.currentPeriod()); err != nil && !errors.Is(err, store.ErrAlreadyProcessed) {
			return checkout, err
		}
	case "failed", "cancelled":
		_ = s.db.MarkCheckoutFailed(r.Context(), checkout.ID, paymentID)
	default:
		return checkout, nil
	}

	return s.db.GetCheckout(r.Context(), checkout.ID)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [12]byte
	i := len(buf)
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
