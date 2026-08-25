package domain

import "testing"

func TestParseListingTarget(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in      string
		key     string
		url     string
		name    string
		wantErr error
	}{
		{in: "", wantErr: ErrEmptyTarget},
		{in: "@", wantErr: ErrNeedRealSite},
		{in: "@Ada_Lovelace", wantErr: ErrNeedRealSite},
		{in: "yoursite.com", key: "yoursite.com", url: "https://yoursite.com", name: "yoursite.com"},
		{in: "https://www.Yoursite.com/App/", key: "yoursite.com/app", url: "https://yoursite.com/app", name: "yoursite.com"},
		{in: "localhost", wantErr: ErrNeedRealSite},
	}

	for _, tc := range cases {
		got, err := ParseListingTarget(tc.in)
		if tc.wantErr != nil {
			if err != tc.wantErr {
				t.Fatalf("%q: error = %v, want %v", tc.in, err, tc.wantErr)
			}
			continue
		}
		if err != nil {
			t.Fatalf("%q: unexpected error %v", tc.in, err)
		}
		if got.Key != tc.key || got.WebsiteURL != tc.url || got.Name != tc.name {
			t.Fatalf("%q: got %+v", tc.in, got)
		}
	}
}

func TestValidateBid(t *testing.T) {
	t.Parallel()

	if _, err := ValidateBid(150, nil); err != ErrWholeDollars {
		t.Fatalf("expected whole dollars, got %v", err)
	}
	paid, err := ValidateBid(0, nil)
	if err != nil || paid != 0 {
		t.Fatalf("free listing: paid=%d err=%v", paid, err)
	}
	paid, err = ValidateBid(100, nil)
	if err != nil || paid != 100 {
		t.Fatalf("new listing: paid=%d err=%v", paid, err)
	}
	if _, err := ValidateBid(-100, nil); err != ErrNeedOneDollar {
		t.Fatalf("expected negative bid rejected, got %v", err)
	}

	existing := &Product{BidCents: 1000}
	paid, err = ValidateBid(200, existing)
	if err != nil || paid != 200 {
		t.Fatalf("extra payment: paid=%d err=%v", paid, err)
	}
	if _, err := ValidateBid(0, existing); err == nil {
		t.Fatal("expected extra payment too small")
	}
}

func TestSlugify(t *testing.T) {
	t.Parallel()

	if got := Slugify("Atlas OS"); got != "atlas-os" {
		t.Fatalf("got %q", got)
	}
	if got := UniqueSlug("Atlas OS", map[string]struct{}{"atlas-os": {}}); got != "atlas-os-2" {
		t.Fatalf("got %q", got)
	}
}
