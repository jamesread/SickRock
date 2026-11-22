// Package uiusage keeps each user's opt-in UI usage log: navigation through routes, the sidebar and
// the header, one JSON object per line (JSONL) in a file per user, for offline analysis of
// navigation loops and hot spots. Only the newest MaxLines lines are kept.
package uiusage

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// MaxLines is how many lines a user's log keeps; older lines are dropped as new ones arrive.
const MaxLines = 1000

// MaxEventsPerRecord is how many events one Record call accepts; the rest are dropped.
const MaxEventsPerRecord = 200

// maxFieldLength truncates free-text fields (paths and labels).
const maxFieldLength = 512

// Kinds of event.
const (
	KindRoute   = "route"
	KindSidebar = "sidebar"
	KindHeader  = "header"
)

// Event is one tracked navigation, as received from the client.
type Event struct {
	Kind        string
	FromPath    string
	ToPath      string
	Label       string
	TimestampMs int64
	SessionID   string
	AppVersion  string
}

// line is how an event is written: keys are short and empty ones are left out.
type line struct {
	Time    string `json:"ts"`
	Version string `json:"version"`
	Session string `json:"session,omitempty"`
	Kind    string `json:"kind"`
	From    string `json:"from,omitempty"`
	To      string `json:"to,omitempty"`
	Label   string `json:"label,omitempty"`
}

// Store keeps the logs as <dir>/user-<id>.jsonl.
type Store struct {
	dir   string
	locks sync.Map // user id -> *sync.Mutex
}

// NewStore creates dir if needed.
func NewStore(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("create UI usage directory: %w", err)
	}
	return &Store{dir: dir}, nil
}

// Path is the file holding a user's log (it may not exist yet).
func (s *Store) Path(userID int) string {
	return filepath.Join(s.dir, fmt.Sprintf("user-%d.jsonl", userID))
}

func (s *Store) lock(userID int) func() {
	mu, _ := s.locks.LoadOrStore(userID, &sync.Mutex{})
	m := mu.(*sync.Mutex)
	m.Lock()
	return m.Unlock
}

// Record appends events to a user's log, keeping the newest MaxLines lines. Events of an unknown
// kind are skipped. version stands in for an event without its own app version. It returns how
// many events were written.
func (s *Store) Record(userID int, events []Event, version string, now time.Time) (int, error) {
	newLines := make([][]byte, 0, min(len(events), MaxEventsPerRecord))
	for _, e := range events[:min(len(events), MaxEventsPerRecord)] {
		b, ok := encode(e, version, now)
		if ok {
			newLines = append(newLines, b)
		}
	}
	if len(newLines) == 0 {
		return 0, nil
	}
	defer s.lock(userID)()
	existing, err := s.readLines(userID)
	if err != nil {
		return 0, err
	}
	all := append(existing, newLines...)
	if len(all) > MaxLines {
		all = all[len(all)-MaxLines:]
	}
	if err := s.writeLines(userID, all); err != nil {
		return 0, err
	}
	return len(newLines), nil
}

// LineCount is how many lines a user's log holds.
func (s *Store) LineCount(userID int) (int, error) {
	defer s.lock(userID)()
	lines, err := s.readLines(userID)
	return len(lines), err
}

// Read returns a user's log, or nil when there is none.
func (s *Store) Read(userID int) ([]byte, error) {
	defer s.lock(userID)()
	b, err := os.ReadFile(s.Path(userID))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	return b, err
}

// Clear deletes a user's log.
func (s *Store) Clear(userID int) error {
	defer s.lock(userID)()
	err := os.Remove(s.Path(userID))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return err
}

func encode(e Event, version string, now time.Time) ([]byte, bool) {
	switch e.Kind {
	case KindRoute, KindSidebar, KindHeader:
	default:
		return nil, false
	}
	ts := now
	if e.TimestampMs > 0 {
		ts = time.UnixMilli(e.TimestampMs)
	}
	b, err := json.Marshal(line{
		Time:    ts.UTC().Format("2006-01-02T15:04:05.000Z07:00"),
		Version: truncate(firstNonEmpty(e.AppVersion, version)),
		Session: truncate(e.SessionID),
		Kind:    e.Kind,
		From:    truncate(e.FromPath),
		To:      truncate(e.ToPath),
		Label:   truncate(e.Label),
	})
	return b, err == nil
}

func (s *Store) readLines(userID int) ([][]byte, error) {
	f, err := os.Open(s.Path(userID))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	var out [][]byte
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		if b := bytes.TrimSpace(sc.Bytes()); len(b) > 0 {
			out = append(out, append([]byte(nil), b...))
		}
	}
	return out, sc.Err()
}

// writeLines replaces a user's log atomically (a temporary file renamed over it).
func (s *Store) writeLines(userID int, lines [][]byte) error {
	tmp, err := os.CreateTemp(s.dir, fmt.Sprintf(".user-%d-*.jsonl", userID))
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	w := bufio.NewWriter(tmp)
	for _, l := range lines {
		_, _ = w.Write(l)
		_ = w.WriteByte('\n')
	}
	if err := w.Flush(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), s.Path(userID))
}

func truncate(s string) string {
	s = strings.TrimSpace(s)
	if len(s) <= maxFieldLength {
		return s
	}
	cut := maxFieldLength
	for cut > 0 && !utf8Start(s[cut]) {
		cut--
	}
	return s[:cut]
}

// utf8Start reports whether b starts a UTF-8 character, so truncation never splits one.
func utf8Start(b byte) bool { return b&0xC0 != 0x80 }

func firstNonEmpty(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}
