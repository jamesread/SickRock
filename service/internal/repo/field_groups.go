package repo

import (
	"context"
	"fmt"
	"strings"
)

// TableFieldGroup is one titled group of columns on the item view.
type TableFieldGroup struct {
	ID         int    `db:"id"`
	Ordinal    int    `db:"ordinal"`
	Title      string `db:"title"`
	ColumnName string `db:"column_name"`
	ColOrdinal int    `db:"col_ordinal"`
}

// FieldGroupInput replaces all field groups for a table configuration.
type FieldGroupInput struct {
	Ordinal     int
	Title       string
	ColumnNames []string
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
	if len(rows) == 0 {
		return []FieldGroupInput{}, nil
	}
	type groupAcc struct {
		ordinal int
		title   string
		cols    []string
	}
	byID := make(map[int]*groupAcc)
	order := make([]int, 0)
	for _, row := range rows {
		acc, ok := byID[row.ID]
		if !ok {
			acc = &groupAcc{ordinal: row.Ordinal, title: row.Title}
			byID[row.ID] = acc
			order = append(order, row.ID)
		}
		if row.ColumnName != "" {
			acc.cols = append(acc.cols, row.ColumnName)
		}
	}
	out := make([]FieldGroupInput, 0, len(order))
	for _, id := range order {
		acc := byID[id]
		out = append(out, FieldGroupInput{
			Ordinal:     acc.ordinal,
			Title:       acc.title,
			ColumnNames: acc.cols,
		})
	}
	return out, nil
}

func validateFieldGroups(groups []FieldGroupInput, allowedColumns map[string]struct{}) error {
	seenCol := make(map[string]struct{})
	for i, g := range groups {
		title := strings.TrimSpace(g.Title)
		if title == "" {
			return fmt.Errorf("group %d: title is required", i+1)
		}
		if len(g.ColumnNames) == 0 {
			return fmt.Errorf("group %d: at least one column is required", i+1)
		}
		for j, col := range g.ColumnNames {
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
	}
	return nil
}

// SetItemFieldGroups replaces all item view field groups for a table configuration.
func (r *Repository) SetItemFieldGroups(ctx context.Context, tableConfiguration string, groups []FieldGroupInput) error {
	tableConfiguration = strings.TrimSpace(tableConfiguration)
	if tableConfiguration == "" {
		return fmt.Errorf("table configuration is required")
	}
	tc, err := r.GetTableConfiguration(ctx, tableConfiguration)
	if err != nil {
		return err
	}
	cols, err := r.ListColumns(ctx, tc)
	if err != nil {
		return err
	}
	allowed := make(map[string]struct{}, len(cols))
	for _, c := range cols {
		allowed[c.Name] = struct{}{}
	}
	syntheticKeys, err := r.ListRelationSyntheticColumnKeys(ctx, tableConfiguration)
	if err != nil {
		return err
	}
	for _, key := range syntheticKeys {
		allowed[key] = struct{}{}
	}
	if err := validateFieldGroups(groups, allowed); err != nil {
		return err
	}

	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `
		DELETE FROM table_field_groups WHERE table_configuration = ?`, tableConfiguration); err != nil {
		return err
	}

	if len(groups) == 0 {
		return tx.Commit()
	}

	for i, g := range groups {
		ord := g.Ordinal
		if ord <= 0 {
			ord = i + 1
		}
		res, err := tx.ExecContext(ctx, `
			INSERT INTO table_field_groups (table_configuration, ordinal, title)
			VALUES (?, ?, ?)`,
			tableConfiguration, ord, strings.TrimSpace(g.Title))
		if err != nil {
			return err
		}
		groupID, err := res.LastInsertId()
		if err != nil {
			return err
		}
		for j, col := range g.ColumnNames {
			colOrd := j + 1
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO table_field_group_columns (field_group_id, column_name, ordinal)
				VALUES (?, ?, ?)`,
				groupID, strings.TrimSpace(col), colOrd); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}
