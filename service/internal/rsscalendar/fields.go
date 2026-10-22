package rsscalendar

import (
	"cmp"
	"strings"
	"time"

	"github.com/mmcdole/gofeed"
	ext "github.com/mmcdole/gofeed/extensions"
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
	start, end := calendarTimesFromItem(item)
	setIfNotEmpty(out, map[string]string{
		"title":       item.Title,
		"link":        item.Link,
		"description": item.Description,
		"content":     item.Content,
		"guid":        item.GUID,
		"pubDate":     parsedOrRawTime(item.PublishedParsed, item.Published),
		"updated":     parsedOrRawTime(item.UpdatedParsed, item.Updated),
		"author":      authorName(item.Author),
	})
	for key, value := range item.Custom {
		if strings.TrimSpace(value) != "" {
			out[key] = value
		}
	}
	setIfNotEmpty(out, map[string]string{
		"startDate": start,
		"endDate":   end,
	})
	addExtensionFields(out, item.Extensions)
	return out
}

// setIfNotEmpty copies the non-empty values of fields into out.
func setIfNotEmpty(out, fields map[string]string) {
	for key, value := range fields {
		if value != "" {
			out[key] = value
		}
	}
}

// parsedOrRawTime formats the parsed time when available, otherwise returns the raw feed value.
func parsedOrRawTime(parsed *time.Time, raw string) string {
	if parsed != nil {
		return formatTime(*parsed)
	}
	return raw
}

func authorName(author *gofeed.Person) string {
	if author == nil {
		return ""
	}
	return author.Name
}

// addExtensionFields adds non-blank feed extension values to out as "ext:<namespace>:<name>".
func addExtensionFields(out map[string]string, extensions ext.Extensions) {
	for key, values := range extensions {
		for subKey, subValues := range values {
			for _, v := range subValues {
				if strings.TrimSpace(v.Value) != "" {
					out["ext:"+key+":"+subKey] = v.Value
				}
			}
		}
	}
}

func calendarTimesFromItem(item *gofeed.Item) (start string, end string) {
	if item == nil || item.Extensions == nil {
		return "", ""
	}
	// Google Calendar RSS uses gcal:timespan with startTime/endTime values.
	for _, span := range item.Extensions["gcal"]["timespan"] {
		if span.Attrs == nil {
			continue
		}
		start = cmp.Or(start, firstAttr(span.Attrs, "startTime", "start"))
		end = cmp.Or(end, firstAttr(span.Attrs, "endTime", "end"))
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
		out = appendUnseenFields(out, seen, ItemFields(item))
	}
	return out
}

func appendUnseenFields(out []string, seen map[string]bool, fields map[string]string) []string {
	for key := range fields {
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, key)
	}
	return out
}
