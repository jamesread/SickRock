package repo

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
)

// ReadOnlyCalendarExport is a configured read-only calendar view at /exports/{slug}.
type ReadOnlyCalendarExport struct {
	Slug               string
	Title              string
	TableConfiguration string
	WhereJSON          string
	DisplayMode        string
	AllowedGroupIDs    []int
	TableViewID        sql.NullInt64
	ID                 int
	WeekendsOnly       bool
	Enabled            bool
}

func (r *Repository) GetReadOnlyCalendarExportBySlug(ctx context.Context, slug string) (*ReadOnlyCalendarExport, error) {
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return nil, fmt.Errorf("slug is required")
	}

	var row struct {
		Slug               string        `db:"slug"`
		Title              string        `db:"title"`
		TableConfiguration string        `db:"table_configuration"`
		WhereJSON          string        `db:"where_json"`
		DisplayMode        string        `db:"display_mode"`
		AllowedGroupIDs    string        `db:"allowed_group_ids"`
		TableViewID        sql.NullInt64 `db:"table_view_id"`
		ID                 int           `db:"id"`
		WeekendsOnly       int           `db:"weekends_only"`
		Enabled            int           `db:"enabled"`
	}

	err := r.db.GetContext(ctx, &row, `
		SELECT id, slug, title, table_configuration, table_view_id, where_json,
		       weekends_only, display_mode, enabled, allowed_group_ids
		FROM read_only_calendar_exports
		WHERE slug = ?`, slug)
	if err != nil {
		return nil, calendarExportLookupError(err)
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

func calendarExportLookupError(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("export not found")
	}
	return err
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
	defer func() { _ = rows.Close() }()
	return scanEnabledCalendarExports(rows)
}

type calendarExportRow struct {
	Slug               string        `db:"slug"`
	Title              string        `db:"title"`
	TableConfiguration string        `db:"table_configuration"`
	WhereJSON          string        `db:"where_json"`
	DisplayMode        string        `db:"display_mode"`
	AllowedGroupIDs    string        `db:"allowed_group_ids"`
	TableViewID        sql.NullInt64 `db:"table_view_id"`
	ID                 int           `db:"id"`
	WeekendsOnly       int           `db:"weekends_only"`
	Enabled            int           `db:"enabled"`
}

func scanEnabledCalendarExports(rows interface {
	Next() bool
	StructScan(dest any) error
	Err() error
}) ([]ReadOnlyCalendarExport, error) {
	out := make([]ReadOnlyCalendarExport, 0, 8)
	for rows.Next() {
		export, err := scanCalendarExportRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, export)
	}
	return out, rows.Err()
}

func scanCalendarExportRow(rows interface {
	StructScan(dest any) error
}) (ReadOnlyCalendarExport, error) {
	var row calendarExportRow
	if err := rows.StructScan(&row); err != nil {
		return ReadOnlyCalendarExport{}, err
	}
	groupIDs, err := parseAllowedGroupIDs(row.AllowedGroupIDs)
	if err != nil {
		return ReadOnlyCalendarExport{}, fmt.Errorf("export %q: invalid allowed_group_ids: %w", row.Slug, err)
	}
	return ReadOnlyCalendarExport{
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
	// Key and calendar columns come first, in this order, when the table has them.
	leading := []string{pkColumn, "sr_created", "sr_updated", "calendar_date", "starts", "finishes"}
	names := presentColumns(leading, tableColumns)
	if freeBusy {
		return names
	}
	for _, col := range tableColumns {
		if !slices.Contains(leading, col.Name) && !slices.Contains(names, col.Name) {
			names = append(names, col.Name)
		}
	}
	return names
}

// presentColumns returns the wanted column names the table has, de-duplicated, in wanted order.
func presentColumns(wanted []string, tableColumns []FieldSpec) []string {
	names := make([]string, 0, len(wanted)+2)
	for _, n := range wanted {
		if hasColumn(tableColumns, n) && !slices.Contains(names, n) {
			names = append(names, n)
		}
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

	pkColumn := tc.PrimaryKeyColumnName()
	query, args := readOnlyExportQuery(tc, calendarExportColumnAllowlist(exp.DisplayMode == "free_busy", columns, pkColumn), where)
	items, err := r.queryItems(ctx, query, args, pkColumn)
	if err != nil {
		return nil, err
	}

	if exp.WeekendsOnly {
		items = filterItemsWeekendsOnly(items)
	}
	return items, nil
}

// readOnlyExportQuery selects selectCols (or just the key when empty) from the table, filtered by where,
// newest first.
func readOnlyExportQuery(tc *TableConfig, selectCols []string, where map[string]string) (string, []any) {
	if len(selectCols) == 0 {
		selectCols = []string{tc.PrimaryKeyColumnName()}
	}
	whereClause, args := exportWhereClause(where)
	query := fmt.Sprintf("SELECT `%s` FROM `%s`.`%s`%s ORDER BY `%s` DESC",
		strings.Join(selectCols, "`, `"), tc.Db.String, tc.Table.String, whereClause, tc.SortColumnName(selectCols))
	return query, args
}

// exportWhereClause builds an AND-ed WHERE clause from an export's filters; values containing % use LIKE.
func exportWhereClause(where map[string]string) (string, []any) {
	if len(where) == 0 {
		return "", nil
	}
	parts := make([]string, 0, len(where))
	args := make([]any, 0, len(where))
	for k, v := range where {
		op := "="
		if strings.Contains(v, "%") {
			op = "LIKE"
		}
		parts = append(parts, fmt.Sprintf("`%s` %s ?", sanitizeDatabaseIdentifier(k), op))
		args = append(args, v)
	}
	return " WHERE " + strings.Join(parts, " AND "), args
}

// queryItems runs query and scans each row into an Item keyed by pkColumn.
func (r *Repository) queryItems(ctx context.Context, query string, args []any, pkColumn string) ([]Item, error) {
	rows, err := r.db.QueryxContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var items []Item
	for rows.Next() {
		rowMap := make(map[string]any)
		if err := rows.MapScan(rowMap); err != nil {
			return nil, err
		}
		items = append(items, scanRowToItem(rowMap, pkColumn))
	}
	return items, rows.Err()
}

func filterItemsWeekendsOnly(items []Item) []Item {
	out := make([]Item, 0, len(items))
	for _, item := range items {
		dt, ok := resolveItemCalendarDate(item)
		if !ok {
			continue
		}
		if isWeekend(dt) {
			out = append(out, item)
		}
	}
	return out
}

func isWeekend(dt time.Time) bool {
	wd := dt.Weekday()
	return wd == time.Saturday || wd == time.Sunday
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
