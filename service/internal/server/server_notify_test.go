package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"connectrpc.com/authn"
	"connectrpc.com/connect"
	armlayer "github.com/jamesread/armature-iam/layer"
	"github.com/jamesread/armature-iam/rbac"
	armstore "github.com/jamesread/armature-iam/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sickrockpb "github.com/jamesread/SickRock/gen/proto"
	"github.com/jamesread/SickRock/internal/iam/systemroles"
	"github.com/jamesread/SickRock/internal/repo"
	"github.com/jamesread/SickRock/internal/testdb"
)

type notifyFixture struct {
	srv               *SickRockServer
	repo              *repo.Repository
	alice, bob, carol int // alice and bob share a group; carol is in another
	hookMu            sync.Mutex
	hookMessages      []string
	hookURL           string
}

func newNotifyFixture(t *testing.T) *notifyFixture {
	t.Helper()
	ctx := context.Background()
	db := testdb.NewMigratedSQLite(t)
	f := &notifyFixture{repo: repo.NewRepository(db)}
	f.srv = NewSickRockServer(f.repo, nil)

	hook := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Message string `json:"message"`
		}
		_ = json.NewDecoder(r.Body).Decode(&payload)
		f.hookMu.Lock()
		f.hookMessages = append(f.hookMessages, payload.Message)
		f.hookMu.Unlock()
	}))
	t.Cleanup(hook.Close)
	f.hookURL = hook.URL

	user := func(name string) int {
		res, err := db.ExecContext(ctx, `INSERT INTO user_accounts (username) VALUES (?)`, name)
		require.NoError(t, err)
		id, _ := res.LastInsertId()
		return int(id)
	}
	group := func(name string, members ...int) {
		res, err := db.ExecContext(ctx, `INSERT INTO user_groups (name, created_at, updated_at) VALUES (?, datetime('now'), datetime('now'))`, name)
		require.NoError(t, err)
		gid, _ := res.LastInsertId()
		for _, m := range members {
			_, err := db.ExecContext(ctx, `INSERT INTO user_group_memberships (created_at, updated_at, user_account_id, user_group_id) VALUES (datetime('now'), datetime('now'), ?, ?)`, m, gid)
			require.NoError(t, err)
		}
	}
	f.alice, f.bob, f.carol = user("alice"), user("bob"), user("carol")
	group("Kitchen", f.alice, f.bob)
	group("Elsewhere", f.carol)
	return f
}

// subscribe gives user a webhook channel subscribed to user.notified.
func (f *notifyFixture) subscribe(t *testing.T, user int) {
	t.Helper()
	ctx := context.Background()
	event, err := f.repo.GetNotificationEventByCode(ctx, "user.notified")
	require.NoError(t, err)
	ch, err := f.repo.CreateUserNotificationChannel(ctx, user, "webhook", f.hookURL+"/"+t.Name(), nil)
	require.NoError(t, err)
	_, err = f.repo.CreateUserNotificationSubscription(ctx, user, event.ID, ch.ID)
	require.NoError(t, err)
}

// as returns a context signed in as user with the given permissions, as the auth layer sets it.
func as(user int, username string, permissions ...string) context.Context {
	perms := map[string]bool{}
	for _, p := range permissions {
		perms[p] = true
	}
	return authn.SetInfo(context.Background(), &armlayer.AuthenticatedUser{
		User: &armstore.UserAccountRow{ID: user, Username: username},
		RBAC: &rbac.EffectiveRBAC{Permissions: perms},
	})
}

func notify(ctx context.Context, srv *SickRockServer, req *sickrockpb.NotifyUsersRequest) (*sickrockpb.NotifyUsersResponse, error) {
	res, err := srv.NotifyUsers(ctx, connect.NewRequest(req))
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}

