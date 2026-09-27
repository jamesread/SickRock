package rolegrants

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/jmoiron/sqlx"
)

// Row is one effective role granted to a user via a specific group membership.
type Row struct {
	RoleID    int
	RoleName  string
	GroupID   int
	GroupName string
}

func ListForUser(ctx context.Context, db *sqlx.DB, userID int) ([]Row, error) {
	if userID <= 0 {
		return nil, fmt.Errorf("user_id is required")
	}
	const q = `
		SELECT r.id, r.name, g.id, g.name
		FROM rbac_roles r
		INNER JOIN rbac_group_roles gr ON gr.role_id = r.id
		INNER JOIN user_groups g ON g.id = gr.user_group_id
		INNER JOIN user_group_memberships ugm ON ugm.user_group_id = g.id
		WHERE ugm.user_account_id = ?
		ORDER BY r.name ASC, g.name ASC`

	rows, err := db.QueryxContext(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("list user role grants: %w", err)
	}
	defer rows.Close()

	out := make([]Row, 0, 8)
	for rows.Next() {
		var row Row
		if err := rows.Scan(&row.RoleID, &row.RoleName, &row.GroupID, &row.GroupName); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func UserIsSuperuser(ctx context.Context, db *sqlx.DB, userID int) (bool, error) {
	var one int
	err := db.GetContext(ctx, &one, `
		SELECT 1
		FROM rbac_roles r
		INNER JOIN rbac_group_roles gr ON gr.role_id = r.id
		INNER JOIN user_group_memberships ugm ON ugm.user_group_id = gr.user_group_id
		WHERE ugm.user_account_id = ? AND r.name = 'superuser'
		LIMIT 1`, userID)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}
