package dodo

import (
	"encoding/base64"
	"net/http"
	"strconv"
	"testing"
	"time"
)

func TestVerifyWebhook(t *testing.T) {
	t.Parallel()

	secret := "whsec_" + base64.StdEncoding.EncodeToString([]byte("super-secret"))
	body := []byte(`{"type":"payment.succeeded","data":{"payment_id":"pay_1"}}`)
	id := "evt_1"
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	sig, err := Sign(secret, id, ts, body)
	if err != nil {
		t.Fatal(err)
	}

	headers := http.Header{}
	headers.Set("webhook-id", id)
	headers.Set("webhook-timestamp", ts)
	headers.Set("webhook-signature", sig)

	if err := Verify(secret, headers, body); err != nil {
		t.Fatalf("valid signature rejected: %v", err)
	}

	headers.Set("webhook-signature", "v1,nope")
	if err := Verify(secret, headers, body); err != ErrBadSignature {
		t.Fatalf("expected bad signature, got %v", err)
	}
}
