package repo

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// ReadOnlyCalendarExport is a configured read-only calendar view at /exports/{slug}.
type ReadOnlyCalendarExport struct {
	ID                 int
	Slug               string
	Title              string
	TableConfiguration string
	TableViewID        sql.NullInt64
	WhereJSON          string
	WeekendsOnly       bool
	DisplayMode        string
	Enabled            bool
	AllowedGroupIDs    []int
}

func (r *Repository) GetReadOnlyCalendarExportBySlug(ctx context.Context, slug string) (*ReadOnlyCalendarExport, error) {
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return nil, fmt.Errorf("slug is required")
	}

	var row struct {
		ID                 int            `db:"id"`
		Slug               string         `db:"slug"`
		Title              string         `db:"title"`
		TableConfiguration string         `db:"table_configuration"`
		TableViewID        sql.NullInt64  `db:"table_view_id"`
		WhereJSON          string         `db:"where_json"`
		WeekendsOnly       int            `db:"weekends_only"`
		DisplayMode        string         `db:"display_mode"`
		Enabled            int            `db:"enabled"`
		AllowedGroupIDs    string         `db:"allowed_group_ids"`
	}

	err := r.db.GetContext(ctx, &row, `
		SELECT id, slug, title, table_configuration, table_view_id, where_json,
		       weekends_only, display_mode, enabled, allowed_group_ids
		FROM read_only_calendar_exports
		WHERE slug = ?`, slug)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("export not found")
		}
		return nil, err
	}

	groupIDs, err := parseAllowedGroupIDs(row.AllowedGroupIDs)
	if err != nil {
		return nil, fmt.Errorf("invalid allowed_group_ids: %w", err)
	}

	return &ReadOnlyCalendarExport{
		ID:                 row.ID,
		Slug:               row.Slug,
		Title:              row.Title,
		TableConfiguration: row.TableConfiguration,
		TableViewID:        row.TableViewID,
		WhereJSON:          row.WhereJSON,
		WeekendsOnly:       row.WeekendsOnly != 0,
		DisplayMode:        row.DisplayMode,
		Enabled:            row.Enabled != 0,
		AllowedGroupIDs:    groupIDs,
	}, nil
}

func (r *Repository) ListEnabledReadOnlyCalendarExports(ctx context.Context) ([]ReadOnlyCalendarExport, error) {
	rows, err := r.db.QueryxContext(ctx, `
		SELECT id, slug, title, table_configuration, table_view_id, where_json,
		       weekends_only, display_mode, enabled, allowed_group_ids
		FROM read_only_calendar_exports
		WHERE enabled != 0
		ORDER BY title, slug`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]ReadOnlyCalendarExport, 0, 8)
	for rows.Next() {
		var row struct {
			ID                 int           `db:"id"`
			Slug               string        `db:"slug"`
			Title              string        `db:"title"`
			TableConfiguration string        `db:"table_configuration"`
			TableViewID        sql.NullInt64 `db:"table_view_id"`
			WhereJSON          string        `db:"where_json"`
			WeekendsOnly       int           `db:"weekends_only"`
			DisplayMode        string        `db:"display_mode"`
			Enabled            int           `db:"enabled"`
			AllowedGroupIDs    string        `db:"allowed_group_ids"`
		}
		if err := rows.StructScan(&row); err != nil {
			return nil, err
		}
		groupIDs, err := parseAllowedGroupIDs(row.AllowedGroupIDs)
		if err != nil {
			return nil, fmt.Errorf("export %q: invalid allowed_group_ids: %w", row.Slug, err)
		}
		out = append(out, ReadOnlyCalendarExport{
			ID:                 row.ID,
			Slug:               row.Slug,
			Title:              row.Title,
			TableConfiguration: row.TableConfiguration,
			TableViewID:        row.TableViewID,
			WhereJSON:          row.WhereJSON,
			WeekendsOnly:       row.WeekendsOnly != 0,
			DisplayMode:        row.DisplayMode,
			Enabled:            row.Enabled != 0,
			AllowedGroupIDs:    groupIDs,
		})
	}
	return out, rows.Err()
}

func parseAllowedGroupIDs(raw string) ([]int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "[]" {
		return nil, nil
	}
	var ids []int
	if err := json.Unmarshal([]byte(raw), &ids); err != nil {
		return nil, err
	}
	return ids, nil
}

func parseExportWhereJSON(raw string) (map[string]string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return map[string]string{}, nil
	}
	out := map[string]string{}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, err
	}
	return out, nil
}

