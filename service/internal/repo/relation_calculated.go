package repo

import (
	"context"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"

	exprlang "github.com/expr-lang/expr"
)

var relationExprPlaceholderRE = regexp.MustCompile(`\{([a-zA-Z_][a-zA-Z0-9_]*)\.([a-zA-Z0-9_]+)\}`)

func (f RelationPullThroughField) IsCalculated() bool {
	return strings.TrimSpace(f.Expr) != ""
}

func (f RelationPullThroughField) IsPullThrough() bool {
	return !f.IsCalculated() &&
		strings.TrimSpace(f.FKColumn) != "" &&
		strings.TrimSpace(f.Field) != ""
}

// RelationCalcStorageKey matches frontend relationCalcStorageKey for synthetic columns.
func RelationCalcStorageKey(displayLabel string, index int) string {
	slug := strings.ToLower(strings.TrimSpace(displayLabel))
	slug = strings.ReplaceAll(slug, " ", "_")
	slug = sanitizeDatabaseIdentifier(slug)
	if slug == "" || slug == "items" {
		return fmt.Sprintf("__calc_%d", index)
	}
	return "__calc_" + slug
}

func normalizeRelationExpr(raw string) string {
	return relationExprPlaceholderRE.ReplaceAllString(raw, "$1.$2")
}

func tableConfigurationAliases(tc *TableConfig) []string {
	seen := make(map[string]struct{})
	out := appendTableAlias(nil, seen, tc.Name)
	if tc.Table.Valid {
		out = appendTableAlias(out, seen, tc.Table.String)
	}
	return out
}

func appendTableAlias(out []string, seen map[string]struct{}, name string) []string {
	key := tableAliasKey(name)
	if key == "" {
		return out
	}
	if _, ok := seen[key]; ok {
		return out
	}
	seen[key] = struct{}{}
	return append(out, key)
}

func tableAliasKey(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	key := strings.ToLower(sanitizeDatabaseIdentifier(name))
	if key == "" || key == "items" {
		return ""
	}
	return key
}

func aliasMatchesScope(alias string, scopeAliases []string) bool {
	key := strings.ToLower(sanitizeDatabaseIdentifier(alias))
	return slices.Contains(scopeAliases, key)
}

func itemFieldMap(item Item) map[string]any {
	out := make(map[string]any, len(item.Fields)+1)
	maps.Copy(out, item.Fields)
	if item.ID != "" {
		out["id"] = item.ID
	}
	return out
}

func lookupFieldValue(fields map[string]any, column string) (any, bool) {
	if fields == nil {
		return nil, false
	}
	if value, ok := fields[column]; ok {
		return value, true
	}
	return lookupFieldValueCaseInsensitive(fields, column)
}

func lookupFieldValueCaseInsensitive(fields map[string]any, column string) (any, bool) {
	lower := strings.ToLower(column)
	for key, value := range fields {
		if strings.ToLower(key) == lower {
			return value, true
		}
	}
	return nil, false
}

func coerceExprValue(raw any) any {
	if raw == nil {
		return nil
	}
	switch value := raw.(type) {
	case float64, float32, int, int64, int32, uint64, bool:
		return value
	case []byte:
		raw = string(value)
	}
	return coerceExprText(raw)
}

func coerceExprText(raw any) any {
	text := strings.TrimSpace(fmt.Sprintf("%v", raw))
	if text == "" {
		return ""
	}
	if number, err := strconv.ParseFloat(text, 64); err == nil {
		return number
	}
	return text
}

func mapFieldsForExpr(fields map[string]any) map[string]any {
	out := make(map[string]any, len(fields)*2)
	for key, value := range fields {
		coerced := coerceExprValue(value)
		out[key] = coerced
		lower := strings.ToLower(key)
		if lower != key {
			if _, exists := out[lower]; !exists {
				out[lower] = coerced
			}
		}
	}
	return out
}

type relationExprScope struct {
	columns map[string]struct{}
	aliases []string
}

