package repo

import (
	"context"
	"testing"

	"github.com/jmoiron/sqlx"
)

func TestCreateDashboardWithNavigationSQLite(t *testing.T) {
	db, err := sqlx.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	if _, err := db.Exec(`
		CREATE TABLE table_dashboards (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT);
		CREATE TABLE table_navigation (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			ordinal INTEGER DEFAULT 99,
			table_configuration INTEGER,
			dashboard_id INTEGER,
			name TEXT
		);
	`); err != nil {
		t.Fatalf("setup schema: %v", err)
	}

	r := NewRepository(db)
	id, err := r.CreateDashboard(context.Background(), "ops", true)
	if err != nil {
		t.Fatalf("CreateDashboard: %v", err)
	}
	if id <= 0 {
		t.Fatalf("expected positive id, got %d", id)
	}

	var navCount int
	if err := db.Get(&navCount, "SELECT COUNT(*) FROM table_navigation WHERE dashboard_id = ? AND name = ?", id, "ops"); err != nil {
		t.Fatalf("count navigation: %v", err)
	}
	if navCount != 1 {
		t.Fatalf("expected 1 navigation row, got %d", navCount)
	}
}

func TestCreateDashboardSkipsNavigationWhenDisabled(t *testing.T) {
	db, err := sqlx.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	if _, err := db.Exec(`
		CREATE TABLE table_dashboards (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT);
		CREATE TABLE table_navigation (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			ordinal INTEGER DEFAULT 99,
			table_configuration INTEGER,
			dashboard_id INTEGER,
			name TEXT
		);
	`); err != nil {
		t.Fatalf("setup schema: %v", err)
	}

	r := NewRepository(db)
	_, err = r.CreateDashboard(context.Background(), "private", false)
	if err != nil {
		t.Fatalf("CreateDashboard: %v", err)
	}

	var navCount int
	if err := db.Get(&navCount, "SELECT COUNT(*) FROM table_navigation"); err != nil {
		t.Fatalf("count navigation: %v", err)
	}
	if navCount != 0 {
		t.Fatalf("expected no navigation rows, got %d", navCount)
	}
}
