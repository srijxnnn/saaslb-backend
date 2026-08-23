package metadesc

import (
	"html"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

var linkTagRE = regexp.MustCompile(`(?is)<link\b([^>]*)>`)

type iconCand struct {
	href  string
	score int
}

// IconFromHTML picks the best favicon from link tags and resolves it
// against pageURL so relative hrefs become something the board can load.
func IconFromHTML(doc, pageURL string) string {
	var best iconCand

	for _, match := range linkTagRE.FindAllStringSubmatch(doc, -1) {
		attrs := parseAttrs(match[1])
		href := resolveURL(pageURL, firstAttr(attrs, "href"))
		if href == "" {
			continue
		}

		rel := strings.ToLower(firstAttr(attrs, "rel"))
		score := iconScore(rel, firstAttr(attrs, "sizes"), firstAttr(attrs, "type"))
		if score == 0 {
			continue
		}
		if score > best.score {
			best = iconCand{href: href, score: score}
		}
	}

	return best.href
}

func FallbackIcon(pageURL string) string {
	parsed, err := url.Parse(pageURL)
	if err != nil || parsed.Host == "" || !httpScheme(parsed) {
		return ""
	}
	host := parsed.Hostname()
	if host == "" {
		return ""
	}
	return "https://" + host + "/favicon.ico"
}

func iconScore(rel, sizes, typ string) int {
	if isAppleTouch(rel) {
		return 200 + sizeScore(sizes)
	}
	if relHas(rel, "icon") || relHas(rel, "shortcut") {
		score := 40 + sizeScore(sizes)
		if strings.Contains(strings.ToLower(typ), "svg") || strings.HasSuffix(strings.ToLower(rel), "svg") {
			score += 120
		}
		return score
	}
	return 0
}

func isAppleTouch(rel string) bool {
	for _, part := range strings.Fields(rel) {
		if strings.HasPrefix(part, "apple-touch-icon") {
			return true
		}
	}
	return false
}

func relHas(rel, needle string) bool {
	for _, part := range strings.Fields(rel) {
		if part == needle {
			return true
		}
	}
	return false
}

func sizeScore(sizes string) int {
	best := 0
	for _, part := range strings.Fields(strings.ToLower(sizes)) {
		if part == "any" {
			if best < 256 {
				best = 256
			}
			continue
		}
		width, _, ok := strings.Cut(part, "x")
		n, err := strconv.Atoi(width)
		if err != nil || !ok || n <= 0 {
			continue
		}
		if n > best {
			best = n
		}
	}
	return best
}

func resolveURL(base, href string) string {
	href = strings.TrimSpace(html.UnescapeString(href))
	if href == "" {
		return ""
	}
	ref, err := url.Parse(href)
	if err != nil {
		return ""
	}
	parsed, err := url.Parse(base)
	if err != nil {
		return ""
	}
	resolved := parsed.ResolveReference(ref)
	if !httpScheme(resolved) {
		return ""
	}
	return resolved.String()
}
