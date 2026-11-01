package repo

import (
	"context"
	"testing"
)

func TestGetAppTitle(t *testing.T) {
	r := newMigratedSQLiteRepo(t)
	ctx := context.Background()

	if _, err := r.db.ExecContext(ctx, `DELETE FROM table_settings WHERE setting_key = ?`, AppTitleSettingKey); err != nil {
		t.Fatal(err)
	}
	if title, err := r.GetAppTitle(ctx); err != nil || title != "" {
		t.Fatalf("unset: GetAppTitle = %q, %v; want empty", title, err)
	}

	if _, err := r.db.ExecContext(ctx, `INSERT INTO table_settings (setting_key, string_val) VALUES (?, NULL)`, AppTitleSettingKey); err != nil {
		t.Fatal(err)
	}
	if title, err := r.GetAppTitle(ctx); err != nil || title != "" {
		t.Fatalf("NULL: GetAppTitle = %q, %v; want empty", title, err)
	}

	if _, err := r.db.ExecContext(ctx, `UPDATE table_settings SET string_val = ? WHERE setting_key = ?`, "Acme Stock", AppTitleSettingKey); err != nil {
		t.Fatal(err)
	}
	if title, err := r.GetAppTitle(ctx); err != nil || title != "Acme Stock" {
		t.Fatalf("set: GetAppTitle = %q, %v; want Acme Stock", title, err)
	}
}
