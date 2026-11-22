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
	q, err := r.newQuantityUpdate(ctx, table, quantityColumn)
	if err != nil {
		return AddItemQuantityResult{}, err
	}
	whereSQL, whereArgs, err := itemMatchWhere(q.columns, match)
	if err != nil {
		return AddItemQuantityResult{}, err
	}

	q.tx, err = r.db.BeginTxx(ctx, nil)
	if err != nil {
		return AddItemQuantityResult{}, err
	}
	defer func() { _ = q.tx.Rollback() }()

	return r.addToNewestMatch(ctx, q, match, fields, delta, whereSQL, whereArgs)
}

// addToNewestMatch adds delta to the newest matching row, or inserts a new row when none matches.
func (r *Repository) addToNewestMatch(ctx context.Context, q *quantityUpdate, match map[string]string, fields map[string]string, delta int64, whereSQL string, whereArgs []any) (AddItemQuantityResult, error) {
	id, current, err := q.newestMatch(ctx, r.db.DriverName(), whereSQL, whereArgs)
	switch {
	case err == nil:
		return r.updateItemQuantity(ctx, q, id, current+delta)
	case errors.Is(err, sql.ErrNoRows):
		return r.insertItemQuantity(ctx, q, match, fields, delta)
	default:
		return AddItemQuantityResult{}, err
	}
}

// quantityUpdate is one AddItemQuantity call's table and transaction.
type quantityUpdate struct {
	tx        *sqlx.Tx
	tc        *TableConfig
	tableRef  string
	pkColumn  string
	qtyColumn string
	columns   []FieldSpec
}

func (r *Repository) newQuantityUpdate(ctx context.Context, table, quantityColumn string) (*quantityUpdate, error) {
	tc, err := r.GetTableConfiguration(ctx, table)
	if err != nil {
		return nil, fmt.Errorf("failed to get table configuration for table %s: %w", table, err)
	}
	columns, err := r.ListColumns(ctx, tc)
	if err != nil {
		return nil, fmt.Errorf("failed to get columns for table %s: %w", table, err)
	}
	qtyCol := writableColumn(columns, quantityColumn)
	if qtyCol == nil {
		return nil, fmt.Errorf("unknown quantity column %q", quantityColumn)
	}
	return &quantityUpdate{
		tc:        tc,
		columns:   columns,
		tableRef:  fmt.Sprintf("`%s`.`%s`", tc.Db.String, tc.Table.String),
		pkColumn:  tc.PrimaryKeyColumnName(),
		qtyColumn: qtyCol.Name,
	}, nil
}

// newestMatch returns the id and quantity of the newest matching row (locked on MySQL), or
// sql.ErrNoRows.
func (q *quantityUpdate) newestMatch(ctx context.Context, driver, whereSQL string, whereArgs []any) (id string, current int64, err error) {
	lock := ""
	if driver == "mysql" {
		lock = " FOR UPDATE"
	}
	selectQuery := fmt.Sprintf("SELECT `%s`, COALESCE(`%s`, 0) FROM %s WHERE %s ORDER BY `%s` DESC LIMIT 1%s",
		q.pkColumn, q.qtyColumn, q.tableRef, whereSQL, q.pkColumn, lock)
	err = q.tx.QueryRowxContext(ctx, selectQuery, whereArgs...).Scan(&id, &current)
	return id, current, err
}

func (r *Repository) updateItemQuantity(ctx context.Context, q *quantityUpdate, id string, total int64) (AddItemQuantityResult, error) {
	if total < 0 {
		return AddItemQuantityResult{}, ErrNegativeQuantity
	}
	set, args := itemUpdateSet(q.columns, "", map[string]string{q.qtyColumn: strconv.FormatInt(total, 10)}, r.sqlCurrentTimestampExpr())
	updateQuery := fmt.Sprintf("UPDATE %s SET %s WHERE `%s` = ?", q.tableRef, strings.Join(set, ", "), q.pkColumn)
	if _, err := q.tx.ExecContext(ctx, updateQuery, append(args, id)...); err != nil {
		return AddItemQuantityResult{}, err
	}
	return r.commitItemQuantity(ctx, q.tx, q.tc, id, total, false)
}

func (r *Repository) insertItemQuantity(ctx context.Context, q *quantityUpdate, match, fields map[string]string, delta int64) (AddItemQuantityResult, error) {
	if delta <= 0 {
		return AddItemQuantityResult{}, ErrNoMatchingItem
	}
	values := newQuantityRowValues(match, fields)
	values[q.qtyColumn] = strconv.FormatInt(delta, 10)
	insertColumns, placeholders, args := itemInsertColumns(q.columns, values, r.sqlCurrentTimestampExpr())
	insertQuery := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", q.tableRef, strings.Join(insertColumns, ", "), strings.Join(placeholders, ", "))
	res, err := q.tx.ExecContext(ctx, insertQuery, args...)
	if err != nil {
		return AddItemQuantityResult{}, err
	}
	lastID, err := res.LastInsertId()
	if err != nil {
		return AddItemQuantityResult{}, err
	}
	return r.commitItemQuantity(ctx, q.tx, q.tc, strconv.FormatInt(lastID, 10), delta, true)
}

// newQuantityRowValues returns fields overlaid with match, leaving out match columns that are
// empty (they stay NULL).
func newQuantityRowValues(match, fields map[string]string) map[string]string {
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
	return values
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
