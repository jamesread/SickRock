package rsscalendar

import (
	"testing"

	"github.com/mmcdole/gofeed"
)

func TestItemFieldsIncludesGuidAndTitle(t *testing.T) {
	item := &gofeed.Item{
		Title: "Team standup",
		GUID:  "evt-1",
		Link:  "https://example.com/e/1",
	}
	fields := ItemFields(item)
	if fields["title"] != "Team standup" {
		t.Fatalf("title: %q", fields["title"])
	}
	if fields["guid"] != "evt-1" {
		t.Fatalf("guid: %q", fields["guid"])
	}
}

func TestApplyMappings(t *testing.T) {
	out := ApplyMappings(map[string]string{
		"title":   "Meet",
		"pubDate": "2026-01-01 10:00:00",
	}, []FieldMapping{
		{RssField: "title", Column: "name"},
		{RssField: "pubDate", Column: "start_at"},
	})
	if out["name"] != "Meet" || out["start_at"] != "2026-01-01 10:00:00" {
		t.Fatalf("unexpected mapping: %v", out)
	}
}
