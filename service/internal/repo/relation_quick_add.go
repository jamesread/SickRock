package repo

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
)

// RelationQuickAdd configures the inline quick-add row of a relation's related rows panel, so
// child rows (e.g. a recipe's ingredients) can be entered one after another on the parent's page.
type RelationQuickAdd struct {
	// Columns of the related table shown in the row, in order; empty for every writable column.
	Columns []string `json:"columns"`
	Enabled bool     `json:"enabled"`
	// CreateMissing lets a name typed into a relation column that matches no row create that row.
	CreateMissing bool `json:"createMissing"`
}

// NormalizeRelationQuickAddJSON validates quick add settings and returns them in canonical form;
// blank text clears them (returns "").
func NormalizeRelationQuickAddJSON(raw string) (string, error) {
	if strings.TrimSpace(raw) == "" {
		return "", nil
	}
	var qa RelationQuickAdd
	if err := json.Unmarshal([]byte(raw), &qa); err != nil {
		return "", fmt.Errorf("quick add settings: %w", err)
	}
	qa.Columns = cleanQuickAddColumns(qa.Columns)
	out, err := json.Marshal(qa)
	return string(out), err
}

// cleanQuickAddColumns trims column names, dropping blanks and repeats.
func cleanQuickAddColumns(names []string) []string {
	columns := make([]string, 0, len(names))
	for _, col := range names {
		if col = strings.TrimSpace(col); col == "" {
			continue
		}
		if col = sanitizeDatabaseIdentifier(col); !slices.Contains(columns, col) {
			columns = append(columns, col)
		}
	}
	return columns
}

// SetTableRelationQuickAdd stores a relation's quick add settings (blank clears them).
func (r *Repository) SetTableRelationQuickAdd(ctx context.Context, constraintName, raw string) error {
	normalized, err := NormalizeRelationQuickAddJSON(raw)
	if err != nil {
		return err
	}
	res, err := r.db.ExecContext(ctx, `UPDATE table_relations SET quick_add_json = ? WHERE constraint_name = ?`,
		nilIfEmpty(normalized), strings.TrimSpace(constraintName))
	if err != nil {
		return err
	}
	return r.requireRelationUpdated(ctx, res, strings.TrimSpace(constraintName))
}
