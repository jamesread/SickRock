package health

import (
	"context"
	"fmt"

	"github.com/jamesread/SickRock/internal/repo"
	armhealth "github.com/jamesread/armature-health"
)

// TableConfigurationDuplicateNamesCheck finds table configuration names used more than once
// (e.g. system configurations re-inserted on every start when the unique index is missing).
type TableConfigurationDuplicateNamesCheck struct {
	Repo *repo.Repository
}

func (c TableConfigurationDuplicateNamesCheck) ID() string {
	return "table_configurations.duplicate_names"
}
func (c TableConfigurationDuplicateNamesCheck) Title() string {
	return "Duplicate table configuration names"
}

func (c TableConfigurationDuplicateNamesCheck) Run(ctx context.Context) ([]armhealth.Issue, error) {
	dups, err := c.Repo.FindDuplicateTableConfigurationNames(ctx)
	if err != nil {
		return nil, err
	}
	issues := make([]armhealth.Issue, 0, len(dups))
	for _, dup := range dups {
		issues = append(issues, armhealth.Issue{
			Key:      "name:" + dup.Name,
			Severity: armhealth.SeverityError,
			Title:    fmt.Sprintf("Table configuration %q exists %d times", dup.Name, dup.Count),
			Detail: fmt.Sprintf("Only one row is used (normally the lowest id, %d); the others are ignored and may hold stale settings. "+
				"Remove the extra rows (repointing any navigation links first) and make sure table_configurations.name has a unique index, "+
				"otherwise system configurations are duplicated on every restart.", dup.MinID),
			Link:  "/table/table_configurations",
			Count: dup.Count,
		})
	}
	return issues, nil
}

// TableConfigurationMissingTableCheck finds table configurations whose physical table does not
// exist in their database (e.g. a wrong "db" value).
type TableConfigurationMissingTableCheck struct {
	Repo *repo.Repository
}

func (c TableConfigurationMissingTableCheck) ID() string { return "table_configurations.missing_table" }
func (c TableConfigurationMissingTableCheck) Title() string {
	return "Table configurations pointing at missing tables"
}

func (c TableConfigurationMissingTableCheck) Run(ctx context.Context) ([]armhealth.Issue, error) {
	targets, err := c.Repo.ListTableConfigurationTargets(ctx)
	if err != nil {
		return nil, err
	}
	return missingTableConfigurationIssues(ctx, c.Repo, targets)
}

func missingTableConfigurationIssues(ctx context.Context, repository *repo.Repository, targets []repo.TableConfigurationTarget) ([]armhealth.Issue, error) {
	var issues []armhealth.Issue
	for _, target := range targets {
		issue, err := missingTableConfigurationIssue(ctx, repository, target)
		if err != nil {
			return nil, err
		}
		if issue != nil {
			issues = append(issues, *issue)
		}
	}
	return issues, nil
}

func missingTableConfigurationIssue(ctx context.Context, repository *repo.Repository, target repo.TableConfigurationTarget) (*armhealth.Issue, error) {
	columns, err := repository.TableColumnNames(ctx, target.Db, target.Table)
	if err != nil {
		return nil, fmt.Errorf("check table %s.%s: %w", target.Db, target.Table, err)
	}
	if len(columns) > 0 {
		return nil, nil
	}
	issue := armhealth.Issue{
		Key:      fmt.Sprintf("tc:%d", target.ID),
		Severity: armhealth.SeverityError,
		Title:    fmt.Sprintf("Table configuration %q points at a missing table", target.Name),
		Detail: fmt.Sprintf("Configuration #%d uses %s, which does not exist, so the table cannot be listed or edited. "+
			"Correct its db/table settings, or delete the configuration if the table was removed on purpose.", target.ID, missingTableWhere(target)),
		Link: fmt.Sprintf("/table/table_configurations/%d/edit", target.ID),
	}
	return &issue, nil
}

func missingTableWhere(target repo.TableConfigurationTarget) string {
	if target.Db != "" {
		return fmt.Sprintf("table %q in database %q", target.Table, target.Db)
	}
	return fmt.Sprintf("table %q", target.Table)
}
