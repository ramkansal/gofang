package extractor

import (
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/ramkansal/gofang/pkg/plugin"
)

// MetadataExtractor extracts page metadata: title, meta tags, headings, etc.
type MetadataExtractor struct{}

func NewMetadataExtractor() *MetadataExtractor { return &MetadataExtractor{} }

func (e *MetadataExtractor) Name() string { return "metadata" }

func (e *MetadataExtractor) Extract(page *plugin.PageData) ([]plugin.ExtractedItem, error) {
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

	var items []plugin.ExtractedItem

	// Page title
	title := strings.TrimSpace(doc.Find("title").First().Text())
	if title != "" {
		items = append(items, plugin.ExtractedItem{
			Type:      "metadata",
			Value:     title,
			SourceURL: page.URL,
			Metadata:  map[string]string{"field": "title"},
		})
	}

	// Meta tags: description, keywords, author, viewport, robots
	doc.Find("meta").Each(func(_ int, s *goquery.Selection) {
		name, _ := s.Attr("name")
		property, _ := s.Attr("property")
		content, _ := s.Attr("content")
		httpEquiv, _ := s.Attr("http-equiv")

		if content == "" {
			return
		}

		key := name
		if key == "" {
			key = property
		}
		if key == "" {
			key = httpEquiv
		}
		if key == "" {
			return
		}

		items = append(items, plugin.ExtractedItem{
			Type:      "metadata",
			Value:     content,
			SourceURL: page.URL,
			Metadata:  map[string]string{"field": strings.ToLower(key)},
		})
	})

	// Headings hierarchy (h1-h6)
	for level := 1; level <= 6; level++ {
		selector := "h" + string(rune('0'+level))
		doc.Find(selector).Each(func(_ int, s *goquery.Selection) {
			text := strings.TrimSpace(s.Text())
			if text == "" {
				return
			}
			items = append(items, plugin.ExtractedItem{
				Type:      "metadata",
				Value:     truncate(text, 300),
				SourceURL: page.URL,
				Metadata:  map[string]string{"field": selector},
			})
		})
	}

	// Canonical URL
	doc.Find(`link[rel="canonical"]`).Each(func(_ int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if exists && href != "" {
			items = append(items, plugin.ExtractedItem{
				Type:      "metadata",
				Value:     href,
				SourceURL: page.URL,
				Metadata:  map[string]string{"field": "canonical"},
			})
		}
	})

	// Language
	lang, exists := doc.Find("html").Attr("lang")
	if exists && lang != "" {
		items = append(items, plugin.ExtractedItem{
			Type:      "metadata",
			Value:     lang,
			SourceURL: page.URL,
			Metadata:  map[string]string{"field": "language"},
		})
	}

	// JSON-LD structured data
	doc.Find(`script[type="application/ld+json"]`).Each(func(i int, s *goquery.Selection) {
		jsonLD := strings.TrimSpace(s.Text())
		if jsonLD != "" {
			items = append(items, plugin.ExtractedItem{
				Type:      "metadata",
				Value:     truncate(jsonLD, 2000),
				SourceURL: page.URL,
				Metadata:  map[string]string{"field": "json-ld", "index": string(rune('0' + i))},
			})
		}
	})

	return items, nil
}
