package server

import (
	"context"
	"testing"

	"connectrpc.com/authn"
	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sickrockpb "github.com/jamesread/SickRock/gen/proto"
	"github.com/jamesread/SickRock/gen/sickrockpbconnect"
	"github.com/jamesread/SickRock/internal/iam"
	"github.com/jamesread/SickRock/internal/repo"
	"github.com/jamesread/SickRock/internal/testdb"
)

// newRollupTestServer has a rollup (id 1) of the ingredients of planned recipes into a list.
func newRollupTestServer(t *testing.T) (*SickRockServer, func(query string) int) {
	t.Helper()
	db := testdb.NewMigratedSQLite(t)
	_, err := db.ExecContext(t.Context(), `
		INSERT INTO table_configurations (name, title, db, "table") VALUES
			('plan', 'Plan', 'main', 'plan'), ('parts', 'Parts', 'main', 'parts'), ('list', 'List', 'main', 'list');
		CREATE TABLE plan (id INTEGER PRIMARY KEY AUTOINCREMENT, day TEXT, recipe INTEGER);
		CREATE TABLE parts (id INTEGER PRIMARY KEY AUTOINCREMENT, recipe INTEGER, item TEXT, qty TEXT);
		CREATE TABLE list (id INTEGER PRIMARY KEY AUTOINCREMENT, item TEXT, qty REAL);
		INSERT INTO plan (day, recipe) VALUES ('2026-11-23', 1), ('2026-11-24', 1), ('2026-12-01', 1);
		INSERT INTO parts (recipe, item, qty) VALUES (1, 'Rice', '0.5');
		INSERT INTO list (item) VALUES ('Stale');
		INSERT INTO rollups (id, name, source_table, source_link_column, detail_table, detail_link_column,
			group_columns_json, sum_columns_json, target_table)
		VALUES (1, 'List', 'plan', 'recipe', 'parts', 'recipe', '["item"]', '["qty"]', 'list');`)
	require.NoError(t, err)
	count := func(query string) int {
		var n int
		require.NoError(t, db.Get(&n, query))
		return n
	}
	return NewSickRockServer(repo.NewRepository(db), nil), count
}

func runRollup(ctx context.Context, srv *SickRockServer, msg *sickrockpb.RunRollupRequest) (*sickrockpb.RunRollupResponse, error) {
	res, err := srv.RunRollup(ctx, connect.NewRequest(msg))
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}

func TestRunRollup(t *testing.T) {
	srv, count := newRollupTestServer(t)
	admin := authn.SetInfo(context.Background(), authUserWith(true))

	res, err := runRollup(admin, srv, &sickrockpb.RunRollupRequest{RollupId: 1, DateColumn: "day", From: "2026-11-23", To: "2026-11-30"})
	require.NoError(t, err)
	assert.Equal(t, "list", res.GetTargetTable())
	assert.Equal(t, int64(2), res.GetSourceRows())
	assert.Equal(t, int64(1), res.GetRowsWritten())
	assert.Equal(t, 1, count(`SELECT COUNT(*) FROM list WHERE item = 'Rice' AND qty = 1`))
	assert.Equal(t, 0, count(`SELECT COUNT(*) FROM list WHERE item = 'Stale'`), "the target's old rows are replaced")
}

func TestRunRollupRefuses(t *testing.T) {
	srv, count := newRollupTestServer(t)
	admin := authn.SetInfo(context.Background(), authUserWith(true))
	cases := []struct {
		ctx  context.Context
		msg  *sickrockpb.RunRollupRequest
		name string
		code connect.Code
	}{
		{name: "signed out", ctx: context.Background(), msg: &sickrockpb.RunRollupRequest{RollupId: 1}, code: connect.CodeUnauthenticated},
		{name: "no rollup.run", ctx: authn.SetInfo(context.Background(), authUserWith(false, "table.view", "table.insert", "table.delete")),
			msg: &sickrockpb.RunRollupRequest{RollupId: 1}, code: connect.CodePermissionDenied},
		{name: "no table grants", ctx: authn.SetInfo(context.Background(), authUserWith(false, iam.PermissionRollupRun)),
			msg: &sickrockpb.RunRollupRequest{RollupId: 1}, code: connect.CodePermissionDenied},
		{name: "unknown rollup", ctx: admin, msg: &sickrockpb.RunRollupRequest{RollupId: 9}, code: connect.CodeNotFound},
		{name: "bad date", ctx: admin, msg: &sickrockpb.RunRollupRequest{RollupId: 1, DateColumn: "day", From: "next week"}, code: connect.CodeInvalidArgument},
		{name: "range without column", ctx: admin, msg: &sickrockpb.RunRollupRequest{RollupId: 1, From: "2026-11-23"}, code: connect.CodeInvalidArgument},
		{name: "unknown filter column", ctx: admin, msg: &sickrockpb.RunRollupRequest{RollupId: 1, Where: map[string]string{"nope": "1"}}, code: connect.CodeFailedPrecondition},
	}
	for _, c := range cases {
		_, err := runRollup(c.ctx, srv, c.msg)
		assert.Equal(t, c.code, connect.CodeOf(err), c.name)
	}
	assert.Equal(t, 1, count(`SELECT COUNT(*) FROM list WHERE item = 'Stale'`), "refused runs leave the target alone")
}

func TestListRollups(t *testing.T) {
	srv, _ := newRollupTestServer(t)
	list := func(ctx context.Context, source string) []*sickrockpb.Rollup {
		res, err := srv.ListRollups(ctx, connect.NewRequest(&sickrockpb.ListRollupsRequest{SourceTable: source}))
		require.NoError(t, err)
		return res.Msg.GetRollups()
	}
	admin := authn.SetInfo(context.Background(), authUserWith(true))
	require.Len(t, list(admin, "plan"), 1)
	assert.Equal(t, "List", list(admin, "")[0].GetName())
	assert.Empty(t, list(admin, "parts"))
	assert.Empty(t, list(authn.SetInfo(context.Background(), authUserWith(false)), ""), "hidden without view access to the source")
}

func TestRunRollupProcedureGates(t *testing.T) {
	assert.Equal(t, iam.PermissionRollupRun, iam.RequiredPermission(sickrockpbconnect.SickRockRunRollupProcedure))
	assert.False(t, iam.IsReadProcedure(sickrockpbconnect.SickRockRunRollupProcedure), "read-only API keys cannot run rollups")
	assert.True(t, isSystemSettingsTableConfiguration(repo.RollupsTableConfiguration), "rollup definitions are admin-only")
}
