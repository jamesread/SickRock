package repo

import (
	"context"
	"testing"
)

func appsRowActions(t *testing.T, r *Repository) string {
	t.Helper()
	var actions string
	if err := r.db.GetContext(context.Background(), &actions,
		`SELECT row_actions FROM table_configurations WHERE name = 'table_apps'`); err != nil {
		t.Fatal(err)
	}
	return actions
}

func setAppsRowActions(t *testing.T, r *Repository, actions any) {
	t.Helper()
	if _, err := r.db.ExecContext(context.Background(),
		`UPDATE table_configurations SET row_actions = ? WHERE name = 'table_apps'`, actions); err != nil {
		t.Fatal(err)
	}
}

func TestSystemTableRowActionsDefaults(t *testing.T) {
	r := newMigratedSQLiteRepo(t)
	ctx := context.Background()
	if err := r.UpdateSystemTableConfigurations(ctx); err != nil {
		t.Fatal(err)
	}
	if got := appsRowActions(t, r); got != appRowActions {
		t.Fatalf("new install: row_actions = %s, want %s", got, appRowActions)
	}

	// An install seeded with the old default gets the new one.
	setAppsRowActions(t, r, appRowActionsV1)
	if err := r.UpdateSystemTableConfigurations(ctx); err != nil {
		t.Fatal(err)
	}
	if got := appsRowActions(t, r); got != appRowActions {
		t.Fatalf("old default: row_actions = %s, want %s", got, appRowActions)
	}

	// An edited value is kept.
	edited := `[{"label":"New table here","path":"/admin/table/create","query":{"navigationAppId":"{id}"}}]`
	setAppsRowActions(t, r, edited)
	if err := r.UpdateSystemTableConfigurations(ctx); err != nil {
		t.Fatal(err)
	}
	if got := appsRowActions(t, r); got != edited {
		t.Fatalf("edited: row_actions = %s, want it kept", got)
	}

	// Cleared (NULL) is seeded with the current default.
	setAppsRowActions(t, r, nil)
	if err := r.UpdateSystemTableConfigurations(ctx); err != nil {
		t.Fatal(err)
	}
	if got := appsRowActions(t, r); got != appRowActions {
		t.Fatalf("NULL: row_actions = %s, want %s", got, appRowActions)
	}
}
