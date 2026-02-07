package main

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/ramkansal/web-crawler/internal/crawler"
	"github.com/ramkansal/web-crawler/pkg/plugin"
)

var version = "1.0.0"

// flags holds all parsed CLI options.
type flags struct {
	// Target
	url string

	// Crawl
	depth             int
	maxPages          int
	parallel          int
	rateLimit         time.Duration
	crawlDuration     time.Duration
	strategy          string
	ignoreQueryParams bool

	// Request
	userAgent        string
	timeout          int
	retry            int
	maxResponseSize  int
	proxy            string
	headers          []string
	resolvers        []string
	disableRedirects bool
	tlsImpersonate   bool

	// Features
	external     bool
	robots       bool
	jsCrawl      bool
	jsLuice      bool
	knownFiles   string
	autoFormFill bool
	formExtract  bool
	techDetect   bool
	fetcher      string

	// Output
	output  string
	silent  bool
	verbose bool
	noColor bool

	// Config files
	configFile  string
	formConfig  string
	fieldConfig string

	// Meta
	showHelp    bool
	showVersion bool
}

func main() {
	f := parseFlags()

	if f.showVersion {
		fmt.Printf("gofang v%s\n", version)
		os.Exit(0)
	}

	if f.showHelp || f.url == "" {
		printUsage()
		if f.url == "" && !f.showHelp {
			os.Exit(1)
		}
		os.Exit(0)
	}

	// Ensure URL has a scheme
	if !strings.HasPrefix(f.url, "http://") && !strings.HasPrefix(f.url, "https://") {
		f.url = "https://" + f.url
	}

	cfg := buildConfig(f)

	c := crawler.New(cfg)
	if err := c.Init(); err != nil {
		fatal("initialization failed: %v", err)
	}
	defer c.Close()

	// Handle Ctrl+C
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sig
		fmt.Fprintf(os.Stderr, "\n\n%s Interrupt received, stopping...\n", clr("yellow", "!"))
		c.Stop()
	}()

	run(c, cfg)
}

func run(c *crawler.Crawler, cfg *crawler.CrawlConfig) {
	if !cfg.Silent {
		printBanner()
		fmt.Printf("\n  %s %s\n", clr("cyan", "Target:"), cfg.TargetURL)
		fmt.Printf("  %s %d  %s %d  %s %s  %s %s\n\n",
			clr("dim", "Depth:"), cfg.MaxDepth,
			clr("dim", "Threads:"), cfg.Parallelism,
			clr("dim", "Fetcher:"), string(cfg.FetcherMode),
			clr("dim", "Strategy:"), string(cfg.Strategy),
		)
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		for event := range c.Events() {
			if cfg.Silent {
				continue
			}
			handleEvent(event, cfg)
		}
	}()

	if err := c.Run(); err != nil {
		fatal("crawl error: %v", err)
	}

	<-done

	if !cfg.Silent {
		time.Sleep(50 * time.Millisecond)
	}
}