func buildRelationExprScopes(
	ctx context.Context,
	r *Repository,
	sourceTableKey string,
) (sourceTC *TableConfig, scopes []relationExprScope, sourceRelations []TableRelation, err error) {
	sourceTC, err = r.GetTableConfiguration(ctx, sourceTableKey)
	if err != nil {
		return nil, nil, nil, err
	}
	sourceScope, err := r.relationExprScopeFor(ctx, sourceTC)
	if err != nil {
		return nil, nil, nil, err
	}
	scopes = append(scopes, sourceScope)

	sourceRelations, err = r.listTableRelationsForSourceConfiguration(ctx, sourceTableKey)
	if err != nil {
		return nil, nil, nil, err
	}
	for _, rel := range sourceRelations {
		refTC, err := r.GetTableConfiguration(ctx, rel.ReferencedTableKey)
		if err != nil {
			return nil, nil, nil, err
		}
		scope, err := r.relationExprScopeFor(ctx, refTC)
		if err != nil {
			return nil, nil, nil, err
		}
		scopes = append(scopes, scope)
	}
	return sourceTC, scopes, sourceRelations, nil
}

// relationExprScopeFor returns a table's expression aliases and lower-cased column names.
func (r *Repository) relationExprScopeFor(ctx context.Context, tc *TableConfig) (relationExprScope, error) {
	cols, err := r.ListColumns(ctx, tc)
	if err != nil {
		return relationExprScope{}, err
	}
	scope := relationExprScope{
		aliases: tableConfigurationAliases(tc),
		columns: make(map[string]struct{}, len(cols)),
	}
	for _, col := range cols {
		scope.columns[strings.ToLower(col.Name)] = struct{}{}
	}
	return scope, nil
}

func validateCalculatedExpr(expression string, scopes []relationExprScope) error {
	expression = strings.TrimSpace(expression)
	if expression == "" {
		return fmt.Errorf("expr is required for calculated columns")
	}
	matches := relationExprPlaceholderRE.FindAllStringSubmatch(expression, -1)
	if len(matches) == 0 {
		return fmt.Errorf("expr must use {table.column} placeholders")
	}
	for _, match := range matches {
		if err := checkExprPlaceholder(match[1], match[2], scopes); err != nil {
			return err
		}
	}
	env := sampleRelationExprEnv(scopes)
	program, err := exprlang.Compile(normalizeRelationExpr(expression), exprlang.Env(env))
	if err != nil {
		return fmt.Errorf("invalid expr: %w", err)
	}
	if _, err := exprlang.Run(program, env); err != nil {
		return fmt.Errorf("invalid expr: %w", err)
	}
	return nil
}

// checkExprPlaceholder checks a {table.column} placeholder names a table in scope and a column on it.
func checkExprPlaceholder(tableAlias, column string, scopes []relationExprScope) error {
	i := slices.IndexFunc(scopes, func(scope relationExprScope) bool { return aliasMatchesScope(tableAlias, scope.aliases) })
	if i < 0 {
		return fmt.Errorf("expr references unknown table %q", tableAlias)
	}
	if _, ok := scopes[i].columns[strings.ToLower(column)]; !ok {
		return fmt.Errorf("expr references unknown column %q on %q", column, tableAlias)
	}
	return nil
}

// sampleRelationExprEnv builds an expression environment with every in-scope column set to 1, for
// type-checking and trial-running expressions.
func sampleRelationExprEnv(scopes []relationExprScope) map[string]any {
	env := make(map[string]any)
	for _, scope := range scopes {
		row := make(map[string]any)
		for col := range scope.columns {
			row[col] = float64(1)
		}
		for _, alias := range scope.aliases {
			env[alias] = row
		}
	}
	return env
}

func (r *Repository) buildRelationExprEnv(
	ctx context.Context,
	sourceTC *TableConfig,
	item Item,
	sourceRelations []TableRelation,
	refCache map[string]map[string]any,
) (map[string]any, error) {
	env := make(map[string]any)
	setExprAliases(env, sourceTC, mapFieldsForExpr(itemFieldMap(item)))
	for _, rel := range sourceRelations {
		refTC, err := r.GetTableConfiguration(ctx, rel.ReferencedTableKey)
		if err != nil {
			return nil, err
		}
		setExprAliases(env, refTC, r.referencedExprFields(ctx, refTC, rel, item, refCache))
	}
	return env, nil
}

