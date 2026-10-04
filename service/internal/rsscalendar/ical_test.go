package rsscalendar

import "testing"

func TestParseICalendarEvent(t *testing.T) {
	body := `BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//SickRock//Test//EN
X-WR-CALNAME:Team
BEGIN:VEVENT
UID:evt-42@example.com
SUMMARY:Standup
DESCRIPTION:Daily sync
DTSTART:20260101T100000Z
DTEND:20260101T103000Z
LOCATION:Room 1
ORGANIZER:mailto:ada@example.com
STATUS:CONFIRMED
END:VEVENT
END:VCALENDAR`

	feed, err := parseICalendar(body)
	if err != nil {
		t.Fatal(err)
	}
	if feed.Title != "Team" {
		t.Fatalf("title %q", feed.Title)
	}
	if len(feed.Items) != 1 {
		t.Fatalf("items %d", len(feed.Items))
	}
	fields := ItemFields(feed.Items[0])
	if fields["title"] != "Standup" {
		t.Fatalf("title field %q", fields["title"])
	}
	if fields["guid"] != "evt-42@example.com" {
		t.Fatalf("guid %q", fields["guid"])
	}
	if fields["startDate"] != "2026-01-01 10:00:00" {
		t.Fatalf("startDate %q", fields["startDate"])
	}
	if fields["endDate"] != "2026-01-01 10:30:00" {
		t.Fatalf("endDate %q", fields["endDate"])
	}
	if fields["location"] != "Room 1" {
		t.Fatalf("location %q", fields["location"])
	}
	if fields["author"] != "ada@example.com" {
		t.Fatalf("author %q", fields["author"])
	}
	if fields["status"] != "CONFIRMED" {
		t.Fatalf("status %q", fields["status"])
	}
}

func TestIsICalendarDetectsContentTypeAndBody(t *testing.T) {
	if !isICalendar("text/calendar; charset=utf-8", []byte("BEGIN:VCALENDAR")) {
		t.Fatal("content type")
	}
	if !isICalendar("text/plain", []byte("BEGIN:VCALENDAR\r\nEND:VCALENDAR")) {
		t.Fatal("body")
	}
	if isICalendar("application/rss+xml", []byte("<rss></rss>")) {
		t.Fatal("rss should not match")
	}
}
