package rsscalendar

import (
	"testing"

	"github.com/mmcdole/gofeed"
	ext "github.com/mmcdole/gofeed/extensions"
	"github.com/stretchr/testify/assert"
)

func TestCalendarTimesFromItem(t *testing.T) {
	start, end := calendarTimesFromItem(nil)
	assert.Empty(t, start)
	assert.Empty(t, end)

	item := &gofeed.Item{Extensions: ext.Extensions{"gcal": {"timespan": {
		{Attrs: nil},
		{Attrs: map[string]string{"startTime": "2026-01-01T10:00:00Z"}},
		{Attrs: map[string]string{"startTime": "ignored", "endTime": "2026-01-01T11:00:00Z"}},
	}}}}
	start, end = calendarTimesFromItem(item)
	assert.Equal(t, "2026-01-01T10:00:00Z", start, "first start wins")
	assert.Equal(t, "2026-01-01T11:00:00Z", end)

	start, _ = calendarTimesFromItem(&gofeed.Item{Extensions: ext.Extensions{"other": {}}})
	assert.Empty(t, start)
}
