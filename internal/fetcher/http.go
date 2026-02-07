package fetcher

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gocolly/colly/v2"
	"github.com/ramkansal/web-crawler/pkg/plugin"
)

// HTTPFetcher uses Colly for fast, efficient HTTP-only page fetching.
type HTTPFetcher struct {
	collector *colly.Collector
	userAgent string
	mu        sync.Mutex
	results   map[string]*plugin.PageData
}

// HTTPFetcherConfig holds configuration for the HTTP fetcher.
type HTTPFetcherConfig struct {
	MaxDepth         int
	Parallelism      int
	RateLimit        time.Duration
	UserAgent        string
	AllowExternal    bool
	RespectRobots    bool
	AllowedDomain    string
	Timeout          time.Duration
	Retry            int
	MaxResponseSize  int
	Proxy            string
	CustomHeaders    []string
	DisableRedirects bool
}

// NewHTTPFetcher creates a new Colly-based HTTP fetcher.
func NewHTTPFetcher(cfg HTTPFetcherConfig) *HTTPFetcher {
	opts := []colly.CollectorOption{
		colly.MaxDepth(cfg.MaxDepth),
		colly.Async(false), // We control concurrency externally
	}

	if !cfg.AllowExternal && cfg.AllowedDomain != "" {
		opts = append(opts, colly.AllowedDomains(
			cfg.AllowedDomain,
			"www."+cfg.AllowedDomain,
		))
	}

	c := colly.NewCollector(opts...)

	if cfg.UserAgent != "" {
		c.UserAgent = cfg.UserAgent
	}

	// Rate limiting
	if cfg.RateLimit > 0 {
		_ = c.Limit(&colly.LimitRule{
			DomainGlob:  "*",
			Parallelism: cfg.Parallelism,
			Delay:       cfg.RateLimit,
		})
	}

	if !cfg.RespectRobots {
		c.IgnoreRobotsTxt = true
	}

	// Set request timeout
	if cfg.Timeout > 0 {
		c.SetRequestTimeout(cfg.Timeout)
	}

	// Set proxy
	if cfg.Proxy != "" {
		c.SetProxy(cfg.Proxy)
	}

	// Set max response size
	if cfg.MaxResponseSize > 0 {
		c.MaxBodySize = cfg.MaxResponseSize
	}

	// Disable redirects
	if cfg.DisableRedirects {
		c.SetRedirectHandler(func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		})
	}

	// Set custom headers
	if len(cfg.CustomHeaders) > 0 {
		c.OnRequest(func(r *colly.Request) {
			for _, h := range cfg.CustomHeaders {
				parts := strings.SplitN(h, ":", 2)
				if len(parts) == 2 {
					r.Headers.Set(strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]))
				}
			}
		})
	}

	f := &HTTPFetcher{
		collector: c,
		userAgent: cfg.UserAgent,
		results:   make(map[string]*plugin.PageData),
	}

	return f
}

func (f *HTTPFetcher) Name() string { return "http" }

func (f *HTTPFetcher) Fetch(targetURL string, depth int) (*plugin.PageData, error) {
	start := time.Now()

	page := &plugin.PageData{
		URL:         targetURL,
		FinalURL:    targetURL,
		FetcherUsed: "http",
		FetchedAt:   start,
		Depth:       depth,
	}

	// Clone the collector for this individual fetch so we get clean state
	c := f.collector.Clone()

	var fetchErr error

	c.OnResponse(func(r *colly.Response) {
		page.StatusCode = r.StatusCode
		page.RawHTML = string(r.Body)
		page.ResponseSize = len(r.Body)
		page.FinalURL = r.Request.URL.String()
		page.ContentType = r.Headers.Get("Content-Type")

		// Capture response headers
		page.Headers = make(http.Header)
		for key, values := range *r.Headers {
			for _, v := range values {
				page.Headers.Add(key, v)
			}
		}
	})

	c.OnError(func(r *colly.Response, err error) {
		fetchErr = err
		if r != nil {
			page.StatusCode = r.StatusCode
			page.FinalURL = r.Request.URL.String()
		}
		page.Error = err.Error()
	})

	// Perform the request
	err := c.Visit(targetURL)
	if err != nil {
		// Check if it's "already visited" — not really an error for us
		if !strings.Contains(err.Error(), "already visited") {
			page.Error = err.Error()
			page.FetchDuration = time.Since(start)
			return page, err
		}
	}

	c.Wait()

	page.FetchDuration = time.Since(start)

	if fetchErr != nil {
		return page, fetchErr
	}

	return page, nil
}

func (f *HTTPFetcher) Close() error {
	return nil
}
