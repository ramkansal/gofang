// Package plugin defines the public interfaces for the web-crawler.
// External tools can import this package to write custom extractors,
// fetchers, or output writers without forking the project.
package plugin

import (
	"net/http"
	"time"
)

// ---------- Core Data Types ----------

// PageData represents a fully fetched web page with all available data.
type PageData struct {
	URL             string               `json:"url"`
	FinalURL        string               `json:"final_url"`
	StatusCode      int                  `json:"status_code"`
	Headers         http.Header          `json:"-"`
	RawHTML         string               `json:"-"`
	RenderedHTML    string               `json:"-"`
	ContentType     string               `json:"content_type"`
	FetchedAt       time.Time            `json:"fetched_at"`
	FetchDuration   time.Duration        `json:"fetch_duration"`
	FetcherUsed     string               `json:"fetcher_used"`
	InterceptedReqs []InterceptedRequest `json:"intercepted_requests,omitempty"`
	Error           string               `json:"error,omitempty"`
	Depth           int                  `json:"depth"`
	ResponseSize    int                  `json:"response_size"`
	Technologies    []string             `json:"technologies,omitempty"`
}

// InterceptedRequest represents an XHR/fetch request captured by the browser fetcher.
type InterceptedRequest struct {
	URL          string `json:"url"`
	Method       string `json:"method"`
	ContentType  string `json:"content_type,omitempty"`
	ResourceType string `json:"resource_type,omitempty"`
}

// ExtractedItem represents a single piece of data extracted from a page.
type ExtractedItem struct {
	Type      string            `json:"type"` // e.g., "link", "email", "form", "phone", etc.
	Value     string            `json:"value"`
	SourceURL string            `json:"source_url"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// CrawlResult holds all extracted data for a single crawled page.
type CrawlResult struct {
	Page           *PageData       `json:"page"`
	ExtractedItems []ExtractedItem `json:"extracted_items"`
}

// CrawlSummary is the final aggregated output of the entire crawl.
type CrawlSummary struct {
	TargetURL   string         `json:"target_url"`
	StartedAt   time.Time      `json:"started_at"`
	FinishedAt  time.Time      `json:"finished_at"`
	Duration    time.Duration  `json:"duration"`
	TotalPages  int            `json:"total_pages"`
	TotalErrors int            `json:"total_errors"`
	TotalItems  int            `json:"total_items"`
	ItemsByType map[string]int `json:"items_by_type"`
	Results     []CrawlResult  `json:"results"`
}

// ---------- Event Types ----------

// CrawlEvent represents a real-time event emitted by the crawler.
type CrawlEvent struct {
	Type    EventType
	URL     string
	Result  *CrawlResult
	Error   error
	Stats   *CrawlStats
	Message string
}

// EventType identifies the kind of event.
type EventType int

const (
	EventPageQueued EventType = iota
	EventPageStarted
	EventPageDone
	EventPageError
	EventExtractionDone
	EventCrawlStarted
	EventCrawlFinished
	EventProgress
)

// CrawlStats holds real-time crawl statistics.
type CrawlStats struct {
	PagesQueued    int            `json:"pages_queued"`
	PagesCrawled   int            `json:"pages_crawled"`
	PagesErrored   int            `json:"pages_errored"`
	ItemsExtracted int            `json:"items_extracted"`
	ItemsByType    map[string]int `json:"items_by_type"`
	Elapsed        time.Duration  `json:"elapsed"`
	PagesPerSec    float64        `json:"pages_per_sec"`
}

// ---------- Plugin Interfaces ----------

// Fetcher defines how pages are retrieved.
type Fetcher interface {
	// Name returns a human-readable identifier for this fetcher.
	Name() string

	// Fetch retrieves the page at the given URL.
	Fetch(url string, depth int) (*PageData, error)

	// Close releases any resources held by the fetcher.
	Close() error
}

// Extractor defines how data is extracted from a fetched page.
type Extractor interface {
	// Name returns a human-readable identifier (e.g., "links", "emails").
	Name() string

	// Extract finds and returns items from the given page data.
	Extract(page *PageData) ([]ExtractedItem, error)
}

// OutputWriter defines how crawl results are persisted.
type OutputWriter interface {
	// Name returns a human-readable identifier for this writer.
	Name() string

	// WriteResult writes a single page's crawl result (called incrementally).
	WriteResult(result *CrawlResult) error

	// Finalize writes the final summary and closes resources.
	Finalize(summary *CrawlSummary) error
}
