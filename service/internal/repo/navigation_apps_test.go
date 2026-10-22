package repo

import (
	"context"
	"testing"

	"github.com/jmoiron/sqlx"
)

func TestGetNavigationAppsOrdersMembers(t *testing.T) {
	db, err := sqlx.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	if _, err = db.ExecContext(ctx, `
		CREATE TABLE table_navigation (id INTEGER PRIMARY KEY, ordinal INTEGER);
		INSERT INTO table_navigation (id, ordinal) VALUES (1, 1), (2, 2), (3, 3);

		CREATE TABLE apps (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			title TEXT NOT NULL,
			ordinal INTEGER NOT NULL DEFAULT 99
		);
		INSERT INTO apps (id, title, ordinal) VALUES (10, 'CRM', 1), (20, 'Ops', 2);

		CREATE TABLE app_navigation_links (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			app_id INTEGER NOT NULL,
			navigation_item_id INTEGER NOT NULL,
			ordinal INTEGER NOT NULL DEFAULT 99,
			UNIQUE (navigation_item_id)
		);
		INSERT INTO app_navigation_links (app_id, navigation_item_id, ordinal) VALUES
			(10, 2, 2),
			(10, 1, 1),
			(20, 3, 1);
	`); err != nil {
		t.Fatalf("schema: %v", err)
	}

	repo := NewRepository(db)
	apps, err := repo.GetNavigationApps(ctx)
	if err != nil {
		t.Fatalf("GetNavigationApps: %v", err)
	}
	if len(apps) != 2 {
		t.Fatalf("apps len %d", len(apps))
	}
	if apps[0].Title != "CRM" || len(apps[0].MemberIDs) != 2 || apps[0].MemberIDs[0] != 1 || apps[0].MemberIDs[1] != 2 {
		t.Fatalf("CRM members: %+v", apps[0])
	}
	if apps[1].Title != "Ops" || len(apps[1].MemberIDs) != 1 || apps[1].MemberIDs[0] != 3 {
		t.Fatalf("Ops members: %+v", apps[1])
	}

	assigned, err := repo.NavigationItemIDsInApps(ctx)
	if err != nil {
		t.Fatalf("NavigationItemIDsInApps: %v", err)
	}
	if len(assigned) != 3 {
		t.Fatalf("assigned len %d", len(assigned))
	}
}