func handleEvent(event plugin.CrawlEvent, cfg *crawler.CrawlConfig) {
	switch event.Type {
	case plugin.EventPageDone:
		if event.Result == nil {
			return
		}
		p := event.Result.Page
		items := event.Result.ExtractedItems

		status := fmt.Sprintf("%d", p.StatusCode)
		switch {
		case p.StatusCode >= 200 && p.StatusCode < 300:
			status = clr("green", status)
		case p.StatusCode >= 300 && p.StatusCode < 400:
			status = clr("yellow", status)
		case p.StatusCode >= 400:
			status = clr("red", status)
		}

		counts := itemCountStr(items)
		dur := fmtDur(p.FetchDuration)

		fmt.Printf("  %s [%s] %s %s %s\n",
			clr("green", "●"),
			status,
			p.URL,
			clr("dim", "("+dur+")"),
			counts,
		)

		for _, item := range items {
			fmt.Printf("      %s %s\n",
				clr("dim", "├─ "+item.Type+":"),
				item.Value,
			)
		}

	case plugin.EventPageError:
		fmt.Printf("  %s %s\n", clr("red", "✗"), event.Message)

	case plugin.EventCrawlStarted:
		// already printed in run()

	case plugin.EventCrawlFinished:
		if event.Stats == nil {
			return
		}
		s := event.Stats
		fmt.Println()
		fmt.Printf("  %s\n", strings.Repeat("─", 50))
		fmt.Printf("  %s Crawl complete\n", clr("green", "✓"))
		fmt.Printf("    Pages:  %s crawled, %s errors\n",
			clr("cyan", fmt.Sprintf("%d", s.PagesCrawled)),
			clr("red", fmt.Sprintf("%d", s.PagesErrored)),
		)
		fmt.Printf("    Items:  %s extracted in %s (%.1f pages/sec)\n",
			clr("yellow", fmt.Sprintf("%d", s.ItemsExtracted)),
			fmtDur(s.Elapsed),
			s.PagesPerSec,
		)
		if len(s.ItemsByType) > 0 {
			fmt.Printf("    Types:  ")
			first := true
			for _, t := range []string{"link", "form", "email", "phone", "social", "metadata", "asset", "api_endpoint"} {
				if count, ok := s.ItemsByType[t]; ok && count > 0 {
					if !first {
						fmt.Printf(", ")
					}
					fmt.Printf("%s:%s", clr("dim", t), clr("cyan", fmt.Sprintf("%d", count)))
					first = false
				}
			}
			fmt.Println()
		}
		if cfg.SaveOutput {
			fmt.Printf("    Output: %s\n", clr("green", cfg.OutputPath))
		}
		fmt.Println()
	}
}

