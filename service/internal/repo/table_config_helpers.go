package repo

import (
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
	out := make(map[string]string)
	unique := make(map[string]DbTablePair)
	for _, p := range pairs {
		if strings.TrimSpace(p.Table) == "" {
			continue
		}
		dbName := p.Db
		if dbName == "" {
			dbName = "main"
		}
		unique[dbTableLookupKey(dbName, p.Table)] = DbTablePair{Db: dbName, Table: p.Table}
	}
	if len(unique) == 0 {
		return out, nil
	}

	clauses := make([]string, 0, len(unique))
	args := make([]interface{}, 0, len(unique)*2)
	for _, p := range unique {
		clauses = append(clauses, "(`db` = ? AND `table` = ?)")
		args = append(args, p.Db, p.Table)
	}
	q := "SELECT name, `db`, `table` FROM table_configurations WHERE " + strings.Join(clauses, " OR ")
	rows, err := r.db.QueryxContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var name, dbName, tableName string
		if err := rows.Scan(&name, &dbName, &tableName); err != nil {
			return nil, err
		}
		if dbName == "" {
			dbName = "main"
		}
		out[dbTableLookupKey(dbName, tableName)] = name
	}
	return out, rows.Err()
}

const defaultTableKeyColumn = "id"

const tableConfigurationSelectColumns = "name, `db`, `table`, COALESCE(title, name) as title, COALESCE(ordinal, 0) as ordinal, create_button_text, create_delegate, row_name, icon, primary_key_column, default_sort_column"

// DisplayRowName returns the singular row label for UI messages (e.g. "Customer created successfully").
func (tc *TableConfig) DisplayRowName() string {
	if tc == nil {
		return "Row"
	}
	if tc.RowName.Valid {
		if label := strings.TrimSpace(tc.RowName.String); label != "" {
			return label
		}
	}
	if title := strings.TrimSpace(tc.Title); title != "" {
		return title
	}
	return "Row"
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

func itemIDFromRowValue(v interface{}) string {
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

func scanRowToItem(rowMap map[string]interface{}, pkColumn string) Item {
	item := Item{Fields: make(map[string]interface{})}
	if pkVal, ok := rowMap[pkColumn]; ok {
		item.ID = itemIDFromRowValue(pkVal)
	}
	if createdAt, ok := rowMap["sr_created"]; ok {
		if createdAtTime, ok := createdAt.(time.Time); ok {
			item.SrCreated = createdAtTime
		} else if createdAtStr, ok := createdAt.(string); ok {
			if parsedTime, err := time.Parse("2006-01-02 15:04:05", createdAtStr); err == nil {
				item.SrCreated = parsedTime
			}
		}
	}
	if updatedAt, ok := rowMap["sr_updated"]; ok {
		if updatedAtTime, ok := updatedAt.(time.Time); ok {
			item.SrUpdated = updatedAtTime
		} else if updatedAtStr, ok := updatedAt.(string); ok {
			if parsedTime, err := time.Parse("2006-01-02 15:04:05", updatedAtStr); err == nil {
				item.SrUpdated = parsedTime
			}
		}
	}
	for colName, value := range rowMap {
		if colName == pkColumn || colName == "sr_created" || colName == "sr_updated" {
			continue
		}
		if valueBytes, ok := value.([]uint8); ok {
			item.Fields[colName] = string(valueBytes)
		} else {
			item.Fields[colName] = value
		}
	}
	return item
}
