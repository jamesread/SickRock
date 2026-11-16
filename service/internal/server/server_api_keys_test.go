package server

import (
	"context"
	"database/sql"
	"testing"

	"connectrpc.com/authn"
	"connectrpc.com/connect"
	iamsqlite "github.com/jamesread/armature-iam/store/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sickrockpb "github.com/jamesread/SickRock/gen/proto"
	"github.com/jamesread/SickRock/internal/iam"
	"github.com/jamesread/SickRock/internal/repo"
	"github.com/jamesread/SickRock/internal/testdb"
)

// newAPIKeyTestServer returns a server with users 1 and 2 and two data tables.
func newAPIKeyTestServer(t *testing.T) *SickRockServer {
	t.Helper()
	db := testdb.NewMigratedSQLite(t)
	_, err := db.ExecContext(t.Context(), `
		INSERT INTO user_accounts (id, username, password_hash, created_by) VALUES (1, 'admin', '', 'test'), (2, 'other', '', 'test');
		CREATE TABLE recipes (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT);
		CREATE TABLE shopping (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT);
		INSERT INTO table_configurations (name, title, db, "table") VALUES ('recipes', 'Recipes', 'main', 'recipes'), ('shopping', 'Shopping', 'main', 'shopping');`)
	require.NoError(t, err)
	auth, err := iam.NewAuthLayer(iamsqlite.New(db.DB), db.DB, nil)
	require.NoError(t, err)
	return NewSickRockServer(repo.NewRepository(db), auth)
}

func listKeys(t *testing.T, ctx context.Context, srv *SickRockServer) *sickrockpb.GetAPIKeysResponse {
	t.Helper()
	res, err := srv.GetAPIKeys(ctx, connect.NewRequest(&sickrockpb.GetAPIKeysRequest{}))
	require.NoError(t, err)
	return res.Msg
}

func TestAPIKeyCreateEditDelete(t *testing.T) {
	srv := newAPIKeyTestServer(t)
	ctx := authn.SetInfo(context.Background(), authUserWith(true, "table.view", "app.read"))

	created, err := srv.CreateAPIKey(ctx, connect.NewRequest(&sickrockpb.CreateAPIKeyRequest{
		Name:      "ci",
		ExpiresAt: 4000000000,
		Scope: &sickrockpb.APIKeyScope{
			RestrictResources: true,
			Resources:         []*sickrockpb.APIKeyResource{{Type: iam.ResourceTypeTable, Key: "recipes"}},
		},
	}))
	require.NoError(t, err)
	keyID := created.Msg.GetApiKeyId()

	list := listKeys(t, ctx, srv)
	require.Len(t, list.GetApiKeys(), 1)
	key := list.GetApiKeys()[0]
	assert.Equal(t, int64(4000000000), key.GetExpiresAt())
	assert.True(t, key.GetIsActive())
	assert.True(t, key.GetScope().GetRestrictResources())
	assert.Equal(t, "recipes", key.GetScope().GetResources()[0].GetKey())
	assert.Equal(t, []string{"app.read", "table.view"}, list.GetAvailablePermissions())
	var tables []string
	for _, o := range list.GetAvailableResources() {
		if o.GetType() == iam.ResourceTypeTable {
			tables = append(tables, o.GetKey())
		}
	}
	assert.Equal(t, []string{"recipes", "shopping"}, tables, "system tables are not offered")

	name, readOnly, noExpiry := "renamed", true, int64(0)
	_, err = srv.UpdateAPIKey(ctx, connect.NewRequest(&sickrockpb.UpdateAPIKeyRequest{
		ApiKeyId: keyID, Name: &name, ReadOnly: &readOnly, ExpiresAt: &noExpiry,
		Scope: &sickrockpb.APIKeyScope{RestrictPermissions: true, Permissions: []string{"table.view"}},
	}))
	require.NoError(t, err)
	key = listKeys(t, ctx, srv).GetApiKeys()[0]
	assert.Equal(t, "renamed", key.GetName())
	assert.True(t, key.GetReadOnly())
	assert.Zero(t, key.GetExpiresAt())
	assert.True(t, key.GetScope().GetRestrictPermissions())
	assert.False(t, key.GetScope().GetRestrictResources())

	// Only the name: the scope is kept.
	other := "only the name"
	_, err = srv.UpdateAPIKey(ctx, connect.NewRequest(&sickrockpb.UpdateAPIKeyRequest{ApiKeyId: keyID, Name: &other}))
	require.NoError(t, err)
	key = listKeys(t, ctx, srv).GetApiKeys()[0]
	assert.Equal(t, "only the name", key.GetName())
	assert.Equal(t, []string{"table.view"}, key.GetScope().GetPermissions())

	_, err = srv.DeleteAPIKey(ctx, connect.NewRequest(&sickrockpb.DeleteAPIKeyRequest{ApiKeyId: keyID}))
	require.NoError(t, err)
	assert.Empty(t, listKeys(t, ctx, srv).GetApiKeys())
	var leftover int
	require.NoError(t, srv.repo.DB().GetContext(ctx, &leftover, `SELECT COUNT(*) FROM api_key_permissions`))
	assert.Zero(t, leftover)
}

