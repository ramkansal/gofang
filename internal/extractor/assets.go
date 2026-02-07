package extractor

import (
	"net/url"
	"path"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/ramkansal/web-crawler/pkg/plugin"
)

// AssetsExtractor extracts imges, scripts, stylesheets, PDFs, videos, fonts, etc.
type AssetsExtractor struct{}

func NewAssetsExtractor() *AssetsExtractor { return &AssetsExtractor{} }

func (e *AssetsExtractor) Name() string { return "assets" }

func (e *AssetsExtractor) Extract(page *plugin.PageData) ([]plugin.ExtractedItem, error) {
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

	addAsset := func(rawURL, assetType, tag string, extra map[string]string) {
		resolved := resolveURL(baseURL, strings.TrimSpace(rawURL))
		if resolved == "" || seen[resolved] {
			return
		}
		seen[resolved] = true

		meta := map[string]string{
			"asset_type": assetType,
			"tag":        tag,
		}
		// Guess file extension
		if u, err := url.Parse(resolved); err == nil {
			ext := strings.ToLower(path.Ext(u.Path))
			if ext != "" {
				meta["extension"] = ext
			}
		}
		for k, v := range extra {
			meta[k] = v
		}

		items = append(items, plugin.ExtractedItem{
			Type:      "asset",
			Value:     resolved,
			SourceURL: page.URL,
			Metadata:  meta,
		})
	}

	// Images: <img src>, <img srcset>, <picture source>
	doc.Find("img[src]").Each(func(_ int, s *goquery.Selection) {
		src, _ := s.Attr("src")
		alt, _ := s.Attr("alt")
		extra := map[string]string{}
		if alt != "" {
			extra["alt"] = truncate(alt, 200)
		}
		if w, exists := s.Attr("width"); exists {
			extra["width"] = w
		}
		if h, exists := s.Attr("height"); exists {
			extra["height"] = h
		}
		addAsset(src, "image", "img", extra)
	})

	doc.Find("img[srcset]").Each(func(_ int, s *goquery.Selection) {
		srcset, _ := s.Attr("srcset")
		for _, entry := range strings.Split(srcset, ",") {
			parts := strings.Fields(strings.TrimSpace(entry))
			if len(parts) > 0 {
				addAsset(parts[0], "image", "img-srcset", nil)
			}
		}
	})

	doc.Find("picture source[srcset]").Each(func(_ int, s *goquery.Selection) {
		srcset, _ := s.Attr("srcset")
		mediaType, _ := s.Attr("type")
		for _, entry := range strings.Split(srcset, ",") {
			parts := strings.Fields(strings.TrimSpace(entry))
			if len(parts) > 0 {
				extra := map[string]string{}
				if mediaType != "" {
					extra["media_type"] = mediaType
				}
				addAsset(parts[0], "image", "picture-source", extra)
			}
		}
	})

	// Scripts
	doc.Find("script[src]").Each(func(_ int, s *goquery.Selection) {
		src, _ := s.Attr("src")
		extra := map[string]string{}
		if t, exists := s.Attr("type"); exists {
			extra["script_type"] = t
		}
		if _, exists := s.Attr("async"); exists {
			extra["async"] = "true"
		}
		if _, exists := s.Attr("defer"); exists {
			extra["defer"] = "true"
		}
		addAsset(src, "script", "script", extra)
	})

	// Stylesheets
	doc.Find(`link[rel="stylesheet"]`).Each(func(_ int, s *goquery.Selection) {
		href, _ := s.Attr("href")
		extra := map[string]string{}
		if media, exists := s.Attr("media"); exists {
			extra["media"] = media
		}
		addAsset(href, "stylesheet", "link", extra)
	})

	// Fonts (preload)
	doc.Find(`link[rel="preload"][as="font"]`).Each(func(_ int, s *goquery.Selection) {
		href, _ := s.Attr("href")
		addAsset(href, "font", "link-preload", nil)
	})

	// Favicons and icons
	doc.Find(`link[rel="icon"], link[rel="shortcut icon"], link[rel="apple-touch-icon"]`).Each(func(_ int, s *goquery.Selection) {
		href, _ := s.Attr("href")
		rel, _ := s.Attr("rel")
		addAsset(href, "icon", "link", map[string]string{"rel": rel})
	})

	// PDFs and other linked files
	doc.Find("a[href]").Each(func(_ int, s *goquery.Selection) {
		href, _ := s.Attr("href")
		lower := strings.ToLower(href)
		fileTypes := map[string]string{
			".pdf":  "document",
			".doc":  "document",
			".docx": "document",
			".xls":  "spreadsheet",
			".xlsx": "spreadsheet",
			".csv":  "data",
			".zip":  "archive",
			".tar":  "archive",
			".gz":   "archive",
			".rar":  "archive",
			".mp4":  "video",
			".webm": "video",
			".mp3":  "audio",
			".wav":  "audio",
			".svg":  "image",
		}
		for ext, assetType := range fileTypes {
			if strings.HasSuffix(lower, ext) || strings.Contains(lower, ext+"?") {
				addAsset(href, assetType, "a", nil)
				break
			}
		}
	})

	// Video sources
	doc.Find("video source[src], video[src]").Each(func(_ int, s *goquery.Selection) {
		src, _ := s.Attr("src")
		if src != "" {
			addAsset(src, "video", "video", nil)
		}
	})

	// Audio sources
	doc.Find("audio source[src], audio[src]").Each(func(_ int, s *goquery.Selection) {
		src, _ := s.Attr("src")
		if src != "" {
			addAsset(src, "audio", "audio", nil)
		}
	})

	return items, nil
}