// calendarExportColumnAllowlist returns columns to SELECT for calendar exports.
func calendarExportColumnAllowlist(freeBusy bool, tableColumns []FieldSpec, pkColumn string) []string {
	if pkColumn == "" {
		pkColumn = defaultTableKeyColumn
	}
	always := map[string]bool{pkColumn: true, "sr_created": true, "sr_updated": true, "calendar_date": true, "starts": true, "finishes": true}
	names := make([]string, 0, 8)
	seen := map[string]bool{}
	for _, n := range []string{pkColumn, "sr_created", "sr_updated", "calendar_date", "starts", "finishes"} {
		for _, col := range tableColumns {
			if col.Name == n && !seen[n] {
				names = append(names, n)
				seen[n] = true
				break
			}
		}
	}
	if freeBusy {
		return names
	}
	for _, col := range tableColumns {
		if always[col.Name] || seen[col.Name] {
			continue
		}
		names = append(names, col.Name)
		seen[col.Name] = true
	}
	return names
}

// ListItemsForReadOnlyExport loads rows for an export definition with server-side filters only.
func (r *Repository) ListItemsForReadOnlyExport(ctx context.Context, exp *ReadOnlyCalendarExport) ([]Item, error) {
	if exp == nil {
		return nil, fmt.Errorf("export is required")
	}
	where, err := parseExportWhereJSON(exp.WhereJSON)
	if err != nil {
		return nil, err
	}

	tc, err := r.GetTableConfiguration(ctx, exp.TableConfiguration)
	if err != nil {
		return nil, err
	}

	columns, err := r.ListColumns(ctx, tc)
	if err != nil {
		return nil, err
	}

	freeBusy := exp.DisplayMode == "free_busy"
	pkColumn := tc.PrimaryKeyColumnName()
	selectCols := calendarExportColumnAllowlist(freeBusy, columns, pkColumn)
	if len(selectCols) == 0 {
		selectCols = []string{pkColumn}
	}

	sortColumn := tc.SortColumnName(selectCols)

	var whereClause string
	var args []interface{}
	if len(where) > 0 {
		parts := make([]string, 0, len(where))
		for k, v := range where {
			col := sanitizeDatabaseIdentifier(k)
			if strings.Contains(v, "%") {
				parts = append(parts, fmt.Sprintf("`%s` LIKE ?", col))
				args = append(args, v)
			} else {
				parts = append(parts, fmt.Sprintf("`%s` = ?", col))
				args = append(args, v)
			}
		}
		whereClause = " WHERE " + strings.Join(parts, " AND ")
	}

	query := fmt.Sprintf("SELECT `%s` FROM `%s`.`%s`%s ORDER BY `%s` DESC",
		strings.Join(selectCols, "`, `"), tc.Db.String, tc.Table.String, whereClause, sortColumn)

	rows, err := r.db.QueryxContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []Item
	for rows.Next() {
		rowMap := make(map[string]interface{})
		if err := rows.MapScan(rowMap); err != nil {
			return nil, err
		}
		items = append(items, scanRowToItem(rowMap, pkColumn))
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if exp.WeekendsOnly {
		items = filterItemsWeekendsOnly(items)
	}
	return items, nil
}

func parseTimeField(v interface{}) time.Time {
	switch t := v.(type) {
	case time.Time:
		return t
	case string:
		if parsed, err := time.Parse("2006-01-02 15:04:05", t); err == nil {
			return parsed
		}
		if parsed, err := time.Parse("2006-01-02", t); err == nil {
			return parsed
		}
	case []uint8:
		s := string(t)
		if parsed, err := time.Parse("2006-01-02 15:04:05", s); err == nil {
			return parsed
		}
	}
	return time.Time{}
}

func filterItemsWeekendsOnly(items []Item) []Item {
	out := make([]Item, 0, len(items))
	for _, item := range items {
		dt, ok := resolveItemCalendarDate(item)
		if !ok {
			continue
		}
		wd := dt.Weekday()
		if wd == time.Saturday || wd == time.Sunday {
			out = append(out, item)
		}
	}
	return out
}

func resolveItemCalendarDate(item Item) (time.Time, bool) {
	get := func(key string) string {
		if v, ok := item.Fields[key]; ok && v != nil {
			return strings.TrimSpace(fmt.Sprint(v))
		}
		return ""
	}
	for _, key := range []string{"calendar_date", "starts", "finishes"} {
		if s := get(key); s != "" {
			if t, ok := parseFlexibleDateTime(s); ok {
				return t, true
			}
		}
	}
	if !item.SrCreated.IsZero() {
		return item.SrCreated, true
	}
	return time.Time{}, false
}

func parseFlexibleDateTime(s string) (time.Time, bool) {
	layouts := []string{
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
		"2006-01-02",
		time.RFC3339,
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}
