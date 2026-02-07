package crawler

import "time"

// CrawlConfig holds all configuration for a crawl session.
type CrawlConfig struct {
	// Target
	TargetURL string

	// Crawl control
	MaxDepth          int
	MaxPages          int
	Parallelism       int
	RateLimit         time.Duration
	CrawlDuration     time.Duration
	Strategy          Strategy
	IgnoreQueryParams bool

	// Request options
	UserAgent        string
	Timeout          time.Duration
	Retry            int
	MaxResponseSize  int
	Proxy            string
	CustomHeaders    []string
	CustomResolvers  []string
	DisableRedirects bool
	TLSImpersonate   bool

	// Feature flags
	AllowExternal  bool
	RespectRobots  bool
	JSCrawl        bool
	JSLuice        bool
	KnownFiles     string
	AutoFormFill   bool
	FormExtraction bool
	TechDetect     bool
	FetcherMode    FetcherMode

	// Output
	OutputPath string
	SaveOutput bool
	Silent     bool
	Verbose    bool
	NoColor    bool

	// Config files
	ConfigFile  string
	FormConfig  string
	FieldConfig string

	// Internal
	BrowserTimeout time.Duration
	PageTimeout    time.Duration
}

// FetcherMode controls which fetcher to use.
type FetcherMode string

const (
	FetcherHTTP    FetcherMode = "http"
	FetcherBrowser FetcherMode = "browser"
	FetcherAuto    FetcherMode = "auto"
)

// Strategy defines the crawl order.
type Strategy string

const (
	StrategyDepthFirst   Strategy = "depth-first"
	StrategyBreadthFirst Strategy = "breadth-first"
)

// DefaultConfig returns a sensible default configuration.
func DefaultConfig() *CrawlConfig {
	return &CrawlConfig{
		MaxDepth:        3,
		MaxPages:        500,
		Parallelism:     5,
		RateLimit:       200 * time.Millisecond,
		Strategy:        StrategyDepthFirst,
		UserAgent:       "WebCrawler/1.0",
		Timeout:         10 * time.Second,
		Retry:           1,
		MaxResponseSize: 4194304, // 4MB
		AllowExternal:   false,
		RespectRobots:   true,
		FetcherMode:     FetcherHTTP,
		SaveOutput:      false,
		OutputPath:      "crawl_results.json",
		BrowserTimeout:  30 * time.Second,
		PageTimeout:     15 * time.Second,
	}
}