func TestNotifyUsersDeliversToSubscribedRecipient(t *testing.T) {
	f := newNotifyFixture(t)
	f.subscribe(t, f.bob)
	ctx := context.WithValue(as(f.alice, "alice", systemroles.PermissionNotificationsSend), requestOriginKey{}, "https://sr.test")

	res, err := notify(ctx, f.srv, &sickrockpb.NotifyUsersRequest{
		RecipientUserIds: []int32{int32(f.bob), int32(f.bob)}, // repeats are sent once
		Title:            "  Weekly plan ready ",
		Message:          "Dinners are sorted\nEnjoy",
		LinkPath:         "/table/meals?view=3&week=2026-11-02",
	})
	require.NoError(t, err)
	require.Len(t, res.GetResults(), 1)
	assert.Equal(t, NotifyStatusSent, res.GetResults()[0].GetStatus())
	assert.Equal(t, "bob", res.GetResults()[0].GetUsername())
	assert.EqualValues(t, 1, res.GetResults()[0].GetDeliveredChannels())
	assert.Equal(t, []string{"alice: Weekly plan ready\nDinners are sorted\nEnjoy\nhttps://sr.test/table/meals?view=3&week=2026-11-02"}, f.hookMessages)
}

func TestNotifyUsersReportsUnsubscribedRecipients(t *testing.T) {
	f := newNotifyFixture(t)
	res, err := notify(as(f.alice, "alice", systemroles.PermissionNotificationsSend), f.srv, &sickrockpb.NotifyUsersRequest{
		RecipientUserIds: []int32{int32(f.bob)}, Title: "Hi",
	})
	require.NoError(t, err)
	assert.Equal(t, NotifyStatusNotSubscribed, res.GetResults()[0].GetStatus())
	assert.Empty(t, f.hookMessages)
}

func TestNotifyUsersOnlyReachesUsersWhoShareAGroup(t *testing.T) {
	f := newNotifyFixture(t)
	f.subscribe(t, f.carol)
	ctx := as(f.alice, "alice", systemroles.PermissionNotificationsSend)
	for _, recipients := range [][]int32{{int32(f.carol)}, {int32(f.bob), int32(f.carol)}, {9999}} {
		_, err := notify(ctx, f.srv, &sickrockpb.NotifyUsersRequest{RecipientUserIds: recipients, Title: "Hi"})
		assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err), "recipients %v", recipients)
	}
	assert.Empty(t, f.hookMessages, "nothing is sent when any recipient is refused")

	users, err := f.srv.ListNotifiableUsers(ctx, connect.NewRequest(&sickrockpb.ListNotifiableUsersRequest{}))
	require.NoError(t, err)
	require.Len(t, users.Msg.GetUsers(), 1)
	assert.Equal(t, "bob", users.Msg.GetUsers()[0].GetUsername())
}

func TestNotifyUsersNeedsPermission(t *testing.T) {
	f := newNotifyFixture(t)
	ctx := as(f.alice, "alice", systemroles.PermissionTableView)
	_, err := notify(ctx, f.srv, &sickrockpb.NotifyUsersRequest{RecipientUserIds: []int32{int32(f.bob)}, Title: "Hi"})
	assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
	_, err = f.srv.ListNotifiableUsers(ctx, connect.NewRequest(&sickrockpb.ListNotifiableUsersRequest{}))
	assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
	_, err = notify(context.Background(), f.srv, &sickrockpb.NotifyUsersRequest{RecipientUserIds: []int32{int32(f.bob)}, Title: "Hi"})
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

func TestNotifyUsersValidation(t *testing.T) {
	f := newNotifyFixture(t)
	ctx := as(f.alice, "alice", systemroles.PermissionNotificationsSend)
	bob := []int32{int32(f.bob)}
	many := make([]int32, NotifyMaxRecipients+1)
	for i := range many {
		many[i] = int32(1000 + i)
	}
	for name, req := range map[string]*sickrockpb.NotifyUsersRequest{
		"no recipients":     {Title: "Hi"},
		"self":              {RecipientUserIds: []int32{int32(f.alice)}, Title: "Hi"},
		"bad id":            {RecipientUserIds: []int32{0}, Title: "Hi"},
		"too many":          {RecipientUserIds: many, Title: "Hi"},
		"no title":          {RecipientUserIds: bob, Title: "   "},
		"long title":        {RecipientUserIds: bob, Title: strings.Repeat("x", NotifyMaxTitleLength+1)},
		"title newline":     {RecipientUserIds: bob, Title: "a\nb"},
		"long message":      {RecipientUserIds: bob, Title: "Hi", Message: strings.Repeat("x", NotifyMaxMessageLength+1)},
		"message control":   {RecipientUserIds: bob, Title: "Hi", Message: "a\x07b"},
		"other host":        {RecipientUserIds: bob, Title: "Hi", LinkPath: "//evil.test/x"},
		"absolute url":      {RecipientUserIds: bob, Title: "Hi", LinkPath: "https://evil.test/x"},
		"relative":          {RecipientUserIds: bob, Title: "Hi", LinkPath: "table/x"},
		"backslash":         {RecipientUserIds: bob, Title: "Hi", LinkPath: `/\evil.test`},
		"newline in link":   {RecipientUserIds: bob, Title: "Hi", LinkPath: "/table/x\nhttps://evil.test"},
		"space in link":     {RecipientUserIds: bob, Title: "Hi", LinkPath: "/table/x y"},
		"javascript scheme": {RecipientUserIds: bob, Title: "Hi", LinkPath: "javascript:alert(1)"},
	} {
		_, err := notify(ctx, f.srv, req)
		assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err), name)
	}

	// At the limits is fine.
	_, err := notify(ctx, f.srv, &sickrockpb.NotifyUsersRequest{
		RecipientUserIds: bob, Title: strings.Repeat("é", NotifyMaxTitleLength), Message: strings.Repeat("x", NotifyMaxMessageLength), LinkPath: "/",
	})
	require.NoError(t, err)
}

