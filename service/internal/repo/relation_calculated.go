package repo

import (
	"context"
	"fmt"
	"regexp"
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
	out := make([]string, 0, 2)
	add := func(name string) {
		name = strings.TrimSpace(name)
		if name == "" {
			return
		}
		key := strings.ToLower(sanitizeDatabaseIdentifier(name))
		if key == "" || key == "items" {
			return
		}
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		out = append(out, key)
	}
	add(tc.Name)
	if tc.Table.Valid {
		add(tc.Table.String)
	}
	return out
}

func aliasMatchesScope(alias string, scopeAliases []string) bool {
	key := strings.ToLower(sanitizeDatabaseIdentifier(alias))
	for _, candidate := range scopeAliases {
		if key == candidate {
			return true
		}
	}
	return false
}

func itemFieldMap(item Item) map[string]interface{} {
	out := make(map[string]interface{}, len(item.Fields)+1)
	for key, value := range item.Fields {
		out[key] = value
	}
	if item.ID != "" {
		out["id"] = item.ID
	}
	return out
}

func lookupFieldValue(fields map[string]interface{}, column string) (interface{}, bool) {
	if fields == nil {
		return nil, false
	}
	if value, ok := fields[column]; ok {
		return value, true
	}
	lower := strings.ToLower(column)
	for key, value := range fields {
		if strings.ToLower(key) == lower {
			return value, true
		}
	}
	return nil, false
}

func coerceExprValue(raw interface{}) interface{} {
	if raw == nil {
		return nil
	}
	switch value := raw.(type) {
	case float64, float32, int, int64, int32, uint64, bool:
		return value
	case []byte:
		raw = string(value)
	}
	text := strings.TrimSpace(fmt.Sprintf("%v", raw))
	if text == "" {
		return ""
	}
	if number, err := strconv.ParseFloat(text, 64); err == nil {
		return number
	}
	return text
}

func mapFieldsForExpr(fields map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(fields)*2)
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
	aliases []string
	columns map[string]struct{}
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
	sourceCols, err := r.ListColumns(ctx, sourceTC)
	if err != nil {
		return nil, nil, nil, err
	}
	sourceScope := relationExprScope{
		aliases: tableConfigurationAliases(sourceTC),
		columns: make(map[string]struct{}, len(sourceCols)),
	}
	for _, col := range sourceCols {
		sourceScope.columns[strings.ToLower(col.Name)] = struct{}{}
	}
	scopes = append(scopes, sourceScope)

	relations, err := r.listTableRelationsForSourceConfiguration(ctx, sourceTableKey)
	if err != nil {
		return nil, nil, nil, err
	}
	for _, rel := range relations {
		refTC, err := r.GetTableConfiguration(ctx, rel.ReferencedTableKey)
		if err != nil {
			return nil, nil, nil, err
		}
		refCols, err := r.ListColumns(ctx, refTC)
		if err != nil {
			return nil, nil, nil, err
		}
		scope := relationExprScope{
			aliases: tableConfigurationAliases(refTC),
			columns: make(map[string]struct{}, len(refCols)),
		}
		for _, col := range refCols {
			scope.columns[strings.ToLower(col.Name)] = struct{}{}
		}
		scopes = append(scopes, scope)
	}
	sourceRelations, err = r.listTableRelationsForSourceConfiguration(ctx, sourceTableKey)
	return sourceTC, scopes, sourceRelations, err
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
		tableAlias := match[1]
		column := strings.ToLower(match[2])
		var matched bool
		for _, scope := range scopes {
			if !aliasMatchesScope(tableAlias, scope.aliases) {
				continue
			}
			if _, ok := scope.columns[column]; !ok {
				return fmt.Errorf("expr references unknown column %q on %q", match[2], tableAlias)
			}
			matched = true
			break
		}
		if !matched {
			return fmt.Errorf("expr references unknown table %q", tableAlias)
		}
	}
	normalized := normalizeRelationExpr(expression)
	env := make(map[string]interface{})
	for _, scope := range scopes {
		row := make(map[string]interface{})
		for col := range scope.columns {
			row[col] = float64(1)
		}
		for _, alias := range scope.aliases {
			env[alias] = row
		}
	}
	program, err := exprlang.Compile(normalized, exprlang.Env(env))
	if err != nil {
		return fmt.Errorf("invalid expr: %w", err)
	}
	if _, err := exprlang.Run(program, env); err != nil {
		return fmt.Errorf("invalid expr: %w", err)
	}
	return nil
}

func (r *Repository) buildRelationExprEnv(
	ctx context.Context,
	sourceTC *TableConfig,
	item Item,
	sourceRelations []TableRelation,
	refCache map[string]map[string]interface{},
) (map[string]interface{}, error) {
	env := make(map[string]interface{})
	sourceFields := mapFieldsForExpr(itemFieldMap(item))
	for _, alias := range tableConfigurationAliases(sourceTC) {
		env[alias] = sourceFields
	}
	for _, rel := range sourceRelations {
		refTC, err := r.GetTableConfiguration(ctx, rel.ReferencedTableKey)
		if err != nil {
			return nil, err
		}
		rawID, ok := lookupFieldValue(item.Fields, rel.SourceColumn)
		if !ok || rawID == nil || fmt.Sprintf("%v", rawID) == "" {
			empty := map[string]interface{}{}
			for _, alias := range tableConfigurationAliases(refTC) {
				env[alias] = empty
			}
			continue
		}
		id := fmt.Sprintf("%v", rawID)
		cacheKey := rel.ReferencedTableKey + "\x00" + id
		var refFields map[string]interface{}
		if cached, ok := refCache[cacheKey]; ok {
			refFields = cached
		} else {
			refItem, err := r.GetItemInTable(ctx, refTC, id)
			if err != nil {
				refFields = map[string]interface{}{}
			} else {
				refFields = mapFieldsForExpr(itemFieldMap(refItem))
			}
			refCache[cacheKey] = refFields
		}
		for _, alias := range tableConfigurationAliases(refTC) {
			env[alias] = refFields
		}
	}
	return env, nil
}

func evaluateRelationCalculatedExpr(expression string, env map[string]interface{}) (string, error) {
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
	hasCalculated := false
	for _, col := range columns {
		if col.IsCalculated() {
			hasCalculated = true
			break
		}
	}
	if len(items) == 0 || !hasCalculated {
		return items, nil
	}
	sourceTC, _, sourceRelations, err := buildRelationExprScopes(ctx, r, sourceTableKey)
	if err != nil {
		return nil, err
	}
	refCache := make(map[string]map[string]interface{})
	out := make([]Item, 0, len(items))
	for _, item := range items {
		env, err := r.buildRelationExprEnv(ctx, sourceTC, item, sourceRelations, refCache)
		if err != nil {
			return nil, err
		}
		copyItem := item
		if copyItem.Fields == nil {
			copyItem.Fields = make(map[string]interface{})
		} else {
			fields := make(map[string]interface{}, len(copyItem.Fields))
			for key, value := range copyItem.Fields {
				fields[key] = value
			}
			copyItem.Fields = fields
		}
		for index, col := range columns {
			if !col.IsCalculated() {
				continue
			}
			key := RelationCalcStorageKey(col.DisplayLabel, index)
			value, err := evaluateRelationCalculatedExpr(col.Expr, env)
			if err != nil {
				copyItem.Fields[key] = ""
				continue
			}
			copyItem.Fields[key] = value
		}
		out = append(out, copyItem)
	}
	return out, nil
}
