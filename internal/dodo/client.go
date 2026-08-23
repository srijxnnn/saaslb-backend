package dodo

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Client struct {
	baseURL    string
	apiKey     string
	productID  string
	httpClient *http.Client
}

func New(baseURL, apiKey, productID string) *Client {
	return &Client{
		baseURL:   baseURL,
		apiKey:    apiKey,
		productID: productID,
		httpClient: &http.Client{
			Timeout: 20 * time.Second,
		},
	}
}

type CreateSessionInput struct {
	AmountCents int
	ReturnURL   string
	Metadata    map[string]string
}

type Session struct {
	SessionID   string `json:"session_id"`
	CheckoutURL string `json:"checkout_url"`
}

type SessionStatus struct {
	ID            string  `json:"id"`
	PaymentID     *string `json:"payment_id"`
	PaymentStatus *string `json:"payment_status"`
}

type APIError struct {
	Status  int
	Code    string
	Message string
	Body    string
}

func (e APIError) Error() string {
	if e.Code != "" && e.Message != "" {
		return fmt.Sprintf("dodo %d %s: %s", e.Status, e.Code, e.Message)
	}
	if e.Message != "" {
		return fmt.Sprintf("dodo %d: %s", e.Status, e.Message)
	}
	if e.Body != "" {
		return fmt.Sprintf("dodo %d: %s", e.Status, e.Body)
	}
	return fmt.Sprintf("dodo %d", e.Status)
}

func (c *Client) CreateCheckout(ctx context.Context, in CreateSessionInput) (Session, error) {
	body := map[string]any{
		"product_cart": []map[string]any{
			{
				"product_id": c.productID,
				"quantity":   1,
				"amount":     in.AmountCents,
			},
		},
		"return_url": in.ReturnURL,
		"metadata":   in.Metadata,
		"feature_flags": map[string]any{
			"redirect_immediately": true,
		},
	}

	var session Session
	if err := c.do(ctx, http.MethodPost, "/checkouts", body, &session); err != nil {
		return Session{}, err
	}
	if session.CheckoutURL == "" {
		return Session{}, fmt.Errorf("dodo did not return a checkout_url")
	}
	return session, nil
}

func (c *Client) GetCheckout(ctx context.Context, sessionID string) (SessionStatus, error) {
	var status SessionStatus
	if err := c.do(ctx, http.MethodGet, "/checkouts/"+sessionID, nil, &status); err != nil {
		return SessionStatus{}, err
	}
	return status, nil
}

func (c *Client) do(ctx context.Context, method, path string, payload any, out any) error {
	var body io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(raw)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "application/json")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode >= 300 {
		return parseAPIError(resp.StatusCode, raw)
	}
	if out == nil || len(raw) == 0 {
		return nil
	}
	return json.Unmarshal(raw, out)
}

func parseAPIError(status int, raw []byte) APIError {
	var payload struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		Error   string `json:"error"`
	}
	if err := json.Unmarshal(raw, &payload); err == nil {
		message := payload.Message
		if message == "" {
			message = payload.Error
		}
		if payload.Code != "" || message != "" {
			return APIError{Status: status, Code: payload.Code, Message: message, Body: truncate(raw, 400)}
		}
	}
	return APIError{Status: status, Body: truncate(raw, 400)}
}

func truncate(raw []byte, n int) string {
	if len(raw) <= n {
		return string(raw)
	}
	return string(raw[:n]) + "..."
}
