package health

import (
	"context"
	"fmt"
	"slices"

	"github.com/jamesread/SickRock/internal/repo"
	armhealth "github.com/jamesread/armature-health"
)

// RelationsDanglingCheck finds relations whose tables or columns no longer exist, which break
// related-row panels and pull-through columns.
type RelationsDanglingCheck struct {
	Repo *repo.Repository
}

func (c RelationsDanglingCheck) ID() string    { return "relations.dangling" }
func (c RelationsDanglingCheck) Title() string { return "Relations to missing tables or columns" }

func (c RelationsDanglingCheck) Run(ctx context.Context) ([]armhealth.Issue, error) {
	relations, err := c.Repo.ListAllTableRelations(ctx)
	if err != nil || len(relations) == 0 {
		return nil, err
	}
	targetByName, err := c.targetsByName(ctx)
	if err != nil {
		return nil, err
	}
	columnsOf := c.cachedColumnsOf(ctx, targetByName)

	var issues []armhealth.Issue
	for _, rel := range relations {
		problem, err := c.relationProblem(rel, targetByName, columnsOf)
		if err != nil {
			return nil, err
		}
		if problem != "" {
			issues = append(issues, danglingRelationIssue(rel, problem, targetByName))
		}
	}
	return issues, nil
}

// targetsByName returns each table configuration's physical table, keeping the first row per name.
func (c RelationsDanglingCheck) targetsByName(ctx context.Context) (map[string]repo.TableConfigurationTarget, error) {
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
	return targetByName, nil
}

// cachedColumnsOf returns a lookup of a table configuration's physical column names, querying each table once.
func (c RelationsDanglingCheck) cachedColumnsOf(ctx context.Context, targetByName map[string]repo.TableConfigurationTarget) func(string) ([]string, error) {
	columnsByName := map[string][]string{}
	return func(name string) ([]string, error) {
		if cols, ok := columnsByName[name]; ok {
			return cols, nil
		}
		target := targetByName[name]
		cols, err := c.Repo.TableColumnNames(ctx, target.Db, target.Table)
		columnsByName[name] = cols
		return cols, err
	}
}

func danglingRelationIssue(rel repo.TableRelation, problem string, targetByName map[string]repo.TableConfigurationTarget) armhealth.Issue {
	link := "/table/table_configurations"
	if _, ok := targetByName[rel.SourceTableKey]; ok {
		link = fmt.Sprintf("/table/%s/foreign-keys", rel.SourceTableKey)
	}
	return armhealth.Issue{
		Key:      "relation:" + rel.ConstraintName,
		Severity: armhealth.SeverityWarning,
		Title: fmt.Sprintf("Relation %s.%s → %s.%s: %s",
			rel.SourceTableKey, rel.SourceColumn, rel.ReferencedTableKey, rel.ReferencedColumn, problem),
		Detail: fmt.Sprintf("Relation %q no longer matches the schema, so related rows and pull-through columns using it fail. "+
			"Delete the relation or restore the table/column.", rel.ConstraintName),
		Link: link,
	}
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
		problem, err := relationSideProblem(side.table, side.column, side.label, targets, columnsOf)
		if err != nil || problem != "" {
			return problem, err
		}
	}
	return "", nil
}

func relationSideProblem(
	table, column, label string,
	targets map[string]repo.TableConfigurationTarget,
	columnsOf func(string) ([]string, error),
) (string, error) {
	if _, ok := targets[table]; !ok {
		return fmt.Sprintf("the %s table configuration %q was deleted", label, table), nil
	}
	cols, err := columnsOf(table)
	if err != nil {
		return "", err
	}
	return relationColumnProblem(label, table, column, cols), nil
}

func relationColumnProblem(label, table, column string, cols []string) string {
	if len(cols) == 0 {
		return fmt.Sprintf("the %s table %q does not exist", label, table)
	}
	if !slices.Contains(cols, column) {
		return fmt.Sprintf("the %s column %q no longer exists", label, column)
	}
	return ""
}
