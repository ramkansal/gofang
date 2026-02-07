package extractor

import (
	"fmt"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/ramkansal/gofang/pkg/plugin"
)

// FormsExtractor extracts all HTML forms and their inputs.
type FormsExtractor struct{}

func NewFormsExtractor() *FormsExtractor { return &FormsExtractor{} }

func (e *FormsExtractor) Name() string { return "forms" }

func (e *FormsExtractor) Extract(page *plugin.PageData) ([]plugin.ExtractedItem, error) {
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

	var items []plugin.ExtractedItem

	doc.Find("form").Each(func(i int, s *goquery.Selection) {
		action, _ := s.Attr("action")
		method, _ := s.Attr("method")
		enctype, _ := s.Attr("enctype")
		name, _ := s.Attr("name")
		id, _ := s.Attr("id")

		if method == "" {
			method = "GET"
		}
		method = strings.ToUpper(method)

		// Collect all input fields
		var inputs []string
		s.Find("input, select, textarea").Each(func(_ int, inp *goquery.Selection) {
			inputName, _ := inp.Attr("name")
			inputType, _ := inp.Attr("type")
			inputValue, _ := inp.Attr("value")
			placeholder, _ := inp.Attr("placeholder")

			tag := goquery.NodeName(inp)
			if inputName == "" && id == "" {
				return
			}

			desc := fmt.Sprintf("%s[name=%s,type=%s", tag, inputName, inputType)
			if inputValue != "" {
				desc += ",value=" + truncate(inputValue, 50)
			}
			if placeholder != "" {
				desc += ",placeholder=" + truncate(placeholder, 50)
			}
			desc += "]"
			inputs = append(inputs, desc)
		})

		meta := map[string]string{
			"method":      method,
			"input_count": fmt.Sprintf("%d", len(inputs)),
		}
		if action != "" {
			meta["action"] = action
		}
		if enctype != "" {
			meta["enctype"] = enctype
		}
		if name != "" {
			meta["name"] = name
		}
		if id != "" {
			meta["id"] = id
		}
		if len(inputs) > 0 {
			meta["inputs"] = strings.Join(inputs, " | ")
		}

		formDesc := fmt.Sprintf("FORM#%d %s %s (%d inputs)", i+1, method, action, len(inputs))

		items = append(items, plugin.ExtractedItem{
			Type:      "form",
			Value:     formDesc,
			SourceURL: page.URL,
			Metadata:  meta,
		})
	})

	return items, nil
}
