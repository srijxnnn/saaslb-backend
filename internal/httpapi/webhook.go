package httpapi

import (
	"errors"
	"io"
	"net/http"

	"saaslb-backend/internal/dodo"
	"saaslb-backend/internal/store"
)

func (s *Server) dodoWebhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeMessage(w, http.StatusBadRequest, "empty webhook body")
		return
	}

	if s.cfg.DodoWebhookKey != "" {
		if err := dodo.Verify(s.cfg.DodoWebhookKey, r.Header, body); err != nil {
			writeError(w, http.StatusUnauthorized, err)
			return
		}
	}

	event, err := dodo.ParseEvent(body)
	if err != nil {
		writeMessage(w, http.StatusBadRequest, "could not parse webhook")
		return
	}

	webhookID := r.Header.Get("webhook-id")
	if webhookID != "" {
		fresh, err := s.db.ClaimWebhook(r.Context(), webhookID, event.Type)
		if err != nil {
			writeMessage(w, http.StatusInternalServerError, "could not record webhook")
			return
		}
		if !fresh {
			writeJSON(w, http.StatusOK, map[string]any{"received": true, "duplicate": true})
			return
		}
	}

	switch event.Type {
	case "payment.succeeded":
		if err := s.handlePaymentSucceeded(r, event); err != nil {
			writeMessage(w, http.StatusInternalServerError, err.Error())
			return
		}
	case "payment.failed", "payment.cancelled":
		_ = s.handlePaymentFailed(r, event)
	}

	writeJSON(w, http.StatusOK, map[string]any{"received": true})
}

func (s *Server) handlePaymentSucceeded(r *http.Request, event dodo.Event) error {
	payment, err := dodo.ParsePayment(event)
	if err != nil {
		return err
	}

	checkout, err := s.lookupCheckout(r, payment)
	if errors.Is(err, store.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}

	_, err = s.db.FulfillCheckout(r.Context(), checkout.ID, payment.PaymentID, s.currentPeriod())
	if errors.Is(err, store.ErrAlreadyProcessed) {
		return nil
	}
	return err
}

func (s *Server) handlePaymentFailed(r *http.Request, event dodo.Event) error {
	payment, err := dodo.ParsePayment(event)
	if err != nil {
		return err
	}
	checkout, err := s.lookupCheckout(r, payment)
	if err != nil {
		return err
	}
	return s.db.MarkCheckoutFailed(r.Context(), checkout.ID, payment.PaymentID)
}

func (s *Server) lookupCheckout(r *http.Request, payment dodo.PaymentData) (store.Checkout, error) {
	if id := payment.LookupCheckoutID(); id != "" {
		if checkout, err := s.db.GetCheckout(r.Context(), id); err == nil {
			return checkout, nil
		}
		if checkout, err := s.db.CheckoutBySession(r.Context(), id); err == nil {
			return checkout, nil
		}
	}
	if payment.PaymentID != "" {
		if checkout, err := s.db.CheckoutByPayment(r.Context(), payment.PaymentID); err == nil {
			return checkout, nil
		}
	}
	return store.Checkout{}, store.ErrNotFound
}
