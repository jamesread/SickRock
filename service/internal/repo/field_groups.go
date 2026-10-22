package repo

import (
	"context"
	"fmt"
	"strings"

	"github.com/jmoiron/sqlx"
)

// TableFieldGroup is one titled group of columns on the item view.
type TableFieldGroup struct {
	Title      string `db:"title"`
	ColumnName string `db:"column_name"`
	ID         int    `db:"id"`
	Ordinal    int    `db:"ordinal"`
	ColOrdinal int    `db:"col_ordinal"`
}

// FieldGroupInput replaces all field groups for a table configuration.
type FieldGroupInput struct {
	Title       string
	ColumnNames []string
	Ordinal     int
}

// GetItemFieldGroups returns field groups with columns ordered for item view.
func (r *Repository) GetItemFieldGroups(ctx context.Context, tableConfiguration string) ([]FieldGroupInput, error) {
	tableConfiguration = strings.TrimSpace(tableConfiguration)
	if tableConfiguration == "" {
		return nil, fmt.Errorf("table configuration is required")
	}
	var rows []TableFieldGroup
	err := r.db.SelectContext(ctx, &rows, `
		SELECT g.id, g.ordinal, g.title, c.column_name, c.ordinal AS col_ordinal
		FROM table_field_groups g
		LEFT JOIN table_field_group_columns c ON c.field_group_id = g.id
		WHERE g.table_configuration = ?
		ORDER BY g.ordinal ASC, g.id ASC, c.ordinal ASC, c.id ASC`, tableConfiguration)
	if err != nil {
		return nil, err
	}
	return groupFieldGroupRows(rows), nil
}

// groupFieldGroupRows folds joined group/column rows into groups, keeping row order.
func groupFieldGroupRows(rows []TableFieldGroup) []FieldGroupInput {
	out := make([]FieldGroupInput, 0)
	indexByID := make(map[int]int)
	for _, row := range rows {
		i, ok := indexByID[row.ID]
		if !ok {
			i = len(out)
			indexByID[row.ID] = i
			out = append(out, FieldGroupInput{Ordinal: row.Ordinal, Title: row.Title})
		}
		if row.ColumnName != "" {
			out[i].ColumnNames = append(out[i].ColumnNames, row.ColumnName)
		}
	}
	return out
}

func validateFieldGroups(groups []FieldGroupInput, allowedColumns map[string]struct{}) error {
	seenCol := make(map[string]struct{})
	for i, g := range groups {
		if strings.TrimSpace(g.Title) == "" {
			return fmt.Errorf("group %d: title is required", i+1)
		}
		if len(g.ColumnNames) == 0 {
			return fmt.Errorf("group %d: at least one column is required", i+1)
		}
		if err := validateFieldGroupColumns(i, g.ColumnNames, allowedColumns, seenCol); err != nil {
			return err
		}
	}
	return nil
}

// validateFieldGroupColumns checks group i's columns are set, allowed, and not already in another group.
func validateFieldGroupColumns(i int, columnNames []string, allowedColumns, seenCol map[string]struct{}) error {
	for j, col := range columnNames {
		name := strings.TrimSpace(col)
		if name == "" {
			return fmt.Errorf("group %d: column %d is empty", i+1, j+1)
		}
		if _, ok := allowedColumns[name]; !ok {
			return fmt.Errorf("group %d: unknown column %q", i+1, name)
		}
		if _, dup := seenCol[name]; dup {
			return fmt.Errorf("column %q appears in more than one group", name)
		}
		seenCol[name] = struct{}{}
	}
	return nil
}

// SetItemFieldGroups replaces all item view field groups for a table configuration.
func (r *Repository) SetItemFieldGroups(ctx context.Context, tableConfiguration string, groups []FieldGroupInput) error {
	tableConfiguration = strings.TrimSpace(tableConfiguration)
	if tableConfiguration == "" {
		return fmt.Errorf("table configuration is required")
	}
	allowed, err := r.fieldGroupAllowedColumns(ctx, tableConfiguration)
	if err != nil {
		return err
	}
	if err = validateFieldGroups(groups, allowed); err != nil {
		return err
	}

	return r.inTx(ctx, func(tx *sqlx.Tx) error {
		if _, err := tx.ExecContext(ctx, `
			DELETE FROM table_field_groups WHERE table_configuration = ?`, tableConfiguration); err != nil {
			return err
		}
		for i, g := range groups {
			if err := insertFieldGroup(ctx, tx, tableConfiguration, ordinalOrPosition(g.Ordinal, i), g); err != nil {
				return err
			}
		}
		return nil
	})
}

// fieldGroupAllowedColumns returns the column names a field group may reference: the table's
// physical columns plus its relation synthetic columns.
func (r *Repository) fieldGroupAllowedColumns(ctx context.Context, tableConfiguration string) (map[string]struct{}, error) {
	tc, err := r.GetTableConfiguration(ctx, tableConfiguration)
	if err != nil {
		return nil, err
	}
	cols, err := r.ListColumns(ctx, tc)
	if err != nil {
		return nil, err
	}
	syntheticKeys, err := r.ListRelationSyntheticColumnKeys(ctx, tableConfiguration)
	if err != nil {
		return nil, err
	}
	allowed := make(map[string]struct{}, len(cols)+len(syntheticKeys))
	for _, c := range cols {
		allowed[c.Name] = struct{}{}
	}
	for _, key := range syntheticKeys {
		allowed[key] = struct{}{}
	}
	return allowed, nil
}

func insertFieldGroup(ctx context.Context, tx *sqlx.Tx, tableConfiguration string, ordinal int, g FieldGroupInput) error {
	res, err := tx.ExecContext(ctx, `
		INSERT INTO table_field_groups (table_configuration, ordinal, title)
		VALUES (?, ?, ?)`,
		tableConfiguration, ordinal, strings.TrimSpace(g.Title))
	if err != nil {
		return err
	}
	groupID, err := res.LastInsertId()
	if err != nil {
		return err
	}
	for j, col := range g.ColumnNames {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO table_field_group_columns (field_group_id, column_name, ordinal)
			VALUES (?, ?, ?)`,
			groupID, strings.TrimSpace(col), j+1); err != nil {
			return err
		}
	}
	return nil
}

// ordinalOrPosition returns ordinal when set, otherwise the 1-based position of index i.
func ordinalOrPosition(ordinal, i int) int {
	if ordinal <= 0 {
		return i + 1
	}
	return ordinal
}
