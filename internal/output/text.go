package output

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/ramkansal/gofang/pkg/plugin"
)

// TextWriter writes crawl results to a plain text file,
// mirroring the terminal output (without ANSI color codes).
type TextWriter struct {
	path  string
	lines []string
	mu    sync.Mutex
}

// NewTextWriter creates a new plain-text output writer.
func NewTextWriter(path string) *TextWriter {
	return &TextWriter{path: path}
}

func (w *TextWriter) Name() string { return "text" }

func (w *TextWriter) WriteResult(result *plugin.CrawlResult) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	p := result.Page
	items := result.ExtractedItems

	status := fmt.Sprintf("%d", p.StatusCode)
	counts := plainItemCounts(items)
	dur := fmtDur(p.FetchDuration)

	w.lines = append(w.lines, fmt.Sprintf("  [%s] %s (%s) %s", status, p.URL, dur, counts))

	for _, item := range items {
		w.lines = append(w.lines, fmt.Sprintf("      +-- %s: %s", item.Type, item.Value))
	}

	return nil
}

func (w *TextWriter) Finalize(summary *plugin.CrawlSummary) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	var b strings.Builder

	// Banner
	b.WriteString("\n  GOFANG v1.0.0\n")
	b.WriteString("  All-in-one web crawler with extraction superpowers\n")
	b.WriteString("  " + strings.Repeat("-", 58) + "\n\n")

	// Target info
	b.WriteString(fmt.Sprintf("  Target: %s\n", summary.TargetURL))
	b.WriteString(fmt.Sprintf("  Started: %s\n\n", summary.StartedAt.Format(time.RFC1123)))

	// Page results
	for _, line := range w.lines {
		b.WriteString(line + "\n")
	}

	// Summary
	b.WriteString("\n  " + strings.Repeat("-", 50) + "\n")
	b.WriteString("  Crawl complete\n")

	pages := 0
	errors := 0
	if summary.Results != nil {
		pages = len(summary.Results)
	}
	b.WriteString(fmt.Sprintf("    Pages:  %d crawled, %d errors\n", pages, errors))
	b.WriteString(fmt.Sprintf("    Items:  %d extracted in %s\n", summary.TotalItems, fmtDur(summary.Duration)))

	if len(summary.ItemsByType) > 0 {
		b.WriteString("    Types:  ")
		first := true
		for _, t := range []string{"link", "form", "email", "phone", "social", "metadata", "asset", "api_endpoint"} {
			if count, ok := summary.ItemsByType[t]; ok && count > 0 {
				if !first {
					b.WriteString(", ")
				}
				b.WriteString(fmt.Sprintf("%s:%d", t, count))
				first = false
			}
		}
		b.WriteString("\n")
	}
	b.WriteString("\n")

	return os.WriteFile(w.path, []byte(b.String()), 0644)
}

// ---------- helpers ----------

func plainItemCounts(items []plugin.ExtractedItem) string {
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
	return "[" + strings.Join(parts, " ") + "]"
}

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