func TestAPIKeyUpdateErrors(t *testing.T) {
	srv := newAPIKeyTestServer(t)
	ctx := authn.SetInfo(context.Background(), authUserWith(true))
	created, err := srv.CreateAPIKey(ctx, connect.NewRequest(&sickrockpb.CreateAPIKeyRequest{Name: "ci"}))
	require.NoError(t, err)
	keyID := created.Msg.GetApiKeyId()

	_, err = srv.UpdateAPIKey(ctx, connect.NewRequest(&sickrockpb.UpdateAPIKeyRequest{
		ApiKeyId: keyID, Scope: &sickrockpb.APIKeyScope{RestrictPermissions: true, Permissions: []string{"no.such"}},
	}))
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))

	blank := "  "
	_, err = srv.UpdateAPIKey(ctx, connect.NewRequest(&sickrockpb.UpdateAPIKeyRequest{ApiKeyId: keyID, Name: &blank}))
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))

	otherUser := authUserWith(false)
	otherUser.User.ID = 2
	name := "stolen"
	_, err = srv.UpdateAPIKey(authn.SetInfo(context.Background(), otherUser), connect.NewRequest(&sickrockpb.UpdateAPIKeyRequest{ApiKeyId: keyID, Name: &name}))
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))

	_, err = srv.CreateAPIKey(ctx, connect.NewRequest(&sickrockpb.CreateAPIKeyRequest{
		Name: "bad", Scope: &sickrockpb.APIKeyScope{RestrictResources: true, Resources: []*sickrockpb.APIKeyResource{{Type: "page", Key: "x"}}},
	}))
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
	assert.Len(t, listKeys(t, ctx, srv).GetApiKeys(), 1, "a key whose scope was refused is not kept")
}

// keyContext is a request from a superuser's API key with the given scope, after the scope
// middleware has run.
func keyContext(scope iam.APIKeyScope, readOnly bool) context.Context {
	au := authUserWith(true)
	scope.OwnerIsSuperuser = true
	au.RBAC = iam.NarrowRBAC(au.RBAC, &scope)
	au.ReadOnly = readOnly
	return iam.ContextWithAPIKeyScope(authn.SetInfo(context.Background(), au), &scope)
}

func TestAPIKeyScopeLimitsTables(t *testing.T) {
	srv := newAPIKeyTestServer(t)
	ctx := keyContext(iam.APIKeyScope{
		RestrictResources: true, Resources: []iam.APIKeyResource{{Type: iam.ResourceTypeTable, Key: "recipes"}},
	}, false)

	require.NoError(t, srv.requireTableAccess(ctx, "recipes", repo.ResourceActionEdit))
	assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(srv.requireTableAccess(ctx, "shopping", repo.ResourceActionView)))
	assert.True(t, srv.canDiscoverTableConfiguration(ctx, "recipes"))
	assert.False(t, srv.canDiscoverTableConfiguration(ctx, "shopping"))

	_, err := srv.ListItems(ctx, connect.NewRequest(&sickrockpb.ListItemsRequest{TcName: "shopping"}))
	assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
	_, err = srv.ListItems(ctx, connect.NewRequest(&sickrockpb.ListItemsRequest{TcName: "recipes"}))
	require.NoError(t, err)
}

func TestAPIKeyScopeLimitsTableActions(t *testing.T) {
	srv := newAPIKeyTestServer(t)
	ctx := keyContext(iam.APIKeyScope{RestrictPermissions: true, Permissions: []string{"app.read", "table.view"}}, false)

	require.NoError(t, srv.requireTableAccess(ctx, "shopping", repo.ResourceActionView), "the owner is a superuser")
	assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(srv.requireTableAccess(ctx, "shopping", repo.ResourceActionEdit)))

	res, err := srv.GetTableAccess(ctx, connect.NewRequest(&sickrockpb.GetTableAccessRequest{TableName: "shopping"}))
	require.NoError(t, err)
	assert.True(t, res.Msg.GetCanView())
	assert.False(t, res.Msg.GetCanEdit())
	assert.False(t, res.Msg.GetCanInsert())
}

func TestReadOnlyAPIKeyTableAccess(t *testing.T) {
	srv := newAPIKeyTestServer(t)
	res, err := srv.GetTableAccess(keyContext(iam.APIKeyScope{}, true), connect.NewRequest(&sickrockpb.GetTableAccessRequest{TableName: "recipes"}))
	require.NoError(t, err)
	assert.True(t, res.Msg.GetCanView())
	assert.False(t, res.Msg.GetCanEdit())
	assert.False(t, res.Msg.GetCanDelete())
}

func TestAPIKeyScopeLimitsNavigation(t *testing.T) {
	srv := newAPIKeyTestServer(t)
	ctx := keyContext(iam.APIKeyScope{
		RestrictResources: true, Resources: []iam.APIKeyResource{{Type: iam.ResourceTypeTable, Key: "recipes"}},
	}, false)
	recipes := repo.NavigationItem{TableName: sql.NullString{String: "recipes", Valid: true}}
	shopping := repo.NavigationItem{TableName: sql.NullString{String: "shopping", Valid: true}}

	assert.True(t, srv.canViewNavigationItem(ctx, recipes))
	assert.False(t, srv.canViewNavigationItem(ctx, shopping))
	assert.True(t, srv.canViewNavigationItem(authn.SetInfo(context.Background(), authUserWith(true)), shopping), "sessions are not limited")
}