func TestNotifyUsersRateLimit(t *testing.T) {
	f := newNotifyFixture(t)
	now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	f.srv.notifyLimiter.now = func() time.Time { return now }
	ctx := as(f.alice, "alice", systemroles.PermissionNotificationsSend)
	req := &sickrockpb.NotifyUsersRequest{RecipientUserIds: []int32{int32(f.bob)}, Title: "Hi"}
	for i := range NotifyRateLimit {
		_, err := notify(ctx, f.srv, req)
		require.NoError(t, err, "send %d", i+1)
	}
	_, err := notify(ctx, f.srv, req)
	assert.Equal(t, connect.CodeResourceExhausted, connect.CodeOf(err))

	// Others are not limited by alice's sends.
	_, err = notify(as(f.bob, "bob", systemroles.PermissionNotificationsSend), f.srv, &sickrockpb.NotifyUsersRequest{RecipientUserIds: []int32{int32(f.alice)}, Title: "Hi"})
	require.NoError(t, err)

	now = now.Add(NotifyRateWindow + time.Second)
	_, err = notify(ctx, f.srv, req)
	require.NoError(t, err, "the window has passed")
}

func TestWithRequestOrigin(t *testing.T) {
	for name, tc := range map[string]struct {
		host    string
		headers map[string]string
		want    string
	}{
		"plain":            {host: "sr.test:8080", want: "http://sr.test:8080"},
		"behind a proxy":   {host: "10.0.0.2:8080", headers: map[string]string{"X-Forwarded-Proto": "https", "X-Forwarded-Host": "stock.example.com"}, want: "https://stock.example.com"},
		"proxy list":       {host: "x", headers: map[string]string{"X-Forwarded-Proto": "https, http", "X-Forwarded-Host": "a.test, b.test"}, want: "https://a.test"},
		"bad proto":        {host: "sr.test", headers: map[string]string{"X-Forwarded-Proto": "javascript"}, want: "http://sr.test"},
		"host with a path": {host: "sr.test", headers: map[string]string{"X-Forwarded-Host": "evil.test/x"}, want: ""},
		"userinfo":         {host: "sr.test", headers: map[string]string{"X-Forwarded-Host": "a@evil.test"}, want: ""},
	} {
		var got string
		handler := WithRequestOrigin(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			got = RequestOrigin(r.Context())
		}))
		req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/x", nil)
		req.Host = tc.host
		for k, v := range tc.headers {
			req.Header.Set(k, v)
		}
		handler.ServeHTTP(httptest.NewRecorder(), req)
		assert.Equal(t, tc.want, got, name)
	}
}