// setExprAliases exposes fields in env under every alias of tc.
func setExprAliases(env map[string]any, tc *TableConfig, fields map[string]any) {
	for _, alias := range tableConfigurationAliases(tc) {
		env[alias] = fields
	}
}

// referencedExprFields returns the expression fields of the row item references through rel, or an
// empty map when the foreign key is unset or the row cannot be loaded. Rows are cached in refCache.
func (r *Repository) referencedExprFields(ctx context.Context, refTC *TableConfig, rel TableRelation, item Item, refCache map[string]map[string]any) map[string]any {
	rawID, ok := lookupFieldValue(item.Fields, rel.SourceColumn)
	if !ok || rawID == nil || fmt.Sprintf("%v", rawID) == "" {
		return map[string]any{}
	}
	id := fmt.Sprintf("%v", rawID)
	cacheKey := rel.ReferencedTableKey + "\x00" + id
	if cached, ok := refCache[cacheKey]; ok {
		return cached
	}
	refFields := map[string]any{}
	if refItem, err := r.GetItemInTable(ctx, refTC, id); err == nil {
		refFields = mapFieldsForExpr(itemFieldMap(refItem))
	}
	refCache[cacheKey] = refFields
	return refFields
}

func evaluateRelationCalculatedExpr(expression string, env map[string]any) (string, error) {
	normalized := normalizeRelationExpr(strings.TrimSpace(expression))
	program, err := exprlang.Compile(
		normalized,
		exprlang.Env(env),
		exprlang.AsAny(),
	)
	if err != nil {
		return "", err
	}
	result, err := exprlang.Run(program, env)
	if err != nil {
		return "", err
	}
	if result == nil {
		return "", nil
	}
	return fmt.Sprintf("%v", result), nil
}

// ApplyAllSourceRelationCalculatedColumns evaluates calculated columns from every outgoing relation.
func (r *Repository) ApplyAllSourceRelationCalculatedColumns(
	ctx context.Context,
	sourceTableKey string,
	items []Item,
) ([]Item, error) {
	relations, err := r.listTableRelationsForSourceConfiguration(ctx, sourceTableKey)
	if err != nil {
		return nil, err
	}
	return r.applySourceRelationCalculatedColumns(ctx, sourceTableKey, items, relations)
}

func (r *Repository) applySourceRelationCalculatedColumns(ctx context.Context, sourceTableKey string, items []Item, relations []TableRelation) ([]Item, error) {
	out := items
	for _, rel := range relations {
		cols, err := parseRelationPullThroughJSON(rel.PullThroughJSON)
		if err != nil {
			return nil, err
		}
		out, err = r.ApplyRelationCalculatedColumns(ctx, sourceTableKey, cols, out)
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

// ApplyRelationCalculatedColumns evaluates calculated relation columns onto listed items.
func (r *Repository) ApplyRelationCalculatedColumns(
	ctx context.Context,
	sourceTableKey string,
	columns []RelationPullThroughField,
	items []Item,
) ([]Item, error) {
	hasCalculated := slices.ContainsFunc(columns, RelationPullThroughField.IsCalculated)
	if len(items) == 0 || !hasCalculated {
		return items, nil
	}
	sourceTC, _, sourceRelations, err := buildRelationExprScopes(ctx, r, sourceTableKey)
	if err != nil {
		return nil, err
	}
	refCache := make(map[string]map[string]any)
	out := make([]Item, 0, len(items))
	for _, item := range items {
		env, err := r.buildRelationExprEnv(ctx, sourceTC, item, sourceRelations, refCache)
		if err != nil {
			return nil, err
		}
		out = append(out, withCalculatedColumns(item, columns, env))
	}
	return out, nil
}

// withCalculatedColumns returns a copy of item with each calculated column evaluated against env
// (stored as "" when evaluation fails).
func withCalculatedColumns(item Item, columns []RelationPullThroughField, env map[string]any) Item {
	fields := make(map[string]any, len(item.Fields))
	maps.Copy(fields, item.Fields)
	item.Fields = fields
	for index, col := range columns {
		if !col.IsCalculated() {
			continue
		}
		value, err := evaluateRelationCalculatedExpr(col.Expr, env)
		if err != nil {
			value = ""
		}
		item.Fields[RelationCalcStorageKey(col.DisplayLabel, index)] = value
	}
	return item
}
