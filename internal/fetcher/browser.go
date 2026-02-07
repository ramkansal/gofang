package fetcher

import (
	"net/http"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
	"github.com/ramkansal/web-crawler/pkg/plugin"
)

// BrowserFetcher uses Rod (headless Chrome) for JS-rendered page fetching.
type BrowserFetcher struct {
	browser     *rod.Browser
	timeout     time.Duration
	pageTimeout time.Duration
	userAgent   string
}

// BrowserFetcherConfig holds configuration for the browser fetcher.
type BrowserFetcherConfig struct {
	Timeout     time.Duration
	PageTimeout time.Duration
	UserAgent   string
	Headless    bool
}

// NewBrowserFetcher creates a new Rod-based browser fetcher.
func NewBrowserFetcher(cfg BrowserFetcherConfig) (*BrowserFetcher, error) {
	u, err := launcher.New().
		Headless(true).
		Set("no-sandbox").
		Set("disable-gpu").
		Set("disable-dev-shm-usage").
		Launch()
	if err != nil {
		return nil, err
	}

	browser := rod.New().ControlURL(u)
	if err := browser.Connect(); err != nil {
		return nil, err
	}

	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	pageTimeout := cfg.PageTimeout
	if pageTimeout == 0 {
		pageTimeout = 15 * time.Second
	}

	return &BrowserFetcher{
		browser:     browser,
		timeout:     timeout,
		pageTimeout: pageTimeout,
		userAgent:   cfg.UserAgent,
	}, nil
}

func (f *BrowserFetcher) Name() string { return "browser" }

func (f *BrowserFetcher) Fetch(targetURL string, depth int) (*plugin.PageData, error) {
	start := time.Now()

	page := &plugin.PageData{
		URL:         targetURL,
		FinalURL:    targetURL,
		FetcherUsed: "browser",
		FetchedAt:   start,
		Depth:       depth,
	}

	// Create a new page with timeout
	rodPage, err := f.browser.Page(proto.TargetCreateTarget{URL: "about:blank"})
	if err != nil {
		page.Error = err.Error()
		page.FetchDuration = time.Since(start)
		return page, err
	}
	defer rodPage.Close()

	rodPage = rodPage.Timeout(f.timeout)

	// Set user agent if configured
	if f.userAgent != "" {
		_ = rodPage.SetUserAgent(&proto.NetworkSetUserAgentOverride{
			UserAgent: f.userAgent,
		})
	}

	// Set up request interception for XHR/API capture
	var intercepted []plugin.InterceptedRequest
	router := rodPage.HijackRequests()
	defer router.Stop()

	router.MustAdd("*", func(ctx *rod.Hijack) {
		// Let the request continue normally
		ctx.ContinueRequest(&proto.FetchContinueRequest{})

		// Capture request details
		reqURL := ctx.Request.URL().String()
		method := ctx.Request.Method()
		resourceType := string(ctx.Request.Type())

		// Only capture XHR/Fetch and document requests
		if isWorthCapturing(resourceType) {
			intercepted = append(intercepted, plugin.InterceptedRequest{
				URL:          reqURL,
				Method:       method,
				ResourceType: resourceType,
				ContentType:  ctx.Request.Header("Content-Type"),
			})
		}
	})

	go router.Run()

	// Navigate to the target URL
	err = rodPage.Navigate(targetURL)
	if err != nil {
		page.Error = err.Error()
		page.FetchDuration = time.Since(start)
		return page, err
	}

	// Wait for the page to stabilize
	err = rodPage.WaitStable(f.pageTimeout)
	if err != nil {
		// Page may not fully stabilize but we can still get content
		// Don't return error, just note it
		if !strings.Contains(err.Error(), "context canceled") {
			page.Error = "page did not fully stabilize: " + err.Error()
		}
	}

	// Capture the final URL after redirects
	info, err := rodPage.Info()
	if err == nil {
		page.FinalURL = info.URL
	}

	// Get response status from navigation (best effort)
	page.StatusCode = 200 // Default assumption for successful navigation
	page.Headers = make(http.Header)

	// Get the rendered HTML
	html, err := rodPage.HTML()
	if err == nil {
		page.RenderedHTML = html
		// Also set RawHTML to rendered version for consistent extraction
		page.RawHTML = html
	}

	// Get content type from the page
	page.ContentType = "text/html"

	// Store intercepted requests
	page.InterceptedReqs = intercepted

	page.FetchDuration = time.Since(start)
	return page, nil
}

func (f *BrowserFetcher) Close() error {
	if f.browser != nil {
		return f.browser.Close()
	}
	return nil
}

// isWorthCapturing determines if a request type is worth recording as an API endpoint.
func isWorthCapturing(resourceType string) bool {
	switch strings.ToLower(resourceType) {
	case "xhr", "fetch", "websocket", "eventsource", "other":
		return true
	default:
		return false
	}
}
