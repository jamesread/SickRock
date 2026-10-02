package repo

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
)

const defaultTableKeyColumn = "id"

const tableConfigurationSelectColumns = "name, `db`, `table`, COALESCE(title, name) as title, COALESCE(ordinal, 0) as ordinal, create_button_text, icon, primary_key_column, default_sort_column"

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
