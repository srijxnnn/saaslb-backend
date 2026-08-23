package dodo

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const webhookTolerance = 5 * time.Minute

var (
	ErrMissingHeaders = errors.New("missing webhook headers")
	ErrBadSignature   = errors.New("invalid webhook signature")
	ErrStaleTimestamp = errors.New("webhook timestamp is too old")
)

type Event struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

type PaymentData struct {
	PaymentID    string            `json:"payment_id"`
	Status       string            `json:"status"`
	TotalAmount  int               `json:"total_amount"`
	Metadata     map[string]string `json:"metadata"`
	CheckoutID   string            `json:"checkout_session_id"`
	SessionID    string            `json:"session_id"`
}

func (p PaymentData) LookupCheckoutID() string {
	if p.Metadata != nil {
		if id := p.Metadata["checkout_id"]; id != "" {
			return id
		}
	}
	if p.CheckoutID != "" {
		return p.CheckoutID
	}
	return p.SessionID
}

func ParseEvent(raw []byte) (Event, error) {
	var event Event
	if err := json.Unmarshal(raw, &event); err != nil {
		return Event{}, err
	}
	return event, nil
}

func ParsePayment(event Event) (PaymentData, error) {
	var payment PaymentData
	if err := json.Unmarshal(event.Data, &payment); err != nil {
		return PaymentData{}, err
	}

	// Dodo sometimes nests metadata values as non-strings.
	var loose struct {
		PaymentID   string          `json:"payment_id"`
		Status      string          `json:"status"`
		TotalAmount int             `json:"total_amount"`
		Metadata    json.RawMessage `json:"metadata"`
	}
	if err := json.Unmarshal(event.Data, &loose); err == nil && len(loose.Metadata) > 0 && payment.Metadata == nil {
		payment.Metadata = stringifyMetadata(loose.Metadata)
	}
	if payment.PaymentID == "" {
		payment.PaymentID = loose.PaymentID
	}
	return payment, nil
}

func stringifyMetadata(raw json.RawMessage) map[string]string {
	var asMap map[string]any
	if err := json.Unmarshal(raw, &asMap); err != nil {
		return nil
	}
	out := make(map[string]string, len(asMap))
	for key, value := range asMap {
		out[key] = fmt.Sprint(value)
	}
	return out
}

func Verify(secret string, headers http.Header, body []byte) error {
	id := header(headers, "webhook-id")
	timestamp := header(headers, "webhook-timestamp")
	signatures := header(headers, "webhook-signature")
	if id == "" || timestamp == "" || signatures == "" {
		return ErrMissingHeaders
	}

	unix, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return ErrStaleTimestamp
	}
	if d := time.Since(time.Unix(unix, 0)); d > webhookTolerance || d < -webhookTolerance {
		return ErrStaleTimestamp
	}

	key, err := decodeSecret(secret)
	if err != nil {
		return err
	}

	signed := id + "." + timestamp + "." + string(body)
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(signed))
	expected := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	for _, part := range strings.Fields(signatures) {
		version, sig, ok := strings.Cut(part, ",")
		if !ok || version != "v1" {
			continue
		}
		if hmac.Equal([]byte(sig), []byte(expected)) {
			return nil
		}
	}
	return ErrBadSignature
}

func decodeSecret(secret string) ([]byte, error) {
	trimmed := strings.TrimSpace(secret)
	trimmed = strings.TrimPrefix(trimmed, "whsec_")
	if decoded, err := base64.StdEncoding.DecodeString(trimmed); err == nil && len(decoded) > 0 {
		return decoded, nil
	}
	if trimmed == "" {
		return nil, errors.New("empty webhook secret")
	}
	return []byte(trimmed), nil
}

func header(headers http.Header, key string) string {
	if value := headers.Get(key); value != "" {
		return value
	}
	return headers.Get(http.CanonicalHeaderKey(key))
}

func Sign(secret, id, timestamp string, body []byte) (string, error) {
	key, err := decodeSecret(secret)
	if err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(id + "." + timestamp + "." + string(body)))
	return "v1," + base64.StdEncoding.EncodeToString(mac.Sum(nil)), nil
}
