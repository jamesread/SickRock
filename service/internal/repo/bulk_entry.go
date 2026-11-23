package repo

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// SetBulkEntryScanColumn makes column the scan column of a table's bulk entry configuration
// (table_configurations.bulk_entry_json), turning bulk entry on. The rest of an existing
// configuration is kept; one that is not a JSON object is replaced.
func (r *Repository) SetBulkEntryScanColumn(ctx context.Context, tableConfiguration, column string) error {
	tableConfiguration = strings.TrimSpace(tableConfiguration)
	column = strings.TrimSpace(column)
	if tableConfiguration == "" || column == "" {
		return fmt.Errorf("table configuration and column are required")
	}
	var raw sql.NullString
	err := r.db.GetContext(ctx, &raw, `SELECT bulk_entry_json FROM table_configurations WHERE name = ?`, tableConfiguration)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("table configuration %q not found", tableConfiguration)
	}
	if err != nil {
		return err
	}
	out, err := json.Marshal(withBulkEntryScanColumn(raw.String, column))
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, `UPDATE table_configurations SET bulk_entry_json = ? WHERE name = ?`, string(out), tableConfiguration)
	return err
}

func withBulkEntryScanColumn(raw, column string) map[string]any {
	config := map[string]any{}
	if strings.TrimSpace(raw) != "" {
		if err := json.Unmarshal([]byte(raw), &config); err != nil || config == nil {
			config = map[string]any{}
		}
	}
	config["scan_column"] = column
	return config
}
