package repo

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

const maxColumnDisplayLabelLen = 255
const maxColumnDisplayAffixLen = 512

// ColumnDisplaySetting is persisted display metadata for one column.
type ColumnDisplaySetting struct {
	ColumnName   string         `db:"column_name"`
	DisplayLabel sql.NullString `db:"display_label"`
	ValuePrefix  sql.NullString `db:"value_prefix"`
	ValueSuffix  sql.NullString `db:"value_suffix"`
}

// ColumnDisplayInput updates display metadata for one column.
type ColumnDisplayInput struct {
	ColumnName   string
	DisplayLabel string
	ValuePrefix  string
	ValueSuffix  string
}

func normalizeColumnDisplayInput(in ColumnDisplayInput) (ColumnDisplayInput, error) {
	col := sanitizeDatabaseIdentifier(in.ColumnName)
	if col == "" {
		return ColumnDisplayInput{}, fmt.Errorf("column name is required")
	}
	label := strings.TrimSpace(in.DisplayLabel)
	prefix := in.ValuePrefix
	suffix := in.ValueSuffix
	if len(label) > maxColumnDisplayLabelLen {
		return ColumnDisplayInput{}, fmt.Errorf("display label for %q is too long (max %d)", col, maxColumnDisplayLabelLen)
	}
	if len(prefix) > maxColumnDisplayAffixLen {
		return ColumnDisplayInput{}, fmt.Errorf("value prefix for %q is too long (max %d)", col, maxColumnDisplayAffixLen)
	}
	if len(suffix) > maxColumnDisplayAffixLen {
		return ColumnDisplayInput{}, fmt.Errorf("value suffix for %q is too long (max %d)", col, maxColumnDisplayAffixLen)
	}
	return ColumnDisplayInput{
		ColumnName:   col,
		DisplayLabel: label,
		ValuePrefix:  prefix,
		ValueSuffix:  suffix,
	}, nil
}

// LookupColumnDisplay returns display metadata for a physical column (case-insensitive name).
func LookupColumnDisplay(settings map[string]ColumnDisplaySetting, columnName string) (ColumnDisplaySetting, bool) {
	if settings == nil {
		return ColumnDisplaySetting{}, false
	}
	if d, ok := settings[columnName]; ok {
		return d, true
	}
	lower := strings.ToLower(columnName)
	for key, d := range settings {
		if strings.ToLower(key) == lower {
			return d, true
		}
	}
	return ColumnDisplaySetting{}, false
}

func (r *Repository) GetColumnDisplaySettings(ctx context.Context, tableConfiguration string) (map[string]ColumnDisplaySetting, error) {
	out := map[string]ColumnDisplaySetting{}
	tableConfiguration = strings.TrimSpace(tableConfiguration)
	if tableConfiguration == "" {
		return out, nil
	}
	var rows []ColumnDisplaySetting
	err := r.db.SelectContext(ctx, &rows, `
		SELECT column_name, display_label, value_prefix, value_suffix
		FROM table_column_display
		WHERE table_configuration = ?`, tableConfiguration)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.ColumnName] = row
	}
	return out, nil
}

func (r *Repository) upsertColumnDisplay(ctx context.Context, tableConfiguration string, in ColumnDisplayInput) error {
	if in.DisplayLabel == "" && in.ValuePrefix == "" && in.ValueSuffix == "" {
		_, err := r.db.ExecContext(ctx,
			`DELETE FROM table_column_display WHERE table_configuration = ? AND column_name = ?`,
			tableConfiguration, in.ColumnName)
		return err
	}
	var label, prefix, suffix interface{}
	if in.DisplayLabel != "" {
		label = in.DisplayLabel
	}
	if in.ValuePrefix != "" {
		prefix = in.ValuePrefix
	}
	if in.ValueSuffix != "" {
		suffix = in.ValueSuffix
	}
	switch r.db.DriverName() {
	case "mysql":
		const q = `
			INSERT INTO table_column_display (table_configuration, column_name, display_label, value_prefix, value_suffix)
			VALUES (?, ?, ?, ?, ?)
			ON DUPLICATE KEY UPDATE
				display_label = VALUES(display_label),
				value_prefix = VALUES(value_prefix),
				value_suffix = VALUES(value_suffix)`
		_, err := r.db.ExecContext(ctx, q, tableConfiguration, in.ColumnName, label, prefix, suffix)
		return err
	default:
		const q = `
			INSERT INTO table_column_display (table_configuration, column_name, display_label, value_prefix, value_suffix)
			VALUES (?, ?, ?, ?, ?)
			ON CONFLICT(table_configuration, column_name) DO UPDATE SET
				display_label = excluded.display_label,
				value_prefix = excluded.value_prefix,
				value_suffix = excluded.value_suffix`
		_, err := r.db.ExecContext(ctx, q, tableConfiguration, in.ColumnName, label, prefix, suffix)
		return err
	}
}

// SetColumnDisplaySettings upserts display metadata for the given columns.
func (r *Repository) SetColumnDisplaySettings(ctx context.Context, tableConfiguration string, settings []ColumnDisplayInput, allowedColumns map[string]struct{}) error {
	tableConfiguration = strings.TrimSpace(tableConfiguration)
	if tableConfiguration == "" {
		return fmt.Errorf("table configuration is required")
	}
	for _, raw := range settings {
		in, err := normalizeColumnDisplayInput(raw)
		if err != nil {
			return err
		}
		if allowedColumns != nil {
			canonical := ""
			for col := range allowedColumns {
				if strings.EqualFold(col, in.ColumnName) {
					canonical = col
					break
				}
			}
			if canonical == "" {
				return fmt.Errorf("unknown column %q", in.ColumnName)
			}
			in.ColumnName = canonical
		}
		if err := r.upsertColumnDisplay(ctx, tableConfiguration, in); err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) DeleteColumnDisplay(ctx context.Context, tableConfiguration, columnName string) error {
	columnName = sanitizeDatabaseIdentifier(columnName)
	if tableConfiguration == "" || columnName == "" {
		return nil
	}
	_, err := r.db.ExecContext(ctx,
		`DELETE FROM table_column_display WHERE table_configuration = ? AND column_name = ?`,
		tableConfiguration, columnName)
	return err
}

func (r *Repository) RenameColumnDisplay(ctx context.Context, tableConfiguration, oldColumnName, newColumnName string) error {
	oldColumnName = sanitizeDatabaseIdentifier(oldColumnName)
	newColumnName = sanitizeDatabaseIdentifier(newColumnName)
	if tableConfiguration == "" || oldColumnName == "" || newColumnName == "" {
		return nil
	}
	_, err := r.db.ExecContext(ctx, `
		UPDATE table_column_display
		SET column_name = ?
		WHERE table_configuration = ? AND column_name = ?`,
		newColumnName, tableConfiguration, oldColumnName)
	return err
}
