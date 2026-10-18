package health

import (
	"context"
	"fmt"
	"slices"

	"github.com/jamesread/SickRock/internal/repo"
)

// RelationsDanglingCheck finds relations whose tables or columns no longer exist, which break
// related-row panels and pull-through columns.
type RelationsDanglingCheck struct {
	Repo *repo.Repository
}

func (c RelationsDanglingCheck) ID() string    { return "relations.dangling" }
func (c RelationsDanglingCheck) Title() string { return "Relations to missing tables or columns" }

func (c RelationsDanglingCheck) Run(ctx context.Context) ([]Issue, error) {
	relations, err := c.Repo.ListAllTableRelations(ctx)
	if err != nil {
		return nil, err
	}
	if len(relations) == 0 {
		return nil, nil
	}
	targets, err := c.Repo.ListTableConfigurationTargets(ctx)
	if err != nil {
		return nil, err
	}
	targetByName := map[string]repo.TableConfigurationTarget{}
	for _, target := range targets {
		if _, ok := targetByName[target.Name]; !ok {
			targetByName[target.Name] = target
		}
	}
	columnsByName := map[string][]string{}
	columnsOf := func(name string) ([]string, error) {
		if cols, ok := columnsByName[name]; ok {
			return cols, nil
		}
		target := targetByName[name]
		cols, err := c.Repo.TableColumnNames(ctx, target.Db, target.Table)
		columnsByName[name] = cols
		return cols, err
	}

	var issues []Issue
	for _, rel := range relations {
		problem, err := c.relationProblem(rel, targetByName, columnsOf)
		if err != nil {
			return nil, err
		}
		if problem == "" {
			continue
		}
		link := "/table/table_configurations"
		if _, ok := targetByName[rel.SourceTableKey]; ok {
			link = fmt.Sprintf("/table/%s/foreign-keys", rel.SourceTableKey)
		}
		issues = append(issues, Issue{
			Key:      "relation:" + rel.ConstraintName,
			Severity: SeverityWarning,
			Title: fmt.Sprintf("Relation %s.%s → %s.%s: %s",
				rel.SourceTableKey, rel.SourceColumn, rel.ReferencedTableKey, rel.ReferencedColumn, problem),
			Detail: fmt.Sprintf("Relation %q no longer matches the schema, so related rows and pull-through columns using it fail. "+
				"Delete the relation or restore the table/column.", rel.ConstraintName),
			Link: link,
		})
	}
	return issues, nil
}

func (c RelationsDanglingCheck) relationProblem(
	rel repo.TableRelation,
	targets map[string]repo.TableConfigurationTarget,
	columnsOf func(string) ([]string, error),
) (string, error) {
	for _, side := range []struct{ table, column, label string }{
		{rel.SourceTableKey, rel.SourceColumn, "source"},
		{rel.ReferencedTableKey, rel.ReferencedColumn, "referenced"},
	} {
		if _, ok := targets[side.table]; !ok {
			return fmt.Sprintf("the %s table configuration %q was deleted", side.label, side.table), nil
		}
		cols, err := columnsOf(side.table)
		if err != nil {
			return "", err
		}
		if len(cols) == 0 {
			return fmt.Sprintf("the %s table %q does not exist", side.label, side.table), nil
		}
		if !slices.Contains(cols, side.column) {
			return fmt.Sprintf("the %s column %q no longer exists", side.label, side.column), nil
		}
	}
	return "", nil
}
