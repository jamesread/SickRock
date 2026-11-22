package uiusage

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var now = time.Date(2026, 11, 22, 9, 30, 0, 0, time.UTC)

func newStore(t *testing.T) *Store {
	t.Helper()
	s, err := NewStore(t.TempDir())
	require.NoError(t, err)
	return s
}

func readLines(t *testing.T, s *Store, userID int) []map[string]string {
	t.Helper()
	b, err := s.Read(userID)
	require.NoError(t, err)
	var out []map[string]string
	for _, l := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		if l == "" {
			continue
		}
		var m map[string]string
		require.NoError(t, json.Unmarshal([]byte(l), &m), l)
		out = append(out, m)
	}
	return out
}

func TestRecordWritesJSONLines(t *testing.T) {
	s := newStore(t)
	n, err := s.Record(7, []Event{
		{Kind: KindRoute, FromPath: "/", ToPath: "/table/pantry", TimestampMs: now.UnixMilli(), SessionID: "abc", AppVersion: "1.4.0"},
		{Kind: KindSidebar, ToPath: "/table/pantry", Label: "Pantry"},
		{Kind: "keypress", ToPath: "/x"},
	}, "1.5.0", now)
	require.NoError(t, err)
	assert.Equal(t, 2, n, "unknown kinds are skipped")

	lines := readLines(t, s, 7)
	require.Len(t, lines, 2)
	assert.Equal(t, map[string]string{
		"ts": "2026-11-22T09:30:00.000Z", "version": "1.4.0", "session": "abc",
		"kind": "route", "from": "/", "to": "/table/pantry",
	}, lines[0])
	assert.Equal(t, "1.5.0", lines[1]["version"], "the server version stands in for a missing one")
	assert.Equal(t, "Pantry", lines[1]["label"])
	assert.NotContains(t, lines[1], "from", "empty fields are left out")
}

func TestRecordKeepsNewestLines(t *testing.T) {
	s := newStore(t)
	for i := range MaxLines + 50 {
		_, err := s.Record(1, []Event{{Kind: KindRoute, ToPath: "/p/" + strings.Repeat("x", i%3), Label: string(rune('a' + i%26))}}, "v", now.Add(time.Duration(i)*time.Second))
		require.NoError(t, err)
	}
	count, err := s.LineCount(1)
	require.NoError(t, err)
	assert.Equal(t, MaxLines, count)
	lines := readLines(t, s, 1)
	assert.Equal(t, now.Add(time.Duration(MaxLines+49)*time.Second).Format("2006-01-02T15:04:05.000Z07:00"), lines[len(lines)-1]["ts"])
}

func TestRecordLimitsEventsAndFieldLength(t *testing.T) {
	s := newStore(t)
	events := make([]Event, MaxEventsPerRecord+10)
	for i := range events {
		events[i] = Event{Kind: KindHeader, Label: strings.Repeat("é", maxFieldLength)}
	}
	n, err := s.Record(1, events, "v", now)
	require.NoError(t, err)
	assert.Equal(t, MaxEventsPerRecord, n)
	label := readLines(t, s, 1)[0]["label"]
	assert.LessOrEqual(t, len(label), maxFieldLength)
	assert.True(t, strings.HasSuffix(label, "é"), "truncation keeps whole characters")
}

func TestLogsArePerUserAndClearable(t *testing.T) {
	s := newStore(t)
	_, err := s.Record(1, []Event{{Kind: KindRoute, ToPath: "/a"}}, "v", now)
	require.NoError(t, err)
	_, err = s.Record(2, []Event{{Kind: KindRoute, ToPath: "/b"}}, "v", now)
	require.NoError(t, err)
	assert.Equal(t, "/a", readLines(t, s, 1)[0]["to"])
	assert.Equal(t, "/b", readLines(t, s, 2)[0]["to"])

	require.NoError(t, s.Clear(1))
	require.NoError(t, s.Clear(1), "clearing twice is fine")
	b, err := s.Read(1)
	require.NoError(t, err)
	assert.Nil(t, b)
	_, err = os.Stat(s.Path(2))
	require.NoError(t, err)
}
