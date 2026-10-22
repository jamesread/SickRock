package repo

import (
	"context"
)

// Queries used by health checks (internal/health) to find broken configuration.

// BrokenNavigationLink is a table_navigation row whose target is missing.
type BrokenNavigationLink struct {
	Name               string `db:"name"`
	Title              string `db:"title"`
	ID                 int    `db:"id"`
	TableConfiguration int    `db:"table_configuration"`
	DashboardID        int    `db:"dashboard_id"`
	WorkflowID         int    `db:"workflow_id"`
	MissingTable       bool   `db:"missing_table"`
	MissingDashboard   bool   `db:"missing_dashboard"`
	MissingWorkflow    bool   `db:"missing_workflow"`
}

// FindBrokenNavigationLinks returns navigation links pointing at a deleted table configuration,
// dashboard or workflow, or at nothing at all.
func (r *Repository) FindBrokenNavigationLinks(ctx context.Context) ([]BrokenNavigationLink, error) {
	var links []BrokenNavigationLink
	err := r.db.SelectContext(ctx, &links, `
		SELECT tn.id, COALESCE(tn.name, '') AS name, COALESCE(tn.title, '') AS title,
			COALESCE(tn.table_configuration, 0) AS table_configuration,
			COALESCE(tn.dashboard_id, 0) AS dashboard_id,
			COALESCE(tn.workflow_id, 0) AS workflow_id,
			CASE WHEN COALESCE(tn.table_configuration, 0) > 0 AND tc.id IS NULL THEN 1 ELSE 0 END AS missing_table,
			CASE WHEN COALESCE(tn.dashboard_id, 0) > 0 AND td.id IS NULL THEN 1 ELSE 0 END AS missing_dashboard,
			CASE WHEN COALESCE(tn.workflow_id, 0) > 0 AND tw.id IS NULL THEN 1 ELSE 0 END AS missing_workflow
		FROM table_navigation tn
		LEFT JOIN table_configurations tc ON tn.table_configuration = tc.id
		LEFT JOIN table_dashboards td ON tn.dashboard_id = td.id
		LEFT JOIN table_workflows tw ON tn.workflow_id = tw.id
		WHERE (COALESCE(tn.table_configuration, 0) > 0 AND tc.id IS NULL)
			OR (COALESCE(tn.dashboard_id, 0) > 0 AND td.id IS NULL)
			OR (COALESCE(tn.workflow_id, 0) > 0 AND tw.id IS NULL)
			OR (COALESCE(tn.table_configuration, 0) = 0 AND COALESCE(tn.dashboard_id, 0) = 0 AND COALESCE(tn.workflow_id, 0) = 0)
		ORDER BY tn.id`)
	return links, err
}

// OrphanedBookmark is a user bookmark whose navigation link no longer exists.
type OrphanedBookmark struct {
	ID             int `db:"id"`
	UserID         int `db:"user_id"`
	NavigationItem int `db:"navigation_item"`
}

func (r *Repository) FindOrphanedBookmarks(ctx context.Context) ([]OrphanedBookmark, error) {
	var bookmarks []OrphanedBookmark
	err := r.db.SelectContext(ctx, &bookmarks, `
		SELECT ub.id, ub.user AS user_id, ub.navigation_item
		FROM table_user_bookmarks ub
		LEFT JOIN table_navigation tn ON ub.navigation_item = tn.id
		WHERE tn.id IS NULL
		ORDER BY ub.id`)
	return bookmarks, err
}

// DuplicateTableConfigurationName is a name used by more than one table configuration.
type DuplicateTableConfigurationName struct {
	Name  string `db:"name"`
	Count int    `db:"count"`
	MinID int    `db:"min_id"`
}

func (r *Repository) FindDuplicateTableConfigurationNames(ctx context.Context) ([]DuplicateTableConfigurationName, error) {
	var dups []DuplicateTableConfigurationName
	err := r.db.SelectContext(ctx, &dups, `
		SELECT name, COUNT(*) AS count, MIN(id) AS min_id
		FROM table_configurations
		GROUP BY name
		HAVING COUNT(*) > 1
		ORDER BY name`)
	return dups, err
}

// TableConfigurationTarget is where a table configuration's data lives.
type TableConfigurationTarget struct {
	Name  string `db:"name"`
	Db    string `db:"db"`
	Table string `db:"table_name"`
	ID    int    `db:"id"`
}

// ListTableConfigurationTargets returns every table configuration's database and physical table
// (the table defaults to the configuration name).
func (r *Repository) ListTableConfigurationTargets(ctx context.Context) ([]TableConfigurationTarget, error) {
	var targets []TableConfigurationTarget
	err := r.db.SelectContext(ctx, &targets,
		"SELECT id, name, COALESCE(db, '') AS db, COALESCE(`table`, '') AS table_name FROM table_configurations ORDER BY id")
	for i := range targets {
		if targets[i].Table == "" {
			targets[i].Table = targets[i].Name
		}
	}
	return targets, err
}

// TableColumnNames returns the columns of a physical table, or nil when the table does not
// exist. On MySQL an empty database means the connection's current database.
func (r *Repository) TableColumnNames(ctx context.Context, database, table string) ([]string, error) {
	var columns []string
	var err error
	if r.db.DriverName() == "mysql" {
		if database == "" {
			err = r.db.SelectContext(ctx, &columns,
				"SELECT COLUMN_NAME FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ? ORDER BY ORDINAL_POSITION", table)
		} else {
			err = r.db.SelectContext(ctx, &columns,
				"SELECT COLUMN_NAME FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ? ORDER BY ORDINAL_POSITION", database, table)
		}
	} else {
		err = r.db.SelectContext(ctx, &columns, "SELECT name FROM pragma_table_info(?)", table)
	}
	return columns, err
}

// ListAllTableRelations returns every stored relation.
func (r *Repository) ListAllTableRelations(ctx context.Context) ([]TableRelation, error) {
	var relations []TableRelation
	err := r.db.SelectContext(ctx, &relations, `
		SELECT id, constraint_name, source_table_key, source_column, referenced_table_key, referenced_column,
			on_delete_action, on_update_action, COALESCE(display_name, '') AS display_name
		FROM table_relations
		ORDER BY id`)
	return relations, err
}
