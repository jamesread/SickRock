package repo

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
)

// DbTablePair identifies a physical table for table_configurations lookup.
type DbTablePair struct {
	Db    string
	Table string
}

func dbTableLookupKey(db, table string) string {
	return db + "\x00" + table
}

// DbTableLookupKey is the map key returned by LookupTableConfigNames.
func DbTableLookupKey(db, table string) string {
	if strings.TrimSpace(table) == "" {
		return ""
	}
	dbName := db
	if dbName == "" {
		dbName = "main"
	}
	return dbTableLookupKey(dbName, table)
}

// LookupTableConfigNames resolves configuration names for many (db, table) pairs in one query.
func (r *Repository) LookupTableConfigNames(ctx context.Context, pairs []DbTablePair) (map[string]string, error) {
	unique := uniqueDbTablePairs(pairs)
	if len(unique) == 0 {
		return map[string]string{}, nil
	}

	clauses := make([]string, 0, len(unique))
	args := make([]any, 0, len(unique)*2)
	for _, p := range unique {
		clauses = append(clauses, "(`db` = ? AND `table` = ?)")
		args = append(args, p.Db, p.Table)
	}
	q := "SELECT name, `db`, `table` FROM table_configurations WHERE " + strings.Join(clauses, " OR ")
	rows, err := r.db.QueryxContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := make(map[string]string)
	for rows.Next() {
		var name, dbName, tableName string
		if err := rows.Scan(&name, &dbName, &tableName); err != nil {
			return nil, err
		}
		out[dbTableLookupKey(cmp.Or(dbName, "main"), tableName)] = name
	}
	return out, rows.Err()
}

// uniqueDbTablePairs de-duplicates pairs with a table name, defaulting an empty database to "main".
func uniqueDbTablePairs(pairs []DbTablePair) map[string]DbTablePair {
	unique := make(map[string]DbTablePair)
	for _, p := range pairs {
		if strings.TrimSpace(p.Table) == "" {
			continue
		}
		dbName := cmp.Or(p.Db, "main")
		unique[dbTableLookupKey(dbName, p.Table)] = DbTablePair{Db: dbName, Table: p.Table}
	}
	return unique
}

const defaultTableKeyColumn = "id"

const tableConfigurationSelectColumns = "name, `db`, `table`, COALESCE(title, name) as title, COALESCE(ordinal, 0) as ordinal, create_button_text, create_delegate, row_name, row_name_plural, icon, primary_key_column, default_sort_column"

// DisplayRowName returns the singular row label for UI messages (e.g. "Customer created successfully").
func (tc *TableConfig) DisplayRowName() string {
	if tc == nil {
		return "Row"
	}
	if label := trimmedValidString(tc.RowName.Valid, tc.RowName.String); label != "" {
		return label
	}
	if title := strings.TrimSpace(tc.Title); title != "" {
		return title
	}
	return "Row"
}

func trimmedValidString(valid bool, value string) string {
	if !valid {
		return ""
	}
	return strings.TrimSpace(value)
}

// DisplayRowNamePlural returns the plural row label for list empty states (e.g. "Customers").
// SetTableRowLabels updates singular and plural row labels on table_configurations.
// Empty strings clear the stored value (NULL).
func (r *Repository) SetTableRowLabels(ctx context.Context, tableConfiguration, rowName, rowNamePlural string) error {
	tcName := strings.TrimSpace(tableConfiguration)
	if tcName == "" {
		return fmt.Errorf("table configuration is required")
	}
	if _, err := r.GetTableConfiguration(ctx, tcName); err != nil {
		return err
	}
	var rn, rnp any
	if t := strings.TrimSpace(rowName); t != "" {
		rn = t
	}
	if t := strings.TrimSpace(rowNamePlural); t != "" {
		rnp = t
	}
	res, err := r.db.ExecContext(ctx,
		`UPDATE table_configurations SET row_name = ?, row_name_plural = ? WHERE name = ?`,
		rn, rnp, tcName,
	)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("table configuration not found: %s", tcName)
	}
	return nil
}

func (tc *TableConfig) DisplayRowNamePlural() string {
	if tc == nil {
		return "items"
	}
	if label := trimmedValidString(tc.RowNamePlural.Valid, tc.RowNamePlural.String); label != "" {
		return label
	}
	return pluralizeRowName(tc.DisplayRowName())
}

func pluralizeRowName(singular string) string {
	if singular == "Row" {
		return "items"
	}
	return singular + "s"
}

func (r *Repository) sqlCurrentTimestampExpr() string {
	if r.db.DriverName() == "mysql" {
		return "NOW()"
	}
	return "datetime('now')"
}

func (r *Repository) sqlUnixTimestampExpr() string {
	if r.db.DriverName() == "mysql" {
		return "UNIX_TIMESTAMP()"
	}
	return "strftime('%s', 'now')"
}

func (tc *TableConfig) PrimaryKeyColumnName() string {
	if tc == nil {
		return defaultTableKeyColumn
	}
	if tc.PrimaryKeyColumn.Valid {
		if col := strings.TrimSpace(tc.PrimaryKeyColumn.String); col != "" {
			return sanitizeDatabaseIdentifier(col)
		}
	}
	return defaultTableKeyColumn
}

func (tc *TableConfig) SortColumnName(columnNames []string) string {
	candidate := defaultTableKeyColumn
	if tc != nil && tc.DefaultSortColumn.Valid {
		if col := strings.TrimSpace(tc.DefaultSortColumn.String); col != "" {
			candidate = sanitizeDatabaseIdentifier(col)
		}
	}
	if slices.Contains(columnNames, candidate) {
		return candidate
	}
	if slices.Contains(columnNames, defaultTableKeyColumn) {
		return defaultTableKeyColumn
	}
	if len(columnNames) > 0 {
		return columnNames[0]
	}
	return defaultTableKeyColumn
}

func itemIDFromRowValue(v any) string {
	if v == nil {
		return ""
	}
	switch id := v.(type) {
	case string:
		return id
	case int64:
		return strconv.FormatInt(id, 10)
	case int32:
		return strconv.FormatInt(int64(id), 10)
	case int:
		return strconv.Itoa(id)
	case []uint8:
		return string(id)
	default:
		return fmt.Sprint(v)
	}
}

func scanRowToItem(rowMap map[string]any, pkColumn string) Item {
	item := Item{
		Fields:    make(map[string]any),
		SrCreated: rowTimestamp(rowMap["sr_created"]),
		SrUpdated: rowTimestamp(rowMap["sr_updated"]),
	}
	if pkVal, ok := rowMap[pkColumn]; ok {
		item.ID = itemIDFromRowValue(pkVal)
	}
	for colName, value := range rowMap {
		if colName == pkColumn || colName == "sr_created" || colName == "sr_updated" {
			continue
		}
		item.Fields[colName] = bytesToString(value)
	}
	return item
}

// rowTimestamp converts a scanned timestamp column (time.Time or MySQL datetime string) to a time,
// returning zero time for missing or unparsable values.
func rowTimestamp(value any) time.Time {
	switch v := value.(type) {
	case time.Time:
		return v
	case string:
		if parsed, err := time.Parse("2006-01-02 15:04:05", v); err == nil {
			return parsed
		}
	}
	return time.Time{}
}
