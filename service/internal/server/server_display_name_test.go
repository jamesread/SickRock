package server

import (
	"context"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sickrockpb "github.com/jamesread/SickRock/gen/proto"
	"github.com/jamesread/SickRock/internal/repo"
	"github.com/jamesread/SickRock/internal/testdb"
)

func TestNormalizeDisplayName(t *testing.T) {
	for in, want := range map[string]string{
		"  Ada Lovelace  ": "Ada Lovelace",
		"":                 "",
		"   ":              "",
		"Zoë 🍝":            "Zoë 🍝",
		strings.Repeat("é", MaxDisplayNameLength): strings.Repeat("é", MaxDisplayNameLength),
	} {
		got, err := normalizeDisplayName(in)
		require.NoError(t, err, in)
		assert.Equal(t, want, got)
	}
	for _, bad := range []string{strings.Repeat("a", MaxDisplayNameLength+1), "Ada\nLovelace", "Ada\x00", "\xff"} {
		_, err := normalizeDisplayName(bad)
		assert.Error(t, err, "%q", bad)
	}
}

func TestDisplayNameIsPerUserAndShownByInit(t *testing.T) {
	db := testdb.NewMigratedSQLite(t)
	_, err := db.ExecContext(t.Context(), `INSERT INTO user_accounts (id, username, password_hash, created_by) VALUES (1, 'alice', '', 'test'), (2, 'bob', '', 'test')`)
	require.NoError(t, err)
	srv := NewSickRockServer(repo.NewRepository(db), nil)
	alice, bob := homeLayoutContext(1), homeLayoutContext(2)

	initName := func(ctx context.Context) string {
		res, err := srv.Init(ctx, connect.NewRequest(&sickrockpb.InitRequest{}))
		require.NoError(t, err)
		return res.Msg.GetCurrentDisplayName()
	}
	assert.Empty(t, initName(alice), "unset: the header shows the username")

	res, err := srv.SetDisplayName(alice, connect.NewRequest(&sickrockpb.SetDisplayNameRequest{DisplayName: "  Alice Smith "}))
	require.NoError(t, err)
	assert.Equal(t, "Alice Smith", res.Msg.GetDisplayName())
	assert.Equal(t, "Alice Smith", initName(alice))
	assert.Empty(t, initName(bob))

	_, err = srv.SetDisplayName(alice, connect.NewRequest(&sickrockpb.SetDisplayNameRequest{DisplayName: " "}))
	require.NoError(t, err)
	assert.Empty(t, initName(alice), "blank clears it")
	stored, err := repo.NewRepository(db).DisplayName(t.Context(), 1)
	require.NoError(t, err)
	assert.Empty(t, stored)
}

func TestSetDisplayNameValidatesAndNeedsSignIn(t *testing.T) {
	db := testdb.NewMigratedSQLite(t)
	_, err := db.ExecContext(t.Context(), `INSERT INTO user_accounts (id, username, password_hash, created_by) VALUES (1, 'alice', '', 'test')`)
	require.NoError(t, err)
	srv := NewSickRockServer(repo.NewRepository(db), nil)

	_, err = srv.SetDisplayName(homeLayoutContext(1), connect.NewRequest(&sickrockpb.SetDisplayNameRequest{DisplayName: strings.Repeat("x", MaxDisplayNameLength+1)}))
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
	_, err = srv.SetDisplayName(context.Background(), connect.NewRequest(&sickrockpb.SetDisplayNameRequest{DisplayName: "Eve"}))
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}
