package crawler

import (
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/ramkansal/gofang/internal/extractor"
	"github.com/ramkansal/gofang/internal/fetcher"
	"github.com/ramkansal/gofang/internal/output"
	"github.com/ramkansal/gofang/pkg/plugin"
)

// Crawler is the core engine that orchestrates fetching, extracting, and outputting.
type Crawler struct {
	config     *CrawlConfig
	httpFetch  plugin.Fetcher
	browFetch  plugin.Fetcher
	extractors *extractor.Registry
	writer     plugin.OutputWriter
	events     chan plugin.CrawlEvent

	// URL frontier
	visited map[string]bool
	queue   []queueItem
	queueMu sync.Mutex
	visitMu sync.Mutex

	// Stats
	stats     plugin.CrawlStats
	statsMu   sync.Mutex
	startTime time.Time

	// Control
	done    chan struct{}
	stopped bool
	stopMu  sync.Mutex
}

type queueItem struct {
	url   string
	depth int
}

// New creates a new Crawler with the given configuration.
func New(config *CrawlConfig) *Crawler {
	return &Crawler{
		config:  config,
		events:  make(chan plugin.CrawlEvent, 1000),
		visited: make(map[string]bool),
		done:    make(chan struct{}),
		stats: plugin.CrawlStats{
			ItemsByType: make(map[string]int),
		},
	}
}

// Events returns the event channel for the TUI or other consumers.
func (c *Crawler) Events() <-chan plugin.CrawlEvent {
	return c.events
}

// Init initializes all components (fetchers, extractors, output).
func (c *Crawler) Init() error {
	// Parse target domain for Colly's allowed domains
	parsedURL, err := url.Parse(c.config.TargetURL)
	if err != nil {
		return fmt.Errorf("invalid target URL: %w", err)
	}
	domain := parsedURL.Hostname()

	// Initialize HTTP fetcher
	c.httpFetch = fetcher.NewHTTPFetcher(fetcher.HTTPFetcherConfig{
		MaxDepth:         c.config.MaxDepth,
		Parallelism:      c.config.Parallelism,
		RateLimit:        c.config.RateLimit,
		UserAgent:        c.config.UserAgent,
		AllowExternal:    c.config.AllowExternal,
		RespectRobots:    c.config.RespectRobots,
		AllowedDomain:    domain,
		Timeout:          c.config.Timeout,
		Retry:            c.config.Retry,
		MaxResponseSize:  c.config.MaxResponseSize,
		Proxy:            c.config.Proxy,
		CustomHeaders:    c.config.CustomHeaders,
		DisableRedirects: c.config.DisableRedirects,
	})

	// Initialize browser fetcher if needed
	if c.config.FetcherMode == FetcherBrowser || c.config.FetcherMode == FetcherAuto {
		bf, err := fetcher.NewBrowserFetcher(fetcher.BrowserFetcherConfig{
			Timeout:     c.config.BrowserTimeout,
			PageTimeout: c.config.PageTimeout,
			UserAgent:   c.config.UserAgent,
			Headless:    true,
		})
		if err != nil {
			c.emit(plugin.CrawlEvent{
				Type:    plugin.EventPageError,
				Message: fmt.Sprintf("Browser fetcher unavailable: %v (falling back to HTTP)", err),
			})
			c.config.FetcherMode = FetcherHTTP
		} else {
			c.browFetch = bf
		}
	}

	// Initialize extractors
	c.extractors = extractor.NewRegistry()

	// Initialize output writer only if saving is requested
	if c.config.SaveOutput {
		c.writer = output.NewTextWriter(c.config.OutputPath)
	}

	return nil
}

// Run starts the crawl. It blocks until the crawl completes.
func (c *Crawler) Run() error {
	c.startTime = time.Now()

	c.emit(plugin.CrawlEvent{
		Type:    plugin.EventCrawlStarted,
		URL:     c.config.TargetURL,
		Message: fmt.Sprintf("Starting crawl of %s", c.config.TargetURL),
	})

	// Seed the queue
	c.enqueue(c.config.TargetURL, 0)

	// Worker pool
	var wg sync.WaitGroup
	sem := make(chan struct{}, c.config.Parallelism)

	for {
		item, ok := c.dequeue()
		if !ok {
			// Wait for in-flight workers before checking again
			break
		}

		if c.isStopped() {
			break
		}

		sem <- struct{}{}
		wg.Add(1)
		go func(item queueItem) {
			defer wg.Done()
			defer func() { <-sem }()

			c.processURL(item)
		}(item)
	}

	wg.Wait()

	// Finalize
	if c.writer != nil {
		summary := c.buildSummary()
		if err := c.writer.Finalize(summary); err != nil {
			c.emit(plugin.CrawlEvent{
				Type:    plugin.EventPageError,
				Error:   err,
				Message: "Failed to write output: " + err.Error(),
			})
		}
	}

	c.emit(plugin.CrawlEvent{
		Type:    plugin.EventCrawlFinished,
		Stats:   c.getStats(),
		Message: fmt.Sprintf("Crawl complete. %d pages, %d items extracted.", c.stats.PagesCrawled, c.stats.ItemsExtracted),
	})

	close(c.events)
	return nil
}

// Stop signals the crawler to stop gracefully.
func (c *Crawler) Stop() {
	c.stopMu.Lock()
	defer c.stopMu.Unlock()
	c.stopped = true
}

func (c *Crawler) isStopped() bool {
	c.stopMu.Lock()
	defer c.stopMu.Unlock()
	return c.stopped
}

