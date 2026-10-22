package repo

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
)

// IsRelationSyntheticColumn reports client-only relation embed columns (not stored in the physical table).
func IsRelationSyntheticColumn(name string) bool {
	return strings.HasPrefix(name, "__pt_") || strings.HasPrefix(name, "__calc_")
}

// PullThroughStorageKey is the client-side synthetic column name for a pull-through field.
func PullThroughStorageKey(fkColumn, field string) string {
	return fmt.Sprintf("__pt_%s_%s", fkColumn, field)
}

// ListRelationSyntheticColumnKeys returns synthetic column names configured on outgoing relations.
func (r *Repository) ListRelationSyntheticColumnKeys(ctx context.Context, sourceTableKey string) ([]string, error) {
	relations, err := r.listTableRelationsForSourceConfiguration(ctx, sourceTableKey)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0)
	for _, rel := range relations {
		cols, err := parseRelationPullThroughJSON(rel.PullThroughJSON)
		if err != nil {
			return nil, err
		}
		for index, col := range cols {
			if key := relationSyntheticColumnKey(col, index); key != "" && !slices.Contains(out, key) {
				out = append(out, key)
			}
		}
	}
	return out, nil
}

// relationSyntheticColumnKey returns the storage key of a calculated or pull-through column, or "".
func relationSyntheticColumnKey(col RelationPullThroughField, index int) string {
	switch {
	case col.IsCalculated():
		return RelationCalcStorageKey(col.DisplayLabel, index)
	case col.IsPullThrough():
		return PullThroughStorageKey(col.FKColumn, col.Field)
	}
	return ""
}

// RelationPullThroughField is a pull-through column or a calculated relation column (stored in pull_through_json).
type RelationPullThroughField struct {
	FKColumn     string `json:"fk_column,omitempty"`
	Field        string `json:"field,omitempty"`
	DisplayLabel string `json:"display_label,omitempty"`
	Expr         string `json:"expr,omitempty"`
}

// ParseRelationPullThroughJSON decodes pull_through_json (pull-through and calculated columns).
func ParseRelationPullThroughJSON(raw sql.NullString) ([]RelationPullThroughField, error) {
	return parseRelationPullThroughJSON(raw)
}

func parseRelationPullThroughJSON(raw sql.NullString) ([]RelationPullThroughField, error) {
	if !raw.Valid {
		return nil, nil
	}
	text := strings.TrimSpace(raw.String)
	if text == "" {
		return nil, nil
	}
	var out []RelationPullThroughField
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		return nil, fmt.Errorf("invalid pull_through_json: %w", err)
	}
	return out, nil
}

