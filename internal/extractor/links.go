package extractor

import (
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/ramkansal/web-crawler/pkg/plugin"
)

// LinksExtractor extracts all hyperlinks from a page.
type LinksExtractor struct{}

func NewLinksExtractor() *LinksExtractor { return &LinksExtractor{} }

func (e *LinksExtractor) Name() string { return "links" }

func (e *LinksExtractor) Extract(page *plugin.PageData) ([]plugin.ExtractedItem, error) {
	html := page.RenderedHTML
	if html == "" {
		html = page.RawHTML
	}
	if html == "" {
		return nil, nil
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, err
	}

	baseURL, err := url.Parse(page.FinalURL)
	if err != nil {
		baseURL, _ = url.Parse(page.URL)
	}

	seen := make(map[string]bool)
	var items []plugin.ExtractedItem

	doc.Find("a[href]").Each(func(_ int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if !exists || href == "" {
			return
		}

		// Skip fragments, javascript:, mailto:, tel:
		trimmed := strings.TrimSpace(href)
		if strings.HasPrefix(trimmed, "#") ||
			strings.HasPrefix(trimmed, "javascript:") ||
			strings.HasPrefix(trimmed, "mailto:") ||
			strings.HasPrefix(trimmed, "tel:") {
			return
		}

		// Resolve relative URL
		resolved := resolveURL(baseURL, trimmed)
		if resolved == "" || seen[resolved] {
			return
		}
		seen[resolved] = true

		// Classify as internal or external
		resolvedParsed, err := url.Parse(resolved)
		linkType := "external"
		if err == nil && resolvedParsed.Host == baseURL.Host {
			linkType = "internal"
		}

		text := strings.TrimSpace(s.Text())
		rel, _ := s.Attr("rel")

		meta := map[string]string{
			"link_type": linkType,
		}
		if text != "" {
			meta["anchor_text"] = truncate(text, 200)
		}
		if rel != "" {
			meta["rel"] = rel
		}

		items = append(items, plugin.ExtractedItem{
			Type:      "link",
			Value:     resolved,
			SourceURL: page.URL,
			Metadata:  meta,
		})
	})

	return items, nil
}

// resolveURL resolves a potentially relative URL against a base URL.
func resolveURL(base *url.URL, raw string) string {
	if base == nil {
		return raw
	}
	ref, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return base.ResolveReference(ref).String()
}

// truncate limits a string to maxLen characters.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
