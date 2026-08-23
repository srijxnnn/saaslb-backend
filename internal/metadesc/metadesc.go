// Package metadesc pulls a short description from a listing's website.
// The board already stores a tagline; this fills or refreshes it from
// the page's own SEO tags instead of asking the bidder to write one.
package metadesc

import (
	"context"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const (
	// MaxTaglineRunes keeps leaderboard cards readable. Meta tags are often
	// 150–160 characters; 240 leaves room without dumping a paragraph.
	MaxTaglineRunes = 240
	maxHTMLBytes    = 512 << 10
	fetchTimeout    = 5 * time.Second
)

var (
	client = &http.Client{
		Timeout: fetchTimeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("too many redirects")
			}
			if !httpScheme(req.URL) {
				return fmt.Errorf("redirect left http(s)")
			}
			return nil
		},
	}

	metaTagRE  = regexp.MustCompile(`(?is)<meta\b([^>]*)>`)
	titleRE    = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
	innerTagRE = regexp.MustCompile(`(?s)<[^>]+>`)
	ldRE       = regexp.MustCompile(`(?is)<script[^>]*type=["']application/ld\+json["'][^>]*>(.*?)</script>`)
	ldDescRE   = regexp.MustCompile(`"description"\s*:\s*"((?:\\.|[^"\\])*)"`)
	attrRE     = regexp.MustCompile(`(?i)([^\s=]+)\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s"'>]+))`)
)

type PageInfo struct {
	Tagline string
	IconURL string
}

// Fetch loads pageURL and returns the best meta description and icon it can
// find. Login walls often have no tags, so we retry www and finally the host label.
func Fetch(ctx context.Context, pageURL string) (PageInfo, error) {
	var info PageInfo
	if err := validateURL(pageURL); err != nil {
		return info, err
	}

	// A checkout or webhook request can be close to its own deadline.
	// Scraping needs a full window even if the parent context is short.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), fetchTimeout)
	defer cancel()

	info, err := fetchOnce(ctx, pageURL)
	if info.Tagline != "" && info.IconURL != "" {
		return info, nil
	}

	if www := withWWW(pageURL); www != pageURL {
		if retry, wwwErr := fetchOnce(ctx, www); wwwErr == nil {
			if info.Tagline == "" {
				info.Tagline = retry.Tagline
			}
			if info.IconURL == "" {
				info.IconURL = retry.IconURL
			}
		} else if err == nil {
			err = wwwErr
		}
	}

	if info.Tagline == "" {
		info.Tagline = BrandFromURL(pageURL)
	}
	if info.IconURL == "" {
		info.IconURL = FallbackIcon(pageURL)
	}
	return info, err
}