// processURL fetches and extracts data from a single URL.
func (c *Crawler) processURL(item queueItem) {
	c.emit(plugin.CrawlEvent{
		Type: plugin.EventPageStarted,
		URL:  item.url,
	})

	// Choose fetcher
	fetchr := c.chooseFetcher(item.url)

	// Fetch the page
	pageData, err := fetchr.Fetch(item.url, item.depth)
	if err != nil {
		c.statsMu.Lock()
		c.stats.PagesErrored++
		c.statsMu.Unlock()

		c.emit(plugin.CrawlEvent{
			Type:    plugin.EventPageError,
			URL:     item.url,
			Error:   err,
			Message: fmt.Sprintf("Error fetching %s: %v", item.url, err),
		})
		return
	}

	// Run all extractors
	items, _ := c.extractors.ExtractAll(pageData)

	result := &plugin.CrawlResult{
		Page:           pageData,
		ExtractedItems: items,
	}

	// Write result to output (if saving enabled)
	if c.writer != nil {
		_ = c.writer.WriteResult(result)
	}

	// Update stats
	c.statsMu.Lock()
	c.stats.PagesCrawled++
	c.stats.ItemsExtracted += len(items)
	for _, item := range items {
		c.stats.ItemsByType[item.Type]++
	}
	elapsed := time.Since(c.startTime)
	c.stats.Elapsed = elapsed
	if elapsed.Seconds() > 0 {
		c.stats.PagesPerSec = float64(c.stats.PagesCrawled) / elapsed.Seconds()
	}
	c.statsMu.Unlock()

	c.emit(plugin.CrawlEvent{
		Type:   plugin.EventPageDone,
		URL:    item.url,
		Result: result,
		Stats:  c.getStats(),
	})

	// Extract links and enqueue them
	if item.depth < c.config.MaxDepth {
		for _, extracted := range items {
			if extracted.Type == "link" {
				linkType := extracted.Metadata["link_type"]
				if linkType == "internal" || c.config.AllowExternal {
					c.enqueue(extracted.Value, item.depth+1)
				}
			}
		}
	}
}

// chooseFetcher decides whether to use HTTP or browser fetcher.
func (c *Crawler) chooseFetcher(targetURL string) plugin.Fetcher {
	switch c.config.FetcherMode {
	case FetcherHTTP:
		return c.httpFetch
	case FetcherBrowser:
		if c.browFetch != nil {
			return c.browFetch
		}
		return c.httpFetch
	case FetcherAuto:
		// Use browser for the first page (root), then HTTP for discovered links
		// This is a simple heuristic — could be improved with content-type detection
		if c.browFetch != nil {
			c.statsMu.Lock()
			crawled := c.stats.PagesCrawled
			c.statsMu.Unlock()
			if crawled == 0 {
				return c.browFetch // Use browser for first page to capture XHR
			}
		}
		return c.httpFetch
	default:
		return c.httpFetch
	}
}

// enqueue adds a URL to the crawl queue if not already visited.
func (c *Crawler) enqueue(rawURL string, depth int) {
	// Normalize the URL
	normalized := normalizeURL(rawURL)
	if normalized == "" {
		return
	}

	c.visitMu.Lock()
	if c.visited[normalized] {
		c.visitMu.Unlock()
		return
	}

	// Check max pages limit
	if len(c.visited) >= c.config.MaxPages {
		c.visitMu.Unlock()
		return
	}

	c.visited[normalized] = true
	c.visitMu.Unlock()

	c.queueMu.Lock()
	c.queue = append(c.queue, queueItem{url: normalized, depth: depth})
	c.queueMu.Unlock()

	c.statsMu.Lock()
	c.stats.PagesQueued++
	c.statsMu.Unlock()

	c.emit(plugin.CrawlEvent{
		Type: plugin.EventPageQueued,
		URL:  normalized,
	})
}

// dequeue pops the next URL from the queue.
func (c *Crawler) dequeue() (queueItem, bool) {
	c.queueMu.Lock()
	defer c.queueMu.Unlock()

	if len(c.queue) == 0 {
		return queueItem{}, false
	}

	item := c.queue[0]
	c.queue = c.queue[1:]
	return item, true
}

// emit sends an event to the event channel (non-blocking).
func (c *Crawler) emit(event plugin.CrawlEvent) {
	select {
	case c.events <- event:
	default:
		// Drop event if channel is full — TUI is too slow, we don't block the crawler
	}
}

// getStats returns a copy of the current stats.
func (c *Crawler) getStats() *plugin.CrawlStats {
	c.statsMu.Lock()
	defer c.statsMu.Unlock()

	statsCopy := c.stats
	byType := make(map[string]int)
	for k, v := range c.stats.ItemsByType {
		byType[k] = v
	}
	statsCopy.ItemsByType = byType
	return &statsCopy
}

// buildSummary creates the final CrawlSummary.
func (c *Crawler) buildSummary() *plugin.CrawlSummary {
	stats := c.getStats()
	return &plugin.CrawlSummary{
		TargetURL:   c.config.TargetURL,
		StartedAt:   c.startTime,
		FinishedAt:  time.Now(),
		Duration:    time.Since(c.startTime),
		TotalPages:  stats.PagesCrawled,
		TotalErrors: stats.PagesErrored,
		TotalItems:  stats.ItemsExtracted,
		ItemsByType: stats.ItemsByType,
	}
}

// Close releases all resources.
func (c *Crawler) Close() error {
	if c.httpFetch != nil {
		c.httpFetch.Close()
	}
	if c.browFetch != nil {
		c.browFetch.Close()
	}
	return nil
}

// normalizeURL cleans up a URL for deduplication.
func normalizeURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}

	// Only crawl http/https
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return ""
	}

	// Remove fragment
	parsed.Fragment = ""

	// Remove trailing slash for consistency
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	if parsed.Path == "" {
		parsed.Path = "/"
	}

	return parsed.String()
}
