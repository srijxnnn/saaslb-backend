package metadesc

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDescriptionFromHTML(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		html string
		want string
	}{
		{
			name: "prefers open graph",
			html: `<meta name="description" content="plain">
<meta property="og:description" content="from og">
<meta name="twitter:description" content="from twitter">`,
			want: "from og",
		},
		{
			name: "falls back to twitter then description",
			html: `<meta name="twitter:description" content="from twitter">
<meta name="description" content="plain">`,
			want: "from twitter",
		},
		{
			name: "content can come before name",
			html: `<meta content="reversed" name="description">`,
			want: "reversed",
		},
		{
			name: "single quotes and entities",
			html: `<meta name='description' content='Ship &amp; sell without a &quot;wiki&quot;'>`,
			want: `Ship & sell without a "wiki"`,
		},
		{
			name: "collapses whitespace",
			html: `<meta name="description" content="  one   two	three  ">`,
			want: "one two three",
		},
		{
			name: "falls back to title",
			html: `<title>No tags</title>`,
			want: "No tags",
		},
		{
			name: "prefers description over title",
			html: `<title>Tab title</title>
<meta name="description" content="the real tagline">`,
			want: "the real tagline",
		},
		{
			name: "falls back to og title",
			html: `<meta property="og:title" content="Brand from OG">`,
			want: "Brand from OG",
		},
		{
			name: "reads json-ld description",
			html: `<script type="application/ld+json">{"name":"Acme","description":"From JSON-LD"}</script>`,
			want: "From JSON-LD",
		},
		{
			name: "ignores redirect titles",
			html: `<title>302 Moved</title>`,
			want: "",
		},
		{
			name: "empty when missing",
			html: `<html><body>nothing useful</body></html>`,
			want: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := DescriptionFromHTML(tc.html); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestDescriptionFromHTMLTruncates(t *testing.T) {
	t.Parallel()

	long := strings.Repeat("word ", 80)
	got := DescriptionFromHTML(`<meta name="description" content="` + long + `">`)
	if got == "" {
		t.Fatal("expected a truncated tagline")
	}
	if n := len([]rune(got)); n > MaxTaglineRunes+1 {
		t.Fatalf("tagline too long: %d runes", n)
	}
	if !strings.HasSuffix(got, "…") {
		t.Fatalf("expected ellipsis, got %q", got)
	}
}

func TestFetch(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<meta property="og:description" content="Live from the site.">`))
	}))
	defer server.Close()

	got, err := Fetch(context.Background(), server.URL)
	if err != nil {
		t.Fatal(err)
	}
	if got.Tagline != "Live from the site." {
		t.Fatalf("got %q", got.Tagline)
	}
	if got.IconURL == "" {
		t.Fatal("expected a fallback favicon")
	}
}

func TestFetchExactDoesNotInventTagline(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	got, err := FetchExact(context.Background(), server.URL)
	if err == nil {
		t.Fatal("expected fetch error")
	}
	if got.Tagline != "" {
		t.Fatalf("should not invent a tagline, got %q", got.Tagline)
	}
	if got.IconURL != "" {
		t.Fatalf("should not invent an icon, got %q", got.IconURL)
	}
}

func TestFetchRejectsNonHTTP(t *testing.T) {
	t.Parallel()

	if _, err := Fetch(context.Background(), "file:///etc/passwd"); err == nil {
		t.Fatal("expected rejected URL")
	}
}

func TestIconFromHTML(t *testing.T) {
	t.Parallel()

	base := "https://mail.example/app"
	got := IconFromHTML(`
<link rel="icon" href="/favicon-32.png" sizes="32x32">
<link rel="apple-touch-icon" href="/apple-touch-icon.png" sizes="180x180">
`, base)
	if got != "https://mail.example/apple-touch-icon.png" {
		t.Fatalf("got %q", got)
	}

	got = IconFromHTML(`<link rel="icon" type="image/svg+xml" href="//cdn.example/mark.svg">`, base)
	if got != "https://cdn.example/mark.svg" {
		t.Fatalf("protocol-relative got %q", got)
	}

	if IconFromHTML(`<p>no icons</p>`, base) != "" {
		t.Fatal("expected empty")
	}
}

func TestBrandFromURL(t *testing.T) {
	t.Parallel()

	if got := BrandFromURL("https://gmail.com"); got != "Gmail" {
		t.Fatalf("got %q", got)
	}
	if got := BrandFromURL("https://www.stripe.com/pricing"); got != "Stripe" {
		t.Fatalf("got %q", got)
	}
	if got := BrandFromURL("http://127.0.0.1:8080"); got != "" {
		t.Fatalf("ip should not become a brand, got %q", got)
	}
}
