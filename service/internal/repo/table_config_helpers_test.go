package repo

import (
	"database/sql"
	"testing"
)

func TestTableConfigPrimaryKeyColumnNameDefault(t *testing.T) {
	tc := &TableConfig{}
	if got := tc.PrimaryKeyColumnName(); got != "id" {
		t.Fatalf("expected id, got %q", got)
	}
}

func TestTableConfigPrimaryKeyColumnNameConfigured(t *testing.T) {
	tc := &TableConfig{PrimaryKeyColumn: sql.NullString{String: "uuid", Valid: true}}
	if got := tc.PrimaryKeyColumnName(); got != "uuid" {
		t.Fatalf("expected uuid, got %q", got)
	}
}

func TestTableConfigSortColumnNameDefault(t *testing.T) {
	tc := &TableConfig{}
	cols := []string{"code", "title"}
	if got := tc.SortColumnName(cols); got != "code" {
		t.Fatalf("expected first column when id missing, got %q", got)
	}
}

func TestTableConfigSortColumnNameConfigured(t *testing.T) {
	tc := &TableConfig{DefaultSortColumn: sql.NullString{String: "created_at", Valid: true}}
	cols := []string{"uuid", "created_at"}
	if got := tc.SortColumnName(cols); got != "created_at" {
		t.Fatalf("expected created_at, got %q", got)
	}
}