func marshalRelationPullThroughJSON(fields []RelationPullThroughField) (any, error) {
	if len(fields) == 0 {
		return nil, nil
	}
	b, err := json.Marshal(fields)
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

func validateRelationPullThroughFields(ctx context.Context, r *Repository, sourceTableKey string, fields []RelationPullThroughField) error {
	if len(fields) == 0 {
		return nil
	}
	_, scopes, relations, err := buildRelationExprScopes(ctx, r, sourceTableKey)
	if err != nil {
		return err
	}
	v := &relationFieldValidator{
		r:               r,
		sourceTableKey:  sourceTableKey,
		scopes:          scopes,
		refKeyByColumn:  make(map[string]string, len(relations)),
		seenPullThrough: make(map[string]struct{}, len(fields)),
		seenCalculated:  make(map[string]struct{}, len(fields)),
	}
	for _, rel := range relations {
		v.refKeyByColumn[strings.ToLower(rel.SourceColumn)] = rel.ReferencedTableKey
	}
	for i, pt := range fields {
		if err := v.validate(ctx, i, pt); err != nil {
			return err
		}
	}
	return nil
}

// relationFieldValidator validates a relation's pull-through and calculated entries, tracking duplicates.
type relationFieldValidator struct {
	r               *Repository
	refKeyByColumn  map[string]string
	seenPullThrough map[string]struct{}
	seenCalculated  map[string]struct{}
	sourceTableKey  string
	scopes          []relationExprScope
}

func (v *relationFieldValidator) validate(ctx context.Context, i int, pt RelationPullThroughField) error {
	switch {
	case pt.IsCalculated():
		return validateCalculatedEntry(i, pt, v.scopes, v.seenCalculated)
	case pt.IsPullThrough():
		return validatePullThroughEntry(ctx, v.r, v.sourceTableKey, pt, v.refKeyByColumn, v.seenPullThrough)
	}
	return fmt.Errorf("entry %d: set fk_column+field or display_label+expr", i+1)
}

// validateCalculatedEntry checks calculated entry i has a label and a valid expression, and is not a duplicate.
func validateCalculatedEntry(i int, pt RelationPullThroughField, scopes []relationExprScope, seen map[string]struct{}) error {
	label := strings.TrimSpace(pt.DisplayLabel)
	expr := strings.TrimSpace(pt.Expr)
	if label == "" {
		return fmt.Errorf("calculated entry %d: display_label is required", i+1)
	}
	if err := validateCalculatedExpr(expr, scopes); err != nil {
		return fmt.Errorf("calculated entry %d: %w", i+1, err)
	}
	key := strings.ToLower(label) + "\x00" + strings.ToLower(expr)
	if _, ok := seen[key]; ok {
		return fmt.Errorf("duplicate calculated column %q", label)
	}
	seen[key] = struct{}{}
	return nil
}

// validatePullThroughEntry checks a pull-through entry is not a duplicate, its fk_column is a foreign
// key on the source table, and its field is a column of the referenced table.
func validatePullThroughEntry(ctx context.Context, r *Repository, sourceTableKey string, pt RelationPullThroughField, refKeyByColumn map[string]string, seen map[string]struct{}) error {
	fkCol := strings.TrimSpace(pt.FKColumn)
	field := strings.TrimSpace(pt.Field)
	key := strings.ToLower(fkCol) + "\x00" + strings.ToLower(field)
	if _, ok := seen[key]; ok {
		return fmt.Errorf("duplicate pull-through: %s.%s", fkCol, field)
	}
	seen[key] = struct{}{}
	refKey, ok := refKeyByColumn[strings.ToLower(fkCol)]
	if !ok {
		return fmt.Errorf("pull-through fk_column %q is not a foreign key on table %q", fkCol, sourceTableKey)
	}
	refTC, err := r.GetTableConfiguration(ctx, refKey)
	if err != nil {
		return fmt.Errorf("pull-through %s: referenced table: %w", fkCol, err)
	}
	cols, err := r.ListColumns(ctx, refTC)
	if err != nil {
		return fmt.Errorf("pull-through %s: load referenced columns: %w", fkCol, err)
	}
	if !slices.ContainsFunc(cols, func(c FieldSpec) bool { return strings.EqualFold(c.Name, field) }) {
		return fmt.Errorf("pull-through field %q is not on referenced table for %s", field, fkCol)
	}
	return nil
}

// UpdateTableRelation updates display_name and pull-through column config for a relation.
func (r *Repository) UpdateTableRelation(ctx context.Context, constraintName, displayName string, pullThrough []RelationPullThroughField) error {
	constraintName = strings.TrimSpace(constraintName)
	if constraintName == "" {
		return fmt.Errorf("constraint name is required")
	}
	rel, err := r.getTableRelationByConstraintName(ctx, constraintName)
	if err != nil {
		return err
	}
	if err = validateRelationPullThroughFields(ctx, r, rel.SourceTableKey, pullThrough); err != nil {
		return err
	}
	pullArg, err := marshalRelationPullThroughJSON(pullThrough)
	if err != nil {
		return err
	}
	res, err := r.db.ExecContext(ctx, `
		UPDATE table_relations SET display_name = ?, pull_through_json = ? WHERE constraint_name = ?`,
		nilIfEmpty(strings.TrimSpace(displayName)), pullArg, constraintName)
	if err != nil {
		return err
	}
	return requireRowsAffected(res, "table relation not found")
}

// requireRowsAffected fails with notFound when a statement changed no rows.
func requireRowsAffected(res sql.Result, notFound string) error {
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return errors.New(notFound)
	}
	return nil
}
