package httpapi

import (
	"testing"

	"saaslb-backend/internal/dodo"
)

func TestCheckoutStartError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		err  error
		want string
	}{
		{
			err:  dodo.APIError{Status: 401, Code: "UNAUTHORIZED", Message: "You are not authorised to perform this action"},
			want: "Dodo rejected the API key. Check that DODO_ENVIRONMENT matches the key (test vs live).",
		},
		{
			err:  dodo.APIError{Status: 403, Code: "MERCHANT_NOT_LIVE", Message: "Merchant is not live"},
			want: "Dodo business is still in test mode. Use test keys or finish live activation.",
		},
		{
			err:  dodo.APIError{Status: 404, Code: "NOT_FOUND", Message: "Item not found"},
			want: "Dodo product was not found in this environment. Check DODO_PRODUCT_ID.",
		},
		{
			err:  dodo.APIError{Status: 422, Code: "WEIRD", Message: "Country AI currently not supported"},
			want: "Dodo did not start checkout. Country AI currently not supported",
		},
	}

	for _, tc := range tests {
		if got := checkoutStartError(tc.err); got != tc.want {
			t.Fatalf("checkoutStartError(%v) = %q, want %q", tc.err, got, tc.want)
		}
	}
}
