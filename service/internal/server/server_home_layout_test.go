package server

import (
	"context"
	"testing"

	"connectrpc.com/authn"
	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sickrockpb "github.com/jamesread/SickRock/gen/proto"
	"github.com/jamesread/SickRock/internal/repo"
	"github.com/jamesread/SickRock/internal/testdb"
)

func homeLayoutContext(userID int) context.Context {
	au := authUserWith(false)
	au.User.ID = userID
	return authn.SetInfo(context.Background(), au)
}

func TestHomeLayoutIsPerUser(t *testing.T) {
	db := testdb.NewMigratedSQLite(t)
	_, err := db.ExecContext(t.Context(), `INSERT INTO user_accounts (id, username, password_hash, created_by) VALUES (1, 'alice', '', 'test'), (2, 'bob', '', 'test')`)
	require.NoError(t, err)
	srv := NewSickRockServer(repo.NewRepository(db), nil)
	alice, bob := homeLayoutContext(1), homeLayoutContext(2)

	get := func(ctx context.Context) string {
		res, err := srv.GetHomeLayout(ctx, connect.NewRequest(&sickrockpb.GetHomeLayoutRequest{}))
		require.NoError(t, err)
		return res.Msg.GetLayoutJson()
	}
	assert.Empty(t, get(alice), "the default layout")

	layout := `{"sections":[{"id":"recently-viewed","visible":true}],"quickAccess":[3]}`
	_, err = srv.SetHomeLayout(alice, connect.NewRequest(&sickrockpb.SetHomeLayoutRequest{LayoutJson: layout}))
	require.NoError(t, err)
	assert.Equal(t, layout, get(alice))
	assert.Empty(t, get(bob))

	_, err = srv.SetHomeLayout(alice, connect.NewRequest(&sickrockpb.SetHomeLayoutRequest{LayoutJson: ""}))
	require.NoError(t, err)
	assert.Empty(t, get(alice), "empty resets to the default")
}

func TestSetHomeLayoutRefusesNonObjects(t *testing.T) {
	db := testdb.NewMigratedSQLite(t)
	_, err := db.ExecContext(t.Context(), `INSERT INTO user_accounts (id, username, password_hash, created_by) VALUES (1, 'alice', '', 'test')`)
	require.NoError(t, err)
	srv := NewSickRockServer(repo.NewRepository(db), nil)
	for _, bad := range []string{"not json", "[1,2]", "null", `{"x":"` + string(make([]byte, maxHomeLayoutBytes)) + `"}`} {
		_, err := srv.SetHomeLayout(homeLayoutContext(1), connect.NewRequest(&sickrockpb.SetHomeLayoutRequest{LayoutJson: bad}))
		assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err), bad[:min(len(bad), 10)])
	}
}
