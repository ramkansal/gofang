package extractor

import (
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/ramkansal/web-crawler/pkg/plugin"
)

// PhonesExtractor extracts phone numbers from page content.
type PhonesExtractor struct {
	patterns []*regexp.Regexp
}

func NewPhonesExtractor() *PhonesExtractor {
	return &PhonesExtractor{
		patterns: []*regexp.Regexp{
			// International format: +1-234-567-8900, +44 20 7946 0958
			regexp.MustCompile(`\+?1?[\s.-]?\(?\d{3}\)?[\s.-]?\d{3}[\s.-]?\d{4}`),
			// Intl with country code: +XX XXXXXXXXXX
			regexp.MustCompile(`\+\d{1,3}[\s.-]?\d{4,14}`),
		},
	}
}

func (e *PhonesExtractor) Name() string { return "phones" }

func (e *PhonesExtractor) Extract(page *plugin.PageData) ([]plugin.ExtractedItem, error) {
	html := page.RenderedHTML
	if html == "" {
		html = page.RawHTML
	}
	if html == "" {
		return nil, nil
	}

	seen := make(map[string]bool)
	var items []plugin.ExtractedItem

	addPhone := func(phone, source string) {
		phone = strings.TrimSpace(phone)
		// Normalize for dedup: remove all non-digit chars except +
		normalized := normalizePhone(phone)
		if len(normalized) < 7 || seen[normalized] {
			return
		}
		seen[normalized] = true
		items = append(items, plugin.ExtractedItem{
			Type:      "phone",
			Value:     phone,
			SourceURL: page.URL,
			Metadata: map[string]string{
				"normalized": normalized,
				"source":     source,
			},
		})
	}

	// Extract from tel: links
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err == nil {
		doc.Find(`a[href^="tel:"]`).Each(func(_ int, s *goquery.Selection) {
			href, _ := s.Attr("href")
			phone := strings.TrimPrefix(href, "tel:")
			addPhone(phone, "tel_link")
		})
	}

	// Extract from page text via regex
	textContent := stripTags(html)
	for _, pattern := range e.patterns {
		matches := pattern.FindAllString(textContent, -1)
		for _, match := range matches {
			addPhone(match, "page_text")
		}
	}

	return items, nil
}

func normalizePhone(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r == '+' || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}
