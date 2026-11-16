package iam

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"connectrpc.com/authn"
	"connectrpc.com/connect"
	"github.com/jamesread/armature-iam/rbac"
	armstore "github.com/jamesread/armature-iam/store"
	iamsqlite "github.com/jamesread/armature-iam/store/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sickrockpbconnect "github.com/jamesread/SickRock/gen/sickrockpbconnect"
	"github.com/jamesread/SickRock/internal/testdb"
	armlayer "github.com/jamesread/armature-iam/layer"
)

func TestAPIKeyScopeAllows(t *testing.T) {
	var none *APIKeyScope
	assert.True(t, none.AllowsPermission("table.view"))
	assert.True(t, none.AllowsResource(ResourceTypeTable, "recipes"))

	open := &APIKeyScope{Permissions: []string{"x"}, Resources: []APIKeyResource{{ResourceTypeTable, "a"}}}
	assert.True(t, open.AllowsPermission("table.view"), "unrestricted ignores the lists")
	assert.True(t, open.AllowsResource(ResourceTypeTable, "recipes"))

	scoped := &APIKeyScope{
		RestrictPermissions: true, Permissions: []string{"table.view"},
		RestrictResources: true, Resources: []APIKeyResource{{ResourceTypeTable, "recipes"}},
	}
	assert.True(t, scoped.AllowsPermission("table.view"))
	assert.False(t, scoped.AllowsPermission("table.edit"))
	assert.True(t, scoped.AllowsResource(ResourceTypeTable, "recipes"))
	assert.False(t, scoped.AllowsResource(ResourceTypeTable, "shopping"))
	assert.False(t, scoped.AllowsResource(ResourceTypeDashboard, "recipes"), "types are not interchangeable")
}

func TestNarrowRBAC(t *testing.T) {
	member := &rbac.EffectiveRBAC{Permissions: map[string]bool{"app.read": true, "table.view": true, "table.edit": true}}
	scope := &APIKeyScope{RestrictPermissions: true, Permissions: []string{"table.view", "system.settings"}}

	got := NarrowRBAC(member, scope)
	assert.True(t, got.Has("table.view"))
	assert.False(t, got.Has("table.edit"))
	assert.False(t, got.Has("system.settings"), "a key never has more than its owner")

	super := &rbac.EffectiveRBAC{IsSuperuser: true, Permissions: map[string]bool{"table.view": true, "system.settings": true, "users.view": true}}
	got = NarrowRBAC(super, scope)
	assert.False(t, got.IsSuperuser)
	assert.True(t, got.Has("system.settings"))
	assert.False(t, got.Has("users.view"))

	assert.Same(t, member, NarrowRBAC(member, &APIKeyScope{}), "an unrestricted scope keeps the owner's RBAC")
	assert.Same(t, member, NarrowRBAC(member, nil))
}

func TestIsReadProcedure(t *testing.T) {
	assert.True(t, IsReadProcedure(sickrockpbconnect.SickRockListItemsProcedure))
	assert.True(t, IsReadProcedure(sickrockpbconnect.SickRockGetItemProcedure))
	assert.False(t, IsReadProcedure(sickrockpbconnect.SickRockCreateItemProcedure))
	assert.False(t, IsReadProcedure(sickrockpbconnect.SickRockSetUserDefaultTableViewProcedure))
	assert.False(t, IsReadProcedure(sickrockpbconnect.SickRockSaveRssCalendarFeedProcedure))
}

func TestAuthorizeAPIKeyProcedure(t *testing.T) {
	scope := &APIKeyScope{RestrictPermissions: true, Permissions: []string{PermissionAppRead}}

	assert.NoError(t, AuthorizeAPIKeyProcedure(sickrockpbconnect.SickRockListItemsProcedure, false, scope))
	assertDenied(t, AuthorizeAPIKeyProcedure(sickrockpbconnect.SickRockCreateItemProcedure, false, scope), "app.access")
	assertDenied(t, AuthorizeAPIKeyProcedure(sickrockpbconnect.SickRockCreateItemProcedure, true, nil), "read-only")
	assertDenied(t, AuthorizeAPIKeyProcedure(sickrockpbconnect.SickRockCreateAPIKeyProcedure, false, &APIKeyScope{}), "cannot manage API keys")
	assert.NoError(t, AuthorizeAPIKeyProcedure(sickrockpbconnect.SickRockPingProcedure, true, scope), "allow-listed procedures always pass")
	assert.NoError(t, AuthorizeAPIKeyProcedure(sickrockpbconnect.SickRockCreateItemProcedure, false, nil), "sessions are not limited here")
}

