package repo

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
)

type ResourceGrantAction string

const (
	ResourceActionView   ResourceGrantAction = "view"
	ResourceActionInsert ResourceGrantAction = "insert"
	ResourceActionEdit   ResourceGrantAction = "edit"
	ResourceActionDelete ResourceGrantAction = "delete"
	ResourceActionStart  ResourceGrantAction = "start"
)

func grantColumn(action ResourceGrantAction) (string, error) {
	switch action {
	case ResourceActionView:
		return "can_view", nil
	case ResourceActionInsert:
		return "can_insert", nil
	case ResourceActionEdit:
		return "can_edit", nil
	case ResourceActionDelete:
		return "can_delete", nil
	case ResourceActionStart:
		return "can_start", nil
	default:
		return "", fmt.Errorf("unknown grant action")
	}
}

func (r *Repository) UserHasResourceGrant(ctx context.Context, userID int, resourceType, resourceKey string, action ResourceGrantAction) (bool, error) {
	if userID <= 0 || resourceType == "" || resourceKey == "" {
		return false, nil
	}
	col, err := grantColumn(action)
	if err != nil {
		return false, err
	}

	query := fmt.Sprintf(`
		SELECT 1
		FROM rbac_resource_grants g
		INNER JOIN user_group_memberships m ON m.user_group_id = g.user_group_id
		WHERE m.user_account_id = ?
		  AND g.resource_type = ?
		  AND g.resource_key = ?
		  AND g.%s != 0
		LIMIT 1`, col)

	var one int
	err = r.db.GetContext(ctx, &one, query, userID, resourceType, resourceKey)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (r *Repository) UserHasDashboardGrant(ctx context.Context, userID int, dashboardName string, action ResourceGrantAction) (bool, error) {
	return r.UserHasResourceGrant(ctx, userID, "dashboard", dashboardName, action)
}

// EnsureEveryoneDashboardGrant grants the Everyone group view access to a dashboard (matches migration seed).
func (r *Repository) EnsureEveryoneDashboardGrant(ctx context.Context, dashboardName string) error {
	if strings.TrimSpace(dashboardName) == "" {
		return fmt.Errorf("dashboard name is required")
	}
	var groupID int
	if err := r.db.GetContext(ctx, &groupID, `SELECT id FROM user_groups WHERE name = 'Everyone' LIMIT 1`); err != nil {
		return err
	}
	switch r.db.DriverName() {
	case "mysql":
		_, err := r.db.ExecContext(ctx, `
			INSERT INTO rbac_resource_grants (user_group_id, resource_type, resource_key, can_view, can_insert, can_edit, can_delete, can_start)
			VALUES (?, 'dashboard', ?, 1, 0, 0, 0, 0)
			ON DUPLICATE KEY UPDATE can_view = 1`, groupID, dashboardName)
		return err
	default:
		_, err := r.db.ExecContext(ctx, `
			INSERT OR IGNORE INTO rbac_resource_grants (user_group_id, resource_type, resource_key, can_view, can_insert, can_edit, can_delete, can_start)
			VALUES (?, 'dashboard', ?, 1, 0, 0, 0, 0)`, groupID, dashboardName)
		return err
	}
}

func (r *Repository) UserHasWorkflowGrant(ctx context.Context, userID int, workflowID int, action ResourceGrantAction) (bool, error) {
	return r.UserHasResourceGrant(ctx, userID, "workflow", strconv.Itoa(workflowID), action)
}
