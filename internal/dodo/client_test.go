package dodo

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreateCheckoutSendsFeatureFlags(t *testing.T) {
	t.Parallel()

	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/checkouts" {
			t.Errorf("path = %s", r.URL.Path)
		}
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(raw, &body); err != nil {
			t.Fatal(err)
		}
		_ = json.NewEncoder(w).Encode(Session{
			SessionID:   "cks_1",
			CheckoutURL: "https://checkout.dodopayments.com/cks_1",
		})
	}))
	t.Cleanup(server.Close)

	client := New(server.URL, "test-key", "pdt_test")
	session, err := client.CreateCheckout(context.Background(), CreateSessionInput{
		AmountCents: 15100,
		ReturnURL:   "https://saasleaderboards.com/?checkout=chk_1",
		Metadata:    map[string]string{"checkout_id": "chk_1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if session.CheckoutURL == "" {
		t.Fatal("missing checkout url")
	}

	if _, ok := body["redirect_immediately"]; ok {
		t.Fatal("redirect_immediately must live under feature_flags")
	}
	flags, ok := body["feature_flags"].(map[string]any)
	if !ok {
		t.Fatalf("feature_flags = %#v", body["feature_flags"])
	}
	if flags["redirect_immediately"] != true {
		t.Fatalf("redirect_immediately = %#v", flags["redirect_immediately"])
	}

	cart, _ := body["product_cart"].([]any)
	if len(cart) != 1 {
		t.Fatalf("product_cart = %#v", body["product_cart"])
	}
	item, _ := cart[0].(map[string]any)
	if item["amount"] != float64(15100) || item["product_id"] != "pdt_test" {
		t.Fatalf("cart item = %#v", item)
	}
}

func TestParseAPIError(t *testing.T) {
	t.Parallel()

	err := parseAPIError(401, []byte(`{"code":"UNAUTHORIZED","message":"You are not authorised to perform this action"}`))
	if err.Code != "UNAUTHORIZED" {
		t.Fatalf("code = %q", err.Code)
	}
	if err.Error() != "dodo 401 UNAUTHORIZED: You are not authorised to perform this action" {
		t.Fatalf("error = %q", err.Error())
	}
}