func itemCountStr(items []plugin.ExtractedItem) string {
	if len(items) == 0 {
		return ""
	}
	counts := make(map[string]int)
	for _, item := range items {
		counts[item.Type]++
	}
	var parts []string
	for _, t := range []string{"link", "form", "email", "phone", "social", "metadata", "asset", "api_endpoint"} {
		if c, ok := counts[t]; ok && c > 0 {
			short := t
			if t == "api_endpoint" {
				short = "api"
			}
			if t == "metadata" {
				short = "meta"
			}
			parts = append(parts, fmt.Sprintf("%s:%d", short, c))
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return clr("dim", "["+strings.Join(parts, " ")+"]")
}

// ---------- Flag parsing ----------

func parseFlags() *flags {
	f := &flags{
		depth:           3,
		maxPages:        500,
		parallel:        5,
		timeout:         10,
		retry:           1,
		maxResponseSize: 4194304,
		strategy:        "depth-first",
		fetcher:         "http",
		robots:          true,
	}

	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		arg := args[i]
		next := func() string {
			if i+1 < len(args) {
				i++
				return args[i]
			}
			fatal("flag %s requires an argument", arg)
			return ""
		}
		nextInt := func() int {
			v := next()
			var n int
			fmt.Sscanf(v, "%d", &n)
			return n
		}

		switch arg {
		// Target
		case "-u", "--url":
			f.url = next()

		// Crawl
		case "-d", "--depth":
			f.depth = nextInt()
		case "-mp", "--max-pages":
			f.maxPages = nextInt()
		case "-c", "--concurrency":
			f.parallel = nextInt()
		case "-rl", "--rate-limit":
			v := next()
			d, err := time.ParseDuration(v)
			if err != nil {
				d = 200 * time.Millisecond
			}
			f.rateLimit = d
		case "-ct", "--crawl-duration":
			v := next()
			d, err := time.ParseDuration(v)
			if err != nil {
				d = 0
			}
			f.crawlDuration = d
		case "-s", "--strategy":
			f.strategy = next()
		case "-iqp", "--ignore-query-params":
			f.ignoreQueryParams = true

		// Request
		case "-ua", "--user-agent":
			f.userAgent = next()
		case "-t", "--timeout":
			f.timeout = nextInt()
		case "-rt", "--retry":
			f.retry = nextInt()
		case "-mrs", "--max-response-size":
			f.maxResponseSize = nextInt()
		case "-px", "--proxy":
			f.proxy = next()
		case "-H", "--header":
			f.headers = append(f.headers, next())
		case "-r", "--resolver":
			v := next()
			for _, r := range strings.Split(v, ",") {
				if r = strings.TrimSpace(r); r != "" {
					f.resolvers = append(f.resolvers, r)
				}
			}
		case "-dr", "--disable-redirects":
			f.disableRedirects = true
		case "-tlsi", "--tls-impersonate":
			f.tlsImpersonate = true

		// Features
		case "-e", "--external":
			f.external = true
		case "--no-robots":
			f.robots = false
		case "-jc", "--js-crawl":
			f.jsCrawl = true
		case "-jsl", "--js-luice":
			f.jsLuice = true
		case "-kf", "--known-files":
			f.knownFiles = next()
		case "-aff", "--auto-form-fill":
			f.autoFormFill = true
		case "-fx", "--form-extraction":
			f.formExtract = true
		case "-td", "--tech-detect":
			f.techDetect = true
		case "-f", "--fetcher":
			f.fetcher = next()

		// Output
		case "-o", "--output":
			f.output = next()
		case "-si", "--silent":
			f.silent = true
		case "-v", "--verbose":
			f.verbose = true
		case "-nc", "--no-color":
			f.noColor = true

		// Config files
		case "--config":
			f.configFile = next()
		case "-fc", "--form-config":
			f.formConfig = next()
		case "-flc", "--field-config":
			f.fieldConfig = next()

		// Meta
		case "-h", "--help":
			f.showHelp = true
		case "-V", "--version":
			f.showVersion = true

		default:
			// Treat bare arg as URL if no URL yet
			if !strings.HasPrefix(arg, "-") && f.url == "" {
				f.url = arg
			} else {
				fmt.Fprintf(os.Stderr, "Unknown flag: %s (use --help for usage)\n", arg)
				os.Exit(1)
			}
		}
	}
	return f
}

func buildConfig(f *flags) *crawler.CrawlConfig {
	cfg := crawler.DefaultConfig()
	cfg.TargetURL = f.url
	cfg.MaxDepth = f.depth
	cfg.MaxPages = f.maxPages
	cfg.Parallelism = f.parallel
	cfg.Timeout = time.Duration(f.timeout) * time.Second
	cfg.Retry = f.retry
	cfg.MaxResponseSize = f.maxResponseSize
	cfg.Strategy = crawler.Strategy(f.strategy)
	cfg.IgnoreQueryParams = f.ignoreQueryParams
	cfg.AllowExternal = f.external
	cfg.RespectRobots = f.robots
	cfg.JSCrawl = f.jsCrawl
	cfg.JSLuice = f.jsLuice
	cfg.KnownFiles = f.knownFiles
	cfg.AutoFormFill = f.autoFormFill
	cfg.FormExtraction = f.formExtract
	cfg.TechDetect = f.techDetect
	cfg.DisableRedirects = f.disableRedirects
	cfg.TLSImpersonate = f.tlsImpersonate
	cfg.Proxy = f.proxy
	cfg.CustomHeaders = f.headers
	cfg.CustomResolvers = f.resolvers
	cfg.Silent = f.silent
	cfg.Verbose = f.verbose
	cfg.NoColor = f.noColor
	cfg.ConfigFile = f.configFile
	cfg.FormConfig = f.formConfig
	cfg.FieldConfig = f.fieldConfig

	if f.rateLimit > 0 {
		cfg.RateLimit = f.rateLimit
	}
	if f.crawlDuration > 0 {
		cfg.CrawlDuration = f.crawlDuration
	}
	if f.userAgent != "" {
		cfg.UserAgent = f.userAgent
	}

	switch strings.ToLower(f.fetcher) {
	case "http":
		cfg.FetcherMode = crawler.FetcherHTTP
	case "browser":
		cfg.FetcherMode = crawler.FetcherBrowser
	default:
		cfg.FetcherMode = crawler.FetcherAuto
	}

	if f.output != "" {
		cfg.SaveOutput = true
		cfg.OutputPath = f.output
	}

	return cfg
}

// ---------- Help / banner ----------

func printUsage() {
	printBanner()
	fmt.Print(`
USAGE:
  gofang [flags] <url>
  gofang -u https://example.com
  gofang -u https://example.com -d 5 -c 10 -f browser

TARGET:
  -u,    --url <string>              target URL to crawl

CRAWL:
  -d,    --depth <int>               maximum depth to crawl (default 3)
  -mp,   --max-pages <int>           maximum number of pages to crawl (default 500)
  -c,    --concurrency <int>         number of concurrent crawl workers (default 5)
  -rl,   --rate-limit <duration>     delay between requests (default 200ms)
  -ct,   --crawl-duration <duration> maximum duration to crawl the target for (e.g. 30s, 5m, 1h)
  -s,    --strategy <string>         visit strategy: depth-first, breadth-first (default "depth-first")
  -iqp,  --ignore-query-params       ignore crawling same path with different query-param values

REQUEST:
  -ua,   --user-agent <string>       custom user-agent string
  -t,    --timeout <int>             time to wait for request in seconds (default 10)
  -rt,   --retry <int>               number of times to retry a failed request (default 1)
  -mrs,  --max-response-size <int>   maximum response size to read in bytes (default 4194304)
  -px,   --proxy <string>            http/socks5 proxy to use
  -H,    --header <string>           custom header in "Key: Value" format (can be used multiple times)
  -r,    --resolver <string>         list of custom resolvers, comma separated
  -dr,   --disable-redirects         disable following redirects
  -tlsi, --tls-impersonate           enable experimental client hello (ja3) tls randomization

FEATURES:
  -e,    --external                  follow and extract external links
  -jc,   --js-crawl                  enable endpoint parsing / crawling in javascript files
  -jsl,  --js-luice                  enable jsluice parsing in javascript files (memory intensive)
  -kf,   --known-files <string>      crawl known files: all, robotstxt, sitemapxml (min depth 3)
  -aff,  --auto-form-fill            enable automatic form filling (experimental)
  -fx,   --form-extraction           extract form, input, textarea & select elements in output
  -td,   --tech-detect               enable technology detection
  -f,    --fetcher <string>          fetcher mode: http, browser, auto (default "http")
         --no-robots                 ignore robots.txt restrictions

OUTPUT:
  -o,    --output <string>           save terminal output to file (disabled by default)
  -si,   --silent                    suppress all output except errors
  -v,    --verbose                   show detailed extraction results per page
  -nc,   --no-color                  disable colored output

CONFIG:
         --config <string>           path to crawler configuration file
  -fc,   --form-config <string>      path to custom form configuration file
  -flc,  --field-config <string>     path to custom field configuration file

META:
  -h,    --help                      show this help message
  -V,    --version                   show version

`)
}

func printBanner() {
	fang := `
   ██████╗  ██████╗ ███████╗ █████╗ ███╗   ██╗ ██████╗
  ██╔════╝ ██╔═══██╗██╔════╝██╔══██╗████╗  ██║██╔════╝
  ██║  ███╗██║   ██║█████╗  ███████║██╔██╗ ██║██║  ███╗
  ██║   ██║██║   ██║██╔══╝  ██╔══██║██║╚██╗██║██║   ██║
  ╚██████╔╝╚██████╔╝██║     ██║  ██║██║ ╚████║╚██████╔╝
   ╚═════╝  ╚═════╝ ╚═╝     ╚═╝  ╚═╝╚═╝  ╚═══╝ ╚═════╝`
	fmt.Println(clr("cyan", fang))
	fmt.Printf("  %s  %s\n", clr("dim", "All-in-one web crawler with extraction superpowers"), clr("dim", "v"+version))
	fmt.Printf("  %s\n", clr("dim", strings.Repeat("─", 58)))
}

// ---------- Utilities ----------

func fmtDur(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	m := int(d.Minutes())
	s := int(d.Seconds()) % 60
	return fmt.Sprintf("%dm%ds", m, s)
}

func clr(color, text string) string {
	codes := map[string]string{
		"red":    "\033[31m",
		"green":  "\033[32m",
		"yellow": "\033[33m",
		"cyan":   "\033[36m",
		"dim":    "\033[2m",
		"bold":   "\033[1m",
		"reset":  "\033[0m",
	}
	c, ok := codes[color]
	if !ok {
		return text
	}
	return c + text + codes["reset"]
}

func fatal(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "\n  %s %s\n\n", clr("red", "ERROR:"), fmt.Sprintf(format, args...))
	os.Exit(1)
}
