package rsscalendar

import (
	"strings"
	"time"

	"github.com/mmcdole/gofeed"
)

// KnownRssFields are suggested source fields for drag-and-drop mapping in the UI.
var KnownRssFields = []string{
	"title",
	"link",
	"description",
	"content",
	"guid",
	"pubDate",
	"updated",
	"author",
	"startDate",
	"endDate",
	"location",
	"status",
}

// ItemFields returns string values for a feed item keyed by RSS field names.
func ItemFields(item *gofeed.Item) map[string]string {
	out := map[string]string{}
	if item == nil {
		return out
	}
	if item.Title != "" {
		out["title"] = item.Title
	}
	if item.Link != "" {
		out["link"] = item.Link
	}
	if item.Description != "" {
		out["description"] = item.Description
	}
	if item.Content != "" {
		out["content"] = item.Content
	}
	if item.GUID != "" {
		out["guid"] = item.GUID
	}
	if item.PublishedParsed != nil {
		out["pubDate"] = formatTime(*item.PublishedParsed)
	} else if item.Published != "" {
		out["pubDate"] = item.Published
	}
	if item.UpdatedParsed != nil {
		out["updated"] = formatTime(*item.UpdatedParsed)
	} else if item.Updated != "" {
		out["updated"] = item.Updated
	}
	if item.Author != nil && item.Author.Name != "" {
		out["author"] = item.Author.Name
	}
	for key, value := range item.Custom {
		if strings.TrimSpace(value) == "" {
			continue
		}
		out[key] = value
	}
	start, end := calendarTimesFromItem(item)
	if start != "" {
		out["startDate"] = start
	}
	if end != "" {
		out["endDate"] = end
	}
	for key, values := range item.Extensions {
		for subKey, subValues := range values {
			for _, v := range subValues {
				if strings.TrimSpace(v.Value) == "" {
					continue
				}
				out["ext:"+key+":"+subKey] = v.Value
			}
		}
	}
	return out
}

func calendarTimesFromItem(item *gofeed.Item) (start string, end string) {
	if item == nil || item.Extensions == nil {
		return "", ""
	}
	// Google Calendar RSS uses gcal:timespan with startTime/endTime values.
	if gcal, ok := item.Extensions["gcal"]; ok {
		if spans, ok := gcal["timespan"]; ok {
			for _, span := range spans {
				if span.Attrs == nil {
					continue
				}
				if start == "" {
					start = firstAttr(span.Attrs, "startTime", "start")
				}
				if end == "" {
					end = firstAttr(span.Attrs, "endTime", "end")
				}
			}
		}
	}
	return start, end
}

func firstAttr(attrs map[string]string, keys ...string) string {
	for _, key := range keys {
		if v := strings.TrimSpace(attrs[key]); v != "" {
			return v
		}
	}
	return ""
}

func formatTime(t time.Time) string {
	return t.UTC().Format("2006-01-02 15:04:05")
}

// DiscoverFields collects union of field names from parsed items (includes extensions).
func DiscoverFields(items []*gofeed.Item) []string {
	seen := map[string]bool{}
	var out []string
	for _, name := range KnownRssFields {
		seen[name] = true
		out = append(out, name)
	}
	for _, item := range items {
		for key := range ItemFields(item) {
			if seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, key)
		}
	}
	return out
}
