package rsscalendar

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/mmcdole/gofeed"
	log "github.com/sirupsen/logrus"
)

const (
	defaultHTTPTimeout = 45 * time.Second
	maxFeedBodyBytes   = 4 << 20 // 4 MiB
	bodySnippetLen     = 512
)

// FetchFeed downloads and parses an RSS/Atom feed URL.
func FetchFeed(ctx context.Context, feedURL string) (*gofeed.Feed, error) {
	feedURL = strings.TrimSpace(feedURL)
	if feedURL == "" {
		return nil, fmt.Errorf("feed URL is required")
	}

	log.WithField("feedURL", feedURL).Debug("RSS calendar: fetching feed")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, feedURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "SickRock-RSS-Calendar/1.0")
	req.Header.Set("Accept", "application/rss+xml, application/atom+xml, application/xml, text/xml, text/calendar, */*")

	client := &http.Client{Timeout: defaultHTTPTimeout}
	resp, err := client.Do(req)
	if err != nil {
		log.WithError(err).WithField("feedURL", feedURL).Warn("RSS calendar: HTTP request failed")
		return nil, err
	}
	defer resp.Body.Close()

	body, readErr := io.ReadAll(io.LimitReader(resp.Body, maxFeedBodyBytes))
	if readErr != nil {
		log.WithError(readErr).WithFields(log.Fields{
			"feedURL":    feedURL,
			"statusCode": resp.StatusCode,
		}).Warn("RSS calendar: failed reading response body")
		return nil, readErr
	}

	contentType := resp.Header.Get("Content-Type")
	log.WithFields(log.Fields{
		"feedURL":       feedURL,
		"statusCode":    resp.StatusCode,
		"contentType":   contentType,
		"contentLength": resp.ContentLength,
		"bytesRead":     len(body),
	}).Info("RSS calendar: feed HTTP response")

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet := bodySnippet(body)
		log.WithFields(log.Fields{
			"feedURL":    feedURL,
			"statusCode": resp.StatusCode,
			"snippet":    snippet,
		}).Warn("RSS calendar: non-success HTTP status")
		return nil, fmt.Errorf("feed HTTP %d: %s", resp.StatusCode, snippet)
	}

	if len(body) == 0 {
		log.WithField("feedURL", feedURL).Warn("RSS calendar: empty response body")
		return nil, fmt.Errorf("parse feed: empty response body")
	}

	var feed *gofeed.Feed
	var parseErr error
	format := "rss"
	if isICalendar(contentType, body) {
		format = "ical"
		log.WithFields(log.Fields{
			"feedURL":     feedURL,
			"contentType": contentType,
		}).Info("RSS calendar: parsing as iCalendar")
		feed, parseErr = parseICalendar(string(body))
	} else {
		parser := gofeed.NewParser()
		feed, parseErr = parser.ParseString(string(body))
	}
	if parseErr != nil {
		snippet := bodySnippet(body)
		looksLikeHTML := strings.Contains(strings.ToLower(snippet), "<html")
		log.WithError(parseErr).WithFields(log.Fields{
			"feedURL":       feedURL,
			"contentType":   contentType,
			"format":        format,
			"bytesRead":     len(body),
			"snippet":       snippet,
			"looksLikeHTML": looksLikeHTML,
		}).Warn("RSS calendar: failed to parse feed")
		hint := ""
		if looksLikeHTML {
			hint = " (response looks like HTML, not a calendar feed)"
		}
		return nil, fmt.Errorf("parse feed: %w%s", parseErr, hint)
	}

	log.WithFields(log.Fields{
		"feedURL":   feedURL,
		"format":    format,
		"feedTitle": feed.Title,
		"itemCount": len(feed.Items),
	}).Info("RSS calendar: feed parsed successfully")

	return feed, nil
}

func bodySnippet(body []byte) string {
	if len(body) == 0 {
		return ""
	}
	n := bodySnippetLen
	if len(body) < n {
		n = len(body)
	}
	s := strings.TrimSpace(string(body[:n]))
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	if !utf8.ValidString(s) {
		return fmt.Sprintf("%q", body[:n])
	}
	return s
}
