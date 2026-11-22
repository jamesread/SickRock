package repo

import (
	"context"
	"database/sql"
	"errors"
)

// HomeLayout returns a user's home page layout JSON, or "" for the default layout.
func (r *Repository) HomeLayout(ctx context.Context, userID int) (string, error) {
	var layout sql.NullString
	err := r.db.GetContext(ctx, &layout, `SELECT home_layout_json FROM user_accounts WHERE id = ?`, userID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return layout.String, err
}

// SetHomeLayout saves a user's home page layout JSON; "" restores the default layout.
func (r *Repository) SetHomeLayout(ctx context.Context, userID int, layoutJSON string) error {
	var value any
	if layoutJSON != "" {
		value = layoutJSON
	}
	_, err := r.db.ExecContext(ctx, `UPDATE user_accounts SET home_layout_json = ? WHERE id = ?`, value, userID)
	return err
}
