package rsscalendar

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseFeedBodyRSS(t *testing.T) {
	body := []byte(`<?xml version="1.0"?><rss version="2.0"><channel><title>Events</title><item><title>Party</title></item></channel></rss>`)
	feed, format, err := parseFeedBody("https://example.com/feed", "application/rss+xml", body)
	require.NoError(t, err)
	assert.Equal(t, "rss", format)
	assert.Equal(t, "Events", feed.Title)
	assert.Len(t, feed.Items, 1)
}

func TestParseFeedBodyHTMLHint(t *testing.T) {
	_, _, err := parseFeedBody("https://example.com/feed", "text/html", []byte("<html><body>Login</body></html>"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "looks like HTML")
}
