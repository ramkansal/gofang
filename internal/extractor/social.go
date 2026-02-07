package extractor

import (
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/ramkansal/web-crawler/pkg/plugin"
)

// SocialExtractor extracts social media links from a page.
type SocialExtractor struct {
	platforms map[string][]string // platform name → URL patterns
}

func NewSocialExtractor() *SocialExtractor {
	return &SocialExtractor{
		platforms: map[string][]string{
			"twitter":   {"twitter.com/", "x.com/"},
			"facebook":  {"facebook.com/", "fb.com/", "fb.me/"},
			"instagram": {"instagram.com/"},
			"linkedin":  {"linkedin.com/"},
			"github":    {"github.com/"},
			"youtube":   {"youtube.com/", "youtu.be/"},
			"tiktok":    {"tiktok.com/"},
			"reddit":    {"reddit.com/"},
			"pinterest": {"pinterest.com/"},
			"discord":   {"discord.gg/", "discord.com/"},
			"telegram":  {"t.me/", "telegram.me/"},
			"mastodon":  {"mastodon.social/"},
			"threads":   {"threads.net/"},
			"bluesky":   {"bsky.app/"},
		},
	}
}

func (e *SocialExtractor) Name() string { return "social" }

func (e *SocialExtractor) Extract(page *plugin.PageData) ([]plugin.ExtractedItem, error) {
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

	baseURL, _ := url.Parse(page.FinalURL)
	if baseURL == nil {
		baseURL, _ = url.Parse(page.URL)
	}

	seen := make(map[string]bool)
	var items []plugin.ExtractedItem

	doc.Find("a[href]").Each(func(_ int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if !exists || href == "" {
			return
		}

		resolved := resolveURL(baseURL, strings.TrimSpace(href))
		if resolved == "" || seen[resolved] {
			return
		}

		lower := strings.ToLower(resolved)
		for platform, patterns := range e.platforms {
			for _, pattern := range patterns {
				if strings.Contains(lower, pattern) {
					seen[resolved] = true
					text := strings.TrimSpace(s.Text())
					meta := map[string]string{
						"platform": platform,
					}
					if text != "" {
						meta["anchor_text"] = truncate(text, 200)
					}
					items = append(items, plugin.ExtractedItem{
						Type:      "social",
						Value:     resolved,
						SourceURL: page.URL,
						Metadata:  meta,
					})
					return
				}
			}
		}
	})

	return items, nil
}
