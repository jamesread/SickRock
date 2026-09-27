package repo

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/jamesread/SickRock/internal/iam/systemroles"
)

type GroupTableRoleGrant struct {
	TableKey string
	RoleID   int
}

type TableAccess struct {
	CanView   bool
	CanInsert bool
	CanEdit   bool
	CanDelete bool
}

func tableActionPermission(action ResourceGrantAction) (string, error) {
	switch action {
	case ResourceActionView:
		return systemroles.PermissionTableView, nil
	case ResourceActionInsert:
		return systemroles.PermissionTableInsert, nil
	case ResourceActionEdit:
		return systemroles.PermissionTableEdit, nil
	case ResourceActionDelete:
		return systemroles.PermissionTableDelete, nil
	default:
		return "", fmt.Errorf("unknown table action")
	}
}

func (r *Repository) UserHasTableGrant(ctx context.Context, userID int, tableConfiguration string, action ResourceGrantAction) (bool, error) {
	if userID <= 0 || tableConfiguration == "" {
		return false, nil
	}
	perm, err := tableActionPermission(action)
	if err != nil {
		return false, err
	}
	const q = `
		SELECT 1
		FROM user_group_memberships m
		INNER JOIN rbac_table_group_roles tgr ON tgr.user_group_id = m.user_group_id
		INNER JOIN rbac_role_permissions rp ON rp.role_id = tgr.role_id
		INNER JOIN rbac_permissions p ON p.id = rp.permission_id
		WHERE m.user_account_id = ?
		  AND tgr.table_key = ?
		  AND p.name = ?
		LIMIT 1`
	var one int
	err = r.db.GetContext(ctx, &one, q, userID, tableConfiguration, perm)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (r *Repository) GetUserTableAccess(ctx context.Context, userID int, tableConfiguration string) (TableAccess, error) {
	out := TableAccess{}
	if userID <= 0 || tableConfiguration == "" {
		return out, nil
	}
	checks := []struct {
		action ResourceGrantAction
		dest   *bool
	}{
		{ResourceActionView, &out.CanView},
		{ResourceActionInsert, &out.CanInsert},
		{ResourceActionEdit, &out.CanEdit},
		{ResourceActionDelete, &out.CanDelete},
	}
	for _, c := range checks {
		ok, err := r.UserHasTableGrant(ctx, userID, tableConfiguration, c.action)
		if err != nil {
			return out, err
		}
		*c.dest = ok
	}
	return out, nil
}

func (r *Repository) ListGroupTableRoleGrants(ctx context.Context, groupID int) ([]GroupTableRoleGrant, error) {
	if groupID <= 0 {
		return nil, fmt.Errorf("group_id is required")
	}
	const q = `SELECT table_key, role_id FROM rbac_table_group_roles WHERE user_group_id = ? ORDER BY table_key`
	rows, err := r.db.QueryxContext(ctx, q, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]GroupTableRoleGrant, 0, 16)
	for rows.Next() {
		var g GroupTableRoleGrant
		if err := rows.Scan(&g.TableKey, &g.RoleID); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

func (r *Repository) SetGroupTableRoleGrants(ctx context.Context, groupID int, grants []GroupTableRoleGrant) error {
	if groupID <= 0 {
		return fmt.Errorf("group_id is required")
	}
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `DELETE FROM rbac_table_group_roles WHERE user_group_id = ?`, groupID); err != nil {
		return fmt.Errorf("clear table group roles: %w", err)
	}
	for _, g := range grants {
		if g.TableKey == "" || g.RoleID <= 0 {
			continue
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO rbac_table_group_roles (user_group_id, table_key, role_id) VALUES (?, ?, ?)`,
			groupID, g.TableKey, g.RoleID,
		); err != nil {
			return fmt.Errorf("insert table group role: %w", err)
		}
	}
	return tx.Commit()
}

type TableShareGrant struct {
	GroupID   int
	GroupName string
	RoleID    int
}

func (r *Repository) ListTableShareGrants(ctx context.Context, tableKey string) ([]TableShareGrant, error) {
	if tableKey == "" {
		return nil, fmt.Errorf("table_key is required")
	}
	const q = `
		SELECT g.id AS group_id, g.name AS group_name, COALESCE(tgr.role_id, 0) AS role_id
		FROM user_groups g
		LEFT JOIN rbac_table_group_roles tgr
			ON tgr.user_group_id = g.id AND tgr.table_key = ?
		ORDER BY g.name`
	rows, err := r.db.QueryxContext(ctx, q, tableKey)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]TableShareGrant, 0, 16)
	for rows.Next() {
		var row TableShareGrant
		if err := rows.Scan(&row.GroupID, &row.GroupName, &row.RoleID); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r *Repository) SetTableShareGrants(ctx context.Context, tableKey string, grants []TableShareGrant) error {
	if tableKey == "" {
		return fmt.Errorf("table_key is required")
	}
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `DELETE FROM rbac_table_group_roles WHERE table_key = ?`, tableKey); err != nil {
		return fmt.Errorf("clear table share grants: %w", err)
	}
	for _, g := range grants {
		if g.GroupID <= 0 || g.RoleID <= 0 {
			continue
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO rbac_table_group_roles (user_group_id, table_key, role_id) VALUES (?, ?, ?)`,
			g.GroupID, tableKey, g.RoleID,
		); err != nil {
			return fmt.Errorf("insert table share grant: %w", err)
		}
	}
	return tx.Commit()
}

func (r *Repository) EnsureEveryoneTableRole(ctx context.Context, tableConfiguration string, roleName string) error {
	if tableConfiguration == "" {
		return fmt.Errorf("table is required")
	}
	if roleName == "" {
		roleName = systemroles.RoleViewer
	}
	var groupID int
	if err := r.db.GetContext(ctx, &groupID, `SELECT id FROM user_groups WHERE name = 'Everyone' LIMIT 1`); err != nil {
		return err
	}
	var roleID int
	if err := r.db.GetContext(ctx, &roleID, `SELECT id FROM rbac_roles WHERE name = ? LIMIT 1`, roleName); err != nil {
		return err
	}
	switch r.db.DriverName() {
	case "mysql":
		_, err := r.db.ExecContext(ctx, `
			INSERT INTO rbac_table_group_roles (user_group_id, table_key, role_id)
			VALUES (?, ?, ?)
			ON DUPLICATE KEY UPDATE role_id = VALUES(role_id)`, groupID, tableConfiguration, roleID)
		return err
	default:
		_, err := r.db.ExecContext(ctx, `
			INSERT OR REPLACE INTO rbac_table_group_roles (user_group_id, table_key, role_id)
			VALUES (?, ?, ?)`, groupID, tableConfiguration, roleID)
		return err
	}
}