func assertDenied(t *testing.T, err error, contains string) {
	t.Helper()
	require.Error(t, err)
	assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
	assert.Contains(t, err.Error(), contains)
}

// newKeyDB returns a migrated database with an API key for the seeded admin (user 1).
func newKeyDB(t *testing.T) (*testKeyDB, int) {
	t.Helper()
	db := testdb.NewMigratedSQLite(t)
	_, err := db.ExecContext(t.Context(), `INSERT INTO user_accounts (id, username, password_hash, created_by) VALUES (1, 'admin', '', 'test'), (2, 'other', '', 'test')`)
	require.NoError(t, err)
	st := iamsqlite.New(db.DB)
	keyID, err := st.CreateAPIKey(t.Context(), 1, "ci", "sk_test", false)
	require.NoError(t, err)
	return &testKeyDB{db: db.DB, store: st}, keyID
}

type testKeyDB struct {
	db    *sql.DB
	store armstore.Store
}

func TestUpdateAPIKeyRoundTrip(t *testing.T) {
	tdb, keyID := newKeyDB(t)
	db := tdb.db
	ctx := t.Context()

	details, err := LoadAPIKeyDetails(ctx, db, keyID)
	require.NoError(t, err)
	assert.False(t, details.Scope.RestrictPermissions)
	assert.Zero(t, details.ExpiresAt)

	name, readOnly, expires := "renamed", true, int64(2000000000)
	require.NoError(t, UpdateAPIKey(ctx, db, keyID, 1, APIKeyUpdate{
		Name: &name, ReadOnly: &readOnly, ExpiresAt: &expires,
		Scope: &APIKeyScope{
			RestrictPermissions: true, Permissions: []string{"table.view", "app.read", "table.view"},
			RestrictResources: true, Resources: []APIKeyResource{{ResourceTypeTable, "recipes"}, {ResourceTypeWorkflow, "3"}},
		},
	}))

	keys, err := tdb.store.ListAPIKeysForUser(ctx, 1)
	require.NoError(t, err)
	require.Len(t, keys, 1)
	assert.Equal(t, "renamed", keys[0].Name)
	assert.True(t, keys[0].ReadOnly)

	details, err = LoadAPIKeyDetails(ctx, db, keyID)
	require.NoError(t, err)
	assert.Equal(t, expires, details.ExpiresAt)
	assert.Equal(t, []string{"app.read", "table.view"}, details.Scope.Permissions)
	assert.Equal(t, []APIKeyResource{{ResourceTypeTable, "recipes"}, {ResourceTypeWorkflow, "3"}}, details.Scope.Resources)

	// Clearing the restrictions removes the lists; leaving fields nil keeps them.
	zero := int64(0)
	require.NoError(t, UpdateAPIKey(ctx, db, keyID, 1, APIKeyUpdate{ExpiresAt: &zero, Scope: &APIKeyScope{}}))
	details, err = LoadAPIKeyDetails(ctx, db, keyID)
	require.NoError(t, err)
	assert.Zero(t, details.ExpiresAt)
	assert.Empty(t, details.Scope.Permissions)
	assert.Empty(t, details.Scope.Resources)
	keys, err = tdb.store.ListAPIKeysForUser(ctx, 1)
	require.NoError(t, err)
	assert.Equal(t, "renamed", keys[0].Name)
}

