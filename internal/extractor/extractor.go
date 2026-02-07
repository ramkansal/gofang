package extractor

import (
	"github.com/ramkansal/gofang/pkg/plugin"
)

// Registry holds all available extractors.
type Registry struct {
	extractors []plugin.Extractor
}

// NewRegistry creates a registry with all built-in extractors.
func NewRegistry() *Registry {
	return &Registry{
		extractors: []plugin.Extractor{
			NewLinksExtractor(),
			NewFormsExtractor(),
			NewEmailsExtractor(),
			NewPhonesExtractor(),
			NewSocialExtractor(),
			NewMetadataExtractor(),
			NewAssetsExtractor(),
			NewAPIExtractor(),
		},
	}
}

// Register adds a custom extractor to the registry.
func (r *Registry) Register(ext plugin.Extractor) {
	r.extractors = append(r.extractors, ext)
}

// ExtractAll runs all registered extractors against the given page.
func (r *Registry) ExtractAll(page *plugin.PageData) ([]plugin.ExtractedItem, error) {
	var allItems []plugin.ExtractedItem
	for _, ext := range r.extractors {
		items, err := ext.Extract(page)
		if err != nil {
			// log but don't abort — other extractors should still run
			continue
		}
		allItems = append(allItems, items...)
	}
	return allItems, nil
}

// Names returns the names of all registered extractors.
func (r *Registry) Names() []string {
	names := make([]string, len(r.extractors))
	for i, ext := range r.extractors {
		names[i] = ext.Name()
	}
	return names
}
