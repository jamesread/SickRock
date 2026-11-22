package notifications

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	repo "github.com/jamesread/SickRock/internal/repo"
	"github.com/jamesread/SickRock/internal/testdb"
)

// hookServer records the paths and payloads posted to it.
type hookServer struct {
	*httptest.Server
	mu       sync.Mutex
	paths    []string
	payloads []map[string]any
}

func newHookServer(t *testing.T) *hookServer {
	t.Helper()
	h := &hookServer{}
	h.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		_ = json.NewDecoder(r.Body).Decode(&payload)
		h.mu.Lock()
		h.paths = append(h.paths, r.URL.Path)
		h.payloads = append(h.payloads, payload)
		h.mu.Unlock()
		if r.URL.Path == "/broken" {
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	t.Cleanup(h.Close)
	return h
}

func TestSendToUserOnlyReachesThatUsersSubscribedChannels(t *testing.T) {
	ctx := context.Background()
	db := testdb.NewMigratedSQLite(t)
	r := repo.NewRepository(db)
	ns := NewNotificationService(r)
	hooks := newHookServer(t)

	newUser := func(name string) int {
		res, err := db.ExecContext(ctx, `INSERT INTO user_accounts (username) VALUES (?)`, name)
		require.NoError(t, err)
		id, _ := res.LastInsertId()
		return int(id)
	}
	alice, bob := newUser("alice"), newUser("bob")
	notified, err := r.GetNotificationEventByCode(ctx, EventUserNotified)
	require.NoError(t, err)
	health, err := r.GetNotificationEventByCode(ctx, "health.issue_raised")
	require.NoError(t, err)

	subscribe := func(user int, path string, eventID int, active bool) {
		ch, chErr := r.CreateUserNotificationChannel(ctx, user, "webhook", hooks.URL+path, nil)
		require.NoError(t, chErr)
		if !active {
			require.NoError(t, r.UpdateUserNotificationChannel(ctx, ch.ID, hooks.URL+path, nil, false))
		}
		_, subErr := r.CreateUserNotificationSubscription(ctx, user, eventID, ch.ID)
		require.NoError(t, subErr)
	}
	subscribe(alice, "/alice", notified.ID, true)
	subscribe(bob, "/bob", notified.ID, true)
	subscribe(bob, "/bob-paused", notified.ID, false)
	subscribe(bob, "/bob-health", health.ID, true)
	subscribe(bob, "/broken", notified.ID, true)

	data := map[string]any{"sender": "alice", "title": "Plan ready", "message": "Have a look", "link": "https://sr.test/table/meals"}
	subscribed, delivered, err := ns.SendToUser(ctx, bob, EventUserNotified, data)
	require.NoError(t, err)
	assert.Equal(t, 2, subscribed)
	assert.Equal(t, 1, delivered)
	assert.ElementsMatch(t, []string{"/bob", "/broken"}, hooks.paths)
	assert.Equal(t, "alice: Plan ready\nHave a look\nhttps://sr.test/table/meals", hooks.payloads[0]["message"])
	assert.Equal(t, "Plan ready", hooks.payloads[0]["data"].(map[string]any)["title"])

	subscribed, delivered, err = ns.SendToUser(ctx, newUser("carol"), EventUserNotified, data)
	require.NoError(t, err)
	assert.Equal(t, 0, subscribed)
	assert.Equal(t, 0, delivered)
}

func TestTargetedEventsAreNeverBroadcast(t *testing.T) {
	db := testdb.NewMigratedSQLite(t)
	ns := NewNotificationService(repo.NewRepository(db))
	err := ns.SendNotification(context.Background(), EventUserNotified, map[string]any{"title": "x"})
	assert.ErrorContains(t, err, "chosen users only")
}

func TestUserNotifiedMessage(t *testing.T) {
	ns := &NotificationService{}
	assert.Equal(t, "alice: Plan ready", ns.buildMessage(EventUserNotified, map[string]any{"sender": "alice", "title": "Plan ready"}))
	assert.Equal(t, "Someone: T\nhttps://x.test/a", ns.buildMessage(EventUserNotified, map[string]any{"title": "T", "link": "https://x.test/a"}))
}
