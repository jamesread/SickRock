package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"connectrpc.com/authn"
	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sickrockpb "github.com/jamesread/SickRock/gen/proto"
	"github.com/jamesread/SickRock/internal/repo"
	"github.com/jamesread/SickRock/internal/testdb"
	"github.com/jamesread/SickRock/internal/uiusage"
)

func newUIUsageTestServer(t *testing.T) *SickRockServer {
	t.Helper()
	db := testdb.NewMigratedSQLite(t)
	_, err := db.ExecContext(t.Context(), `INSERT INTO user_accounts (id, username, password_hash, created_by) VALUES (1, 'alice', '', 'test'), (2, 'bob', '', 'test')`)
	require.NoError(t, err)
	srv := NewSickRockServer(repo.NewRepository(db), nil)
	store, err := uiusage.NewStore(t.TempDir())
	require.NoError(t, err)
	srv.SetUIUsageStore(store)
	return srv
}

func userContext(id int, name string) context.Context {
	au := authUserWith(false)
	au.User.ID = id
	au.User.Username = name
	return authn.SetInfo(context.Background(), au)
}

func recordRoute(t *testing.T, ctx context.Context, srv *SickRockServer, to string) *sickrockpb.RecordUIUsageResponse {
	t.Helper()
	res, err := srv.RecordUIUsage(ctx, connect.NewRequest(&sickrockpb.RecordUIUsageRequest{
		Events: []*sickrockpb.UIUsageEvent{{Kind: "route", FromPath: "/", ToPath: to, AppVersion: "1.2.3"}},
	}))
	require.NoError(t, err)
	return res.Msg
}

func download(srv *SickRockServer, ctx context.Context) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	srv.UIUsageLogHandler().ServeHTTP(rec, httptest.NewRequestWithContext(ctx, http.MethodGet, "/ui-usage.jsonl", nil))
	return rec
}

func TestUIUsageNeedsOptIn(t *testing.T) {
	srv := newUIUsageTestServer(t)
	alice := userContext(1, "alice")

	res := recordRoute(t, alice, srv, "/table/pantry")
	assert.False(t, res.GetTrackingEnabled(), "the client is told to stop")
	assert.Zero(t, res.GetAccepted())

	_, err := srv.SetUIUsageTracking(alice, connect.NewRequest(&sickrockpb.SetUIUsageTrackingRequest{Enabled: true}))
	require.NoError(t, err)
	res = recordRoute(t, alice, srv, "/table/pantry")
	assert.True(t, res.GetTrackingEnabled())
	assert.Equal(t, int32(1), res.GetAccepted())

	settings, err := srv.GetUIUsageSettings(alice, connect.NewRequest(&sickrockpb.GetUIUsageSettingsRequest{}))
	require.NoError(t, err)
	assert.True(t, settings.Msg.GetTrackingEnabled())
	assert.Equal(t, int32(1), settings.Msg.GetLineCount())
	assert.Equal(t, int32(uiusage.MaxLines), settings.Msg.GetMaxLines())
}

func TestUIUsageLogDownloadIsYourOwn(t *testing.T) {
	srv := newUIUsageTestServer(t)
	alice, bob := userContext(1, "alice"), userContext(2, "bob")
	for _, ctx := range []context.Context{alice, bob} {
		_, err := srv.SetUIUsageTracking(ctx, connect.NewRequest(&sickrockpb.SetUIUsageTrackingRequest{Enabled: true}))
		require.NoError(t, err)
	}
	recordRoute(t, alice, srv, "/alice-page")
	recordRoute(t, bob, srv, "/bob-page")

	rec := download(srv, alice)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/x-ndjson", rec.Header().Get("Content-Type"))
	assert.Contains(t, rec.Header().Get("Content-Disposition"), "sickrock-ui-usage-alice.jsonl")
	assert.Contains(t, rec.Body.String(), `"to":"/alice-page"`)
	assert.Contains(t, rec.Body.String(), `"version":"1.2.3"`)
	assert.NotContains(t, rec.Body.String(), "bob-page")

	assert.Equal(t, http.StatusUnauthorized, download(srv, context.Background()).Code)

	_, err := srv.ClearUIUsageLog(alice, connect.NewRequest(&sickrockpb.ClearUIUsageLogRequest{}))
	require.NoError(t, err)
	assert.Empty(t, strings.TrimSpace(download(srv, alice).Body.String()))
	assert.Contains(t, download(srv, bob).Body.String(), "bob-page")
}

func TestUIUsageWithoutStore(t *testing.T) {
	srv := newUIUsageTestServer(t)
	srv.SetUIUsageStore(nil)
	_, err := srv.SetUIUsageTracking(userContext(1, "alice"), connect.NewRequest(&sickrockpb.SetUIUsageTrackingRequest{Enabled: true}))
	assert.Equal(t, connect.CodeUnavailable, connect.CodeOf(err))
	_, err = srv.SetUIUsageTracking(userContext(1, "alice"), connect.NewRequest(&sickrockpb.SetUIUsageTrackingRequest{Enabled: false}))
	require.NoError(t, err, "turning it off always works")
}
