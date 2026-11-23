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

func TestDefaultExportView(t *testing.T) {
	db := testdb.NewMigratedSQLite(t)
	_, err := db.ExecContext(t.Context(), `
		CREATE TABLE pantry (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT);
		CREATE TABLE garden (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT);
		INSERT INTO table_configurations (name, title, db, "table") VALUES ('pantry', 'Pantry', 'main', 'pantry'), ('garden', 'Garden', 'main', 'garden');`)
	require.NoError(t, err)
	r := repo.NewRepository(db)
	srv := NewSickRockServer(r, nil)
	ctx := authn.SetInfo(context.Background(), authUserWith(true))
	pantryView, err := r.CreateTableView(t.Context(), "pantry", "Stock", "table", "", nil)
	require.NoError(t, err)
	gardenView, err := r.CreateTableView(t.Context(), "garden", "Beds", "table", "", nil)
	require.NoError(t, err)

	set := func(viewID int) error {
		_, err := srv.SetTableDefaultExportView(ctx, connect.NewRequest(&sickrockpb.SetTableDefaultExportViewRequest{TableConfiguration: "pantry", ViewId: int32(viewID)}))
		return err
	}
	stored := func() int32 {
		res, err := srv.GetTableStructure(ctx, connect.NewRequest(&sickrockpb.GetTableStructureRequest{PageId: "pantry"}))
		require.NoError(t, err)
		return res.Msg.GetDefaultExportViewId()
	}

	assert.Zero(t, stored(), "unset: the table's default view")
	require.NoError(t, set(pantryView))
	assert.Equal(t, int32(pantryView), stored())
	require.NoError(t, set(repo.AllColumnsViewID))
	assert.Equal(t, int32(-1), stored())
	require.NoError(t, set(0))
	assert.Zero(t, stored(), "0 goes back to the default view")

	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(set(gardenView)), "another table's view")
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(set(-7)))
	_, err = srv.SetTableDefaultExportView(ctx, connect.NewRequest(&sickrockpb.SetTableDefaultExportViewRequest{ViewId: 1}))
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
	_, err = srv.SetTableDefaultExportView(authn.SetInfo(context.Background(), authUserWith(false)), connect.NewRequest(&sickrockpb.SetTableDefaultExportViewRequest{TableConfiguration: "pantry", ViewId: int32(pantryView)}))
	assert.Error(t, err, "needs edit access to the table")
	assert.Zero(t, stored())
}
