package extractor

import (
	"strings"

	"github.com/ramkansal/gofang/pkg/plugin"
)

// APIExtractor extracts XHR/fetch API endpoints captured by the browser fetcher.
type APIExtractor struct{}

func NewAPIExtractor() *APIExtractor { return &APIExtractor{} }

func (e *APIExtractor) Name() string { return "api_endpoints" }

func (e *APIExtractor) Extract(page *plugin.PageData) ([]plugin.ExtractedItem, error) {
	if len(page.InterceptedReqs) == 0 {
		return nil, nil
	}

	seen := make(map[string]bool)
	var items []plugin.ExtractedItem

	for _, req := range page.InterceptedReqs {
		// Skip static assets — we only want API/XHR endpoints
		if isStaticResource(req.URL, req.ResourceType) {
			continue
		}

		key := req.Method + " " + req.URL
		if seen[key] {
			continue
		}
		seen[key] = true

		meta := map[string]string{
			"method": req.Method,
		}
		if req.ContentType != "" {
			meta["content_type"] = req.ContentType
		}
		if req.ResourceType != "" {
			meta["resource_type"] = req.ResourceType
		}

		items = append(items, plugin.ExtractedItem{
			Type:      "api_endpoint",
			Value:     req.URL,
			SourceURL: page.URL,
			Metadata:  meta,
		})
	}

	return items, nil
}

// isStaticResource returns true for common static resource types we don't want to flag as API endpoints.
func isStaticResource(url string, resourceType string) bool {
	lower := strings.ToLower(url)

	// Filter by resource type if available
	staticTypes := []string{"image", "stylesheet", "font", "media", "manifest", "texttrack"}
	for _, st := range staticTypes {
		if strings.EqualFold(resourceType, st) {
			return true
		}
	}

	// Filter by file extension
	staticExts := []string{
		".png", ".jpg", ".jpeg", ".gif", ".svg", ".ico", ".webp", ".avif",
		".css", ".woff", ".woff2", ".ttf", ".eot", ".otf",
		".mp4", ".webm", ".mp3", ".wav", ".ogg",
		".map",
	}
	for _, ext := range staticExts {
		if strings.Contains(lower, ext) {
			return true
		}
	}

	// Common tracking/analytics patterns to include (they are APIs)
	return false
}
