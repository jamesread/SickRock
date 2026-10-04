package rsscalendar

import (
	"fmt"
	"strings"

	ics "github.com/arran4/golang-ical"
	"github.com/mmcdole/gofeed"
)

func isICalendar(contentType string, body []byte) bool {
	ct := strings.ToLower(contentType)
	if strings.Contains(ct, "text/calendar") || strings.Contains(ct, "application/ics") {
		return true
	}
	trimmed := strings.TrimSpace(string(body))
	trimmed = strings.TrimPrefix(trimmed, "\uFEFF")
	return strings.HasPrefix(strings.ToUpper(trimmed), "BEGIN:VCALENDAR")
}

func parseICalendar(body string) (*gofeed.Feed, error) {
	cal, err := ics.ParseCalendar(strings.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("parse icalendar: %w", err)
	}

	feed := &gofeed.Feed{
		Title: calendarTitle(cal),
		Items: make([]*gofeed.Item, 0, len(cal.Events())),
	}
	for _, event := range cal.Events() {
		if event == nil {
			continue
		}
		feed.Items = append(feed.Items, icalEventToItem(event))
	}
	return feed, nil
}

func calendarTitle(cal *ics.Calendar) string {
	if cal == nil {
		return "iCalendar"
	}
	for _, prop := range cal.CalendarProperties {
		if strings.EqualFold(prop.IANAToken, "X-WR-CALNAME") && strings.TrimSpace(prop.Value) != "" {
			return unescapeICSText(prop.Value)
		}
	}
	return "iCalendar"
}

func icalEventToItem(event *ics.VEvent) *gofeed.Item {
	item := &gofeed.Item{
		Title:       unescapeICSText(propValue(event, ics.ComponentPropertySummary)),
		Description: unescapeICSText(propValue(event, ics.ComponentPropertyDescription)),
		GUID:        strings.TrimSpace(event.Id()),
		Link:        strings.TrimSpace(propValue(event, ics.ComponentPropertyUrl)),
		Custom:      map[string]string{},
	}
	if item.Description != "" {
		item.Content = item.Description
	}

	if start, err := event.GetStartAt(); err == nil {
		t := start.UTC()
		item.PublishedParsed = &t
		item.Published = formatTime(t)
		item.Custom["startDate"] = formatTime(t)
	}
	if end, err := event.GetEndAt(); err == nil {
		item.Custom["endDate"] = formatTime(end.UTC())
	}
	if modified, err := event.GetLastModifiedAt(); err == nil {
		t := modified.UTC()
		item.UpdatedParsed = &t
		item.Updated = formatTime(t)
	}

	if organizer := strings.TrimSpace(propValue(event, ics.ComponentPropertyOrganizer)); organizer != "" {
		item.Author = &gofeed.Person{Name: organizerDisplay(organizer)}
	}
	if location := unescapeICSText(propValue(event, ics.ComponentPropertyLocation)); location != "" {
		item.Custom["location"] = location
	}
	if status := strings.TrimSpace(propValue(event, ics.ComponentPropertyStatus)); status != "" {
		item.Custom["status"] = status
	}
	return item
}

func propValue(event *ics.VEvent, name ics.ComponentProperty) string {
	prop := event.GetProperty(name)
	if prop == nil {
		return ""
	}
	return prop.Value
}

func organizerDisplay(raw string) string {
	raw = strings.TrimSpace(raw)
	lower := strings.ToLower(raw)
	if strings.HasPrefix(lower, "mailto:") {
		return raw[len("mailto:"):]
	}
	return raw
}

func unescapeICSText(value string) string {
	value = strings.ReplaceAll(value, `\n`, "\n")
	value = strings.ReplaceAll(value, `\N`, "\n")
	value = strings.ReplaceAll(value, `\,`, ",")
	value = strings.ReplaceAll(value, `\;`, ";")
	value = strings.ReplaceAll(value, `\\`, `\`)
	return strings.TrimSpace(value)
}