func fetchOnce(ctx context.Context, pageURL string) (PageInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
	if err != nil {
		return PageInfo{}, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	resp, err := client.Do(req)
	if err != nil {
		return PageInfo{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return PageInfo{}, fmt.Errorf("site returned %s", resp.Status)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxHTMLBytes))
	if err != nil {
		return PageInfo{}, err
	}

	base := pageURL
	if resp.Request != nil && resp.Request.URL != nil {
		base = resp.Request.URL.String()
	}
	htmlDoc := string(raw)
	icon := IconFromHTML(htmlDoc, base)
	if icon == "" {
		icon = FallbackIcon(pageURL)
	}
	return PageInfo{
		Tagline: DescriptionFromHTML(htmlDoc),
		IconURL: icon,
	}, nil
}

// DescriptionFromHTML prefers Open Graph, then Twitter, then the classic
// description tag. Login walls and app shells often have none of those, so
// we also try JSON-LD, titles, and ignore "302 Moved" junk.
func DescriptionFromHTML(doc string) string {
	var og, twitter, plain, ogTitle, twitterTitle, appName, siteName string

	for _, match := range metaTagRE.FindAllStringSubmatch(doc, -1) {
		attrs := parseAttrs(match[1])
		name := strings.ToLower(firstAttr(attrs, "property", "name", "itemprop"))
		content := strings.TrimSpace(attrs["content"])
		if content == "" {
			continue
		}
		switch name {
		case "og:description":
			og = content
		case "twitter:description":
			twitter = content
		case "description":
			plain = content
		case "og:title":
			ogTitle = content
		case "twitter:title":
			twitterTitle = content
		case "application-name":
			appName = content
		case "og:site_name":
			siteName = content
		}
	}

	if desc := usable(firstNonEmpty(og, twitter, plain, jsonLDDescription(doc))); desc != "" {
		return desc
	}
	return usable(firstNonEmpty(ogTitle, twitterTitle, appName, siteName, htmlTitle(doc)))
}

func jsonLDDescription(doc string) string {
	for _, match := range ldRE.FindAllStringSubmatch(doc, -1) {
		found := ldDescRE.FindStringSubmatch(match[1])
		if len(found) < 2 {
			continue
		}
		quoted := `"` + found[1] + `"`
		value, err := strconv.Unquote(quoted)
		if err != nil {
			value = found[1]
		}
		if value != "" {
			return value
		}
	}
	return ""
}

func usable(value string) string {
	value = clean(value)
	if value == "" {
		return ""
	}
	lower := strings.ToLower(value)
	if strings.Contains(lower, "just a moment") {
		return ""
	}
	if strings.Contains(lower, "moved") && (strings.Contains(lower, "301") || strings.Contains(lower, "302") || strings.Contains(lower, "303")) {
		return ""
	}
	if strings.Contains(lower, "404") && (strings.Contains(lower, "error") || strings.Contains(lower, "not found")) {
		return ""
	}
	return value
}

func withWWW(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" {
		return raw
	}
	host := parsed.Hostname()
	if strings.HasPrefix(strings.ToLower(host), "www.") {
		return raw
	}
	parsed.Host = "www." + host
	return parsed.String()
}

func BrandFromURL(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	host := strings.TrimPrefix(strings.ToLower(parsed.Hostname()), "www.")
	label := strings.Split(host, ".")[0]
	if label == "" || !unicode.IsLetter(rune(label[0])) {
		return ""
	}
	return strings.ToUpper(label[:1]) + label[1:]
}

func htmlTitle(doc string) string {
	match := titleRE.FindStringSubmatch(doc)
	if len(match) < 2 {
		return ""
	}
	return innerTagRE.ReplaceAllString(match[1], "")
}

func parseAttrs(raw string) map[string]string {
	attrs := map[string]string{}
	for _, match := range attrRE.FindAllStringSubmatch(raw, -1) {
		value := match[2]
		if value == "" {
			value = match[3]
		}
		if value == "" {
			value = match[4]
		}
		attrs[strings.ToLower(match[1])] = value
	}
	return attrs
}

func firstAttr(attrs map[string]string, keys ...string) string {
	for _, key := range keys {
		if value := attrs[key]; value != "" {
			return value
		}
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func clean(value string) string {
	value = html.UnescapeString(value)
	value = strings.ReplaceAll(value, "\u00a0", " ")
	value = strings.Join(strings.Fields(value), " ")
	value = strings.TrimSpace(value)
	if value == "" || !utf8.ValidString(value) {
		return ""
	}

	runes := []rune(value)
	if len(runes) <= MaxTaglineRunes {
		return value
	}
	trimmed := strings.TrimRightFunc(string(runes[:MaxTaglineRunes]), unicode.IsSpace)
	return strings.TrimRight(trimmed, ".,;:—- ") + "…"
}

func validateURL(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" || !httpScheme(parsed) {
		return fmt.Errorf("need an http(s) website")
	}
	return nil
}

func httpScheme(u *url.URL) bool {
	scheme := strings.ToLower(u.Scheme)
	return scheme == "http" || scheme == "https"
}

func looksLikeHTML(contentType string) bool {
	if contentType == "" {
		return true
	}
	media := strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
	return media == "text/html" || media == "application/xhtml+xml" || strings.HasPrefix(media, "text/")
}
