package repo

import (
	"context"
	"testing"

	"github.com/jamesread/SickRock/internal/testdb"
)

// newMigratedSQLiteRepo returns a repository on a fresh SQLite database with the real migrations applied.
func newMigratedSQLiteRepo(t *testing.T) *Repository {
	t.Helper()
	return NewRepository(testdb.NewMigratedSQLite(t))
}

func TestTableViewLayoutJSONRoundTrip(t *testing.T) {
	r := newMigratedSQLiteRepo(t)
	ctx := context.Background()

	columns := []TableViewColumn{
		{ColumnName: "name", IsVisible: true, ColumnOrder: 0, SortOrder: "asc"},
		{ColumnName: "hidden", IsVisible: false, ColumnOrder: 1},
	}
	id, err := r.CreateTableView(ctx, "table_settings", "Mine", "table", `{"groupBy":"name"}`, columns)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if id <= 0 {
		t.Fatalf("expected new view id, got %d", id)
	}

	views, err := r.GetTableViews(ctx, "table_settings")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	view := findView(t, views, id)
	if view.LayoutJSON != `{"groupBy":"name"}` {
		t.Fatalf("layout_json %q", view.LayoutJSON)
	}
	if len(view.Columns) != 1 || view.Columns[0].ColumnName != "name" || view.Columns[0].SortOrder != "asc" {
		t.Fatalf("columns %+v", view.Columns)
	}

	if err := r.UpdateTableView(ctx, id, "table_settings", "Mine", "table", "", columns); err != nil {
		t.Fatalf("update: %v", err)
	}
	views, _ = r.GetTableViews(ctx, "table_settings")
	if got := findView(t, views, id).LayoutJSON; got != "" {
		t.Fatalf("layout_json after clearing %q", got)
	}

	if err := r.UpdateTableView(ctx, id, "other_table", "Mine", "table", "", columns); err == nil {
		t.Fatal("expected update of a view through another table to fail")
	}
}

func TestUserDefaultTableView(t *testing.T) {
	r := newMigratedSQLiteRepo(t)
	ctx := context.Background()
	col := []TableViewColumn{{ColumnName: "name", IsVisible: true}}

	a, _ := r.CreateTableView(ctx, "table_settings", "A", "table", "", col)
	b, _ := r.CreateTableView(ctx, "table_settings", "B", "table", "", col)
	other, _ := r.CreateTableView(ctx, "table_navigation", "Other", "table", "", col)

	if got, _ := r.GetUserDefaultTableView(ctx, 1, "table_settings"); got != 0 {
		t.Fatalf("expected no default, got %d", got)
	}
	if err := r.SetUserDefaultTableView(ctx, 1, "table_settings", a); err != nil {
		t.Fatalf("set: %v", err)
	}
	if err := r.SetUserDefaultTableView(ctx, 1, "table_settings", b); err != nil {
		t.Fatalf("replace: %v", err)
	}
	if got, _ := r.GetUserDefaultTableView(ctx, 1, "table_settings"); got != b {
		t.Fatalf("user 1 default %d, want %d", got, b)
	}
	if got, _ := r.GetUserDefaultTableView(ctx, 2, "table_settings"); got != 0 {
		t.Fatalf("defaults must be per user, user 2 got %d", got)
	}
	if err := r.SetUserDefaultTableView(ctx, 1, "table_settings", other); err == nil {
		t.Fatal("expected a view from another table to be rejected")
	}

	if err := r.DeleteTableView(ctx, b); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if got, _ := r.GetUserDefaultTableView(ctx, 1, "table_settings"); got != 0 {
		t.Fatalf("default should be removed with its view, got %d", got)
	}

	if err := r.SetUserDefaultTableView(ctx, 1, "table_settings", a); err != nil {
		t.Fatalf("set: %v", err)
	}
	if err := r.SetUserDefaultTableView(ctx, 1, "table_settings", 0); err != nil {
		t.Fatalf("clear: %v", err)
	}
	if got, _ := r.GetUserDefaultTableView(ctx, 1, "table_settings"); got != 0 {
		t.Fatalf("expected cleared default, got %d", got)
	}
}

func findView(t *testing.T, views []TableView, id int) TableView {
	t.Helper()
	for _, v := range views {
		if v.ID == id {
			return v
		}
	}
	t.Fatalf("view %d not found in %+v", id, views)
	return TableView{}
}
