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

func TestSetTableDefaultView(t *testing.T) {
	db := testdb.NewMigratedSQLite(t)
	_, err := db.ExecContext(t.Context(), `
		CREATE TABLE meals (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT);
		CREATE TABLE garden (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT);
		INSERT INTO table_configurations (name, title, db, "table") VALUES ('meals', 'Meals', 'main', 'meals'), ('garden', 'Garden', 'main', 'garden');`)
	require.NoError(t, err)
	r := repo.NewRepository(db)
	srv := NewSickRockServer(r, nil)
	ctx := authn.SetInfo(context.Background(), authUserWith(true))
	tableView, err := r.CreateTableView(t.Context(), "meals", "All", "table", "", nil)
	require.NoError(t, err)
	weekView, err := r.CreateTableView(t.Context(), "meals", "Week", "weeklyplan", "", nil)
	require.NoError(t, err)
	gardenView, err := r.CreateTableView(t.Context(), "garden", "Beds", "table", "", nil)
	require.NoError(t, err)

	set := func(ctx context.Context, viewID int) error {
		_, err := srv.SetTableDefaultView(ctx, connect.NewRequest(&sickrockpb.SetTableDefaultViewRequest{TableName: "meals", ViewId: int32(viewID)}))
		return err
	}
	defaults := func(table string) []int {
		var ids []int
		require.NoError(t, db.Select(&ids, `SELECT id FROM table_views WHERE table_name = ? AND is_default ORDER BY id`, table))
		return ids
	}
	_, err = db.ExecContext(t.Context(), `UPDATE table_views SET is_default = 1 WHERE id IN (?, ?)`, tableView, gardenView)
	require.NoError(t, err)

	require.NoError(t, set(ctx, weekView))
	assert.Equal(t, []int{weekView}, defaults("meals"), "a weekly plan view can be the default, and only one is")
	assert.Equal(t, "weeklyplan", srv.defaultViewType(ctx, "meals"), "the table opens on it")
	assert.Equal(t, []int{gardenView}, defaults("garden"), "other tables are untouched")

	require.NoError(t, set(ctx, 0))
	assert.Empty(t, defaults("meals"), "0 clears it")

	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(set(ctx, gardenView)), "another table's view")
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(set(ctx, -1)))
	assert.Error(t, set(authn.SetInfo(context.Background(), authUserWith(false)), weekView), "needs edit access to the table")
	assert.Empty(t, defaults("meals"))
}
