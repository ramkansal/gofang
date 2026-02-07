package extractor

import (
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/ramkansal/web-crawler/pkg/plugin"
)

// EmailsExtractor extracts email addresses from page content and mailto: links.
type EmailsExtractor struct {
	pattern *regexp.Regexp
}

func NewEmailsExtractor() *EmailsExtractor {
	return &EmailsExtractor{
		pattern: regexp.MustCompile(`[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`),
	}
}

func (e *EmailsExtractor) Name() string { return "emails" }

func (e *EmailsExtractor) Extract(page *plugin.PageData) ([]plugin.ExtractedItem, error) {
	html := page.RenderedHTML
	if html == "" {
		html = page.RawHTML
	}
	if html == "" {
		return nil, nil
	}

	seen := make(map[string]bool)
	var items []plugin.ExtractedItem

	addEmail := func(email, source string) {
		email = strings.ToLower(strings.TrimSpace(email))
		if email == "" || seen[email] {
			return
		}
		// Filter out common false positives
		if strings.HasSuffix(email, ".png") ||
			strings.HasSuffix(email, ".jpg") ||
			strings.HasSuffix(email, ".gif") ||
			strings.HasSuffix(email, ".css") ||
			strings.HasSuffix(email, ".js") ||
			strings.Contains(email, "example.com") ||
			strings.Contains(email, "sentry.io") ||
			strings.Contains(email, "webpack") {
			return
		}
		seen[email] = true
		items = append(items, plugin.ExtractedItem{
			Type:      "email",
			Value:     email,
			SourceURL: page.URL,
			Metadata: map[string]string{
				"source": source,
			},
		})
	}

	// Extract from mailto: links
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err == nil {
		doc.Find(`a[href^="mailto:"]`).Each(func(_ int, s *goquery.Selection) {
			href, _ := s.Attr("href")
			email := strings.TrimPrefix(href, "mailto:")
			// Remove query params (e.g., ?subject=...)
			if idx := strings.Index(email, "?"); idx != -1 {
				email = email[:idx]
			}
			addEmail(email, "mailto_link")
		})
	}

	// Extract from page text via regex
	// Strip HTML tags first for cleaner extraction
	textContent := stripTags(html)
	matches := e.pattern.FindAllString(textContent, -1)
	for _, match := range matches {
		addEmail(match, "page_text")
	}

	return items, nil
}

// stripTags removes HTML tags from a string for text extraction.
func stripTags(s string) string {
	var builder strings.Builder
	inTag := false
	for _, r := range s {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
			builder.WriteRune(' ')
		case !inTag:
			builder.WriteRune(r)
		}
	}
	return builder.String()
}
