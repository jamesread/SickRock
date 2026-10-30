package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/jmoiron/sqlx"
)

// ErrNoMatchingItem is returned when a negative quantity is added and no row matches.
var ErrNoMatchingItem = errors.New("no matching item")

// ErrNegativeQuantity is returned when adding a quantity would leave a row below zero.
var ErrNegativeQuantity = errors.New("quantity would become negative")

// AddItemQuantityResult is the row an AddItemQuantity call changed or created.
type AddItemQuantityResult struct {
	Item     Item
	Quantity int64
	Created  bool
}

// AddItemQuantity adds delta to quantityColumn on the newest row whose columns equal match (an empty
// value matches NULL), or, when no row matches and delta is positive, inserts a row with match, fields
// and quantityColumn = delta. The lookup and write run in one transaction.
func (r *Repository) AddItemQuantity(ctx context.Context, table string, match map[string]string, quantityColumn string, delta int64, fields map[string]string) (AddItemQuantityResult, error) {
	tc, err := r.GetTableConfiguration(ctx, table)
	if err != nil {
		return AddItemQuantityResult{}, fmt.Errorf("failed to get table configuration for table %s: %w", table, err)
	}
	columns, err := r.ListColumns(ctx, tc)
	if err != nil {
		return AddItemQuantityResult{}, fmt.Errorf("failed to get columns for table %s: %w", table, err)
	}
	qtyCol := writableColumn(columns, quantityColumn)
	if qtyCol == nil {
		return AddItemQuantityResult{}, fmt.Errorf("unknown quantity column %q", quantityColumn)
	}
	whereSQL, whereArgs, err := itemMatchWhere(columns, match)
	if err != nil {
		return AddItemQuantityResult{}, err
	}

	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return AddItemQuantityResult{}, err
	}
	defer func() { _ = tx.Rollback() }()

	pkColumn := tc.PrimaryKeyColumnName()
	tableRef := fmt.Sprintf("`%s`.`%s`", tc.Db.String, tc.Table.String)
	lock := ""
	if r.db.DriverName() == "mysql" {
		lock = " FOR UPDATE"
	}
	selectQuery := fmt.Sprintf("SELECT `%s`, COALESCE(`%s`, 0) FROM %s WHERE %s ORDER BY `%s` DESC LIMIT 1%s",
		pkColumn, qtyCol.Name, tableRef, whereSQL, pkColumn, lock)

	var id string
	var current int64
	err = tx.QueryRowxContext(ctx, selectQuery, whereArgs...).Scan(&id, &current)
	switch {
	case err == nil:
		total := current + delta
		if total < 0 {
			return AddItemQuantityResult{}, ErrNegativeQuantity
		}
		set, args := itemUpdateSet(columns, "", map[string]string{qtyCol.Name: strconv.FormatInt(total, 10)}, r.sqlCurrentTimestampExpr())
		updateQuery := fmt.Sprintf("UPDATE %s SET %s WHERE `%s` = ?", tableRef, strings.Join(set, ", "), pkColumn)
		if _, err = tx.ExecContext(ctx, updateQuery, append(args, id)...); err != nil {
			return AddItemQuantityResult{}, err
		}
		return r.commitItemQuantity(ctx, tx, tc, id, total, false)
	case errors.Is(err, sql.ErrNoRows):
		if delta <= 0 {
			return AddItemQuantityResult{}, ErrNoMatchingItem
		}
		values := maps.Clone(fields)
		if values == nil {
			values = map[string]string{}
		}
		for k, v := range match {
			if v == "" {
				delete(values, k)
			} else {
				values[k] = v
			}
		}
		values[qtyCol.Name] = strconv.FormatInt(delta, 10)
		insertColumns, placeholders, args := itemInsertColumns(columns, values, r.sqlCurrentTimestampExpr())
		insertQuery := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", tableRef, strings.Join(insertColumns, ", "), strings.Join(placeholders, ", "))
		res, err := tx.ExecContext(ctx, insertQuery, args...)
		if err != nil {
			return AddItemQuantityResult{}, err
		}
		lastID, err := res.LastInsertId()
		if err != nil {
			return AddItemQuantityResult{}, err
		}
		return r.commitItemQuantity(ctx, tx, tc, strconv.FormatInt(lastID, 10), delta, true)
	default:
		return AddItemQuantityResult{}, err
	}
}

func (r *Repository) commitItemQuantity(ctx context.Context, tx *sqlx.Tx, tc *TableConfig, id string, total int64, created bool) (AddItemQuantityResult, error) {
	if err := tx.Commit(); err != nil {
		return AddItemQuantityResult{}, err
	}
	item, err := r.GetItemInTable(ctx, tc, id)
	if err != nil {
		return AddItemQuantityResult{}, err
	}
	return AddItemQuantityResult{Item: item, Quantity: total, Created: created}, nil
}

// itemMatchWhere builds an AND of column equality tests (an empty value tests IS NULL) in a stable
// column order. Every column must exist on the table.
func itemMatchWhere(columns []FieldSpec, match map[string]string) (string, []any, error) {
	if len(match) == 0 {
		return "", nil, errors.New("at least one match column is required")
	}
	parts := make([]string, 0, len(match))
	args := make([]any, 0, len(match))
	for _, key := range slices.Sorted(maps.Keys(match)) {
		col := writableColumn(columns, key)
		if col == nil {
			return "", nil, fmt.Errorf("unknown match column %q", key)
		}
		if match[key] == "" {
			parts = append(parts, fmt.Sprintf("`%s` IS NULL", col.Name))
			continue
		}
		parts = append(parts, fmt.Sprintf("`%s` = ?", col.Name))
		args = append(args, match[key])
	}
	return strings.Join(parts, " AND "), args, nil
}