func TestUpdateAPIKeyRefusesBadInput(t *testing.T) {
	tdb, keyID := newKeyDB(t)
	db := tdb.db
	ctx := t.Context()

	var invalid *InvalidScopeError
	err := UpdateAPIKey(ctx, db, keyID, 1, APIKeyUpdate{Scope: &APIKeyScope{RestrictPermissions: true, Permissions: []string{"tabel.view"}}})
	require.ErrorAs(t, err, &invalid)
	err = UpdateAPIKey(ctx, db, keyID, 1, APIKeyUpdate{Scope: &APIKeyScope{RestrictResources: true, Resources: []APIKeyResource{{"page", "x"}}}})
	require.ErrorAs(t, err, &invalid)

	details, err := LoadAPIKeyDetails(ctx, db, keyID)
	require.NoError(t, err)
	assert.False(t, details.Scope.RestrictPermissions, "a refused update changes nothing")

	name := "mine"
	require.ErrorIs(t, UpdateAPIKey(ctx, db, keyID, 2, APIKeyUpdate{Name: &name}), ErrAPIKeyNotFound, "another user's key")
	require.ErrorIs(t, UpdateAPIKey(ctx, db, keyID+100, 1, APIKeyUpdate{Name: &name}), ErrAPIKeyNotFound)
}

// scopeRequest runs applyAPIKeyScope for a request authenticated as the admin with key sk_test.
func scopeRequest(t *testing.T, tdb *testKeyDB, procedure string, readOnly bool) (*armlayer.AuthenticatedUser, context.Context, error) {
	t.Helper()
	au := &armlayer.AuthenticatedUser{
		User:     &armstore.UserAccountRow{ID: 1, Username: "admin"},
		RBAC:     &rbac.EffectiveRBAC{IsSuperuser: true, Permissions: map[string]bool{"app.read": true, "app.access": true, "table.view": true}},
		ReadOnly: readOnly,
	}
	req := httptest.NewRequestWithContext(authn.SetInfo(t.Context(), au), http.MethodPost, procedure, nil)
	req.Header.Set("Authorization", "Bearer sk_test")
	ctx, err := applyAPIKeyScope(req, tdb.db)
	return au, ctx, err
}

func TestApplyAPIKeyScope(t *testing.T) {
	tdb, keyID := newKeyDB(t)
	db := tdb.db
	ctx := t.Context()

	au, reqCtx, err := scopeRequest(t, tdb, sickrockpbconnect.SickRockCreateItemProcedure, false)
	require.NoError(t, err)
	assert.True(t, au.RBAC.IsSuperuser, "an unrestricted key keeps its owner's RBAC")
	require.NotNil(t, APIKeyScopeFromContext(reqCtx))
	assert.True(t, APIKeyScopeFromContext(reqCtx).OwnerIsSuperuser)

	_, _, err = scopeRequest(t, tdb, sickrockpbconnect.SickRockCreateItemProcedure, true)
	assertDenied(t, err, "read-only")

	require.NoError(t, UpdateAPIKey(ctx, db, keyID, 1, APIKeyUpdate{Scope: &APIKeyScope{RestrictPermissions: true, Permissions: []string{"app.read", "table.view"}}}))
	au, _, err = scopeRequest(t, tdb, sickrockpbconnect.SickRockListItemsProcedure, false)
	require.NoError(t, err)
	assert.False(t, au.RBAC.IsSuperuser)
	assert.True(t, au.RBAC.Has("table.view"))
	assert.False(t, au.RBAC.Has("app.access"))
	_, _, err = scopeRequest(t, tdb, sickrockpbconnect.SickRockCreateItemProcedure, false)
	assertDenied(t, err, "app.access")

	past := time.Now().Add(-time.Minute).Unix()
	require.NoError(t, UpdateAPIKey(ctx, db, keyID, 1, APIKeyUpdate{ExpiresAt: &past}))
	_, _, err = scopeRequest(t, tdb, sickrockpbconnect.SickRockListItemsProcedure, false)
	require.Error(t, err)
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

func TestApplyAPIKeyScopeIgnoresSessions(t *testing.T) {
	tdb, _ := newKeyDB(t)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, sickrockpbconnect.SickRockCreateItemProcedure, nil)
	req.Header.Set("Authorization", "Bearer 0b5e-session-id")
	ctx, err := applyAPIKeyScope(req, tdb.db)
	require.NoError(t, err)
	assert.Nil(t, APIKeyScopeFromContext(ctx))
}
