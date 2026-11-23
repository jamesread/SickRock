package repo

import (
	"context"
	"database/sql"
	"errors"
)

// DisplayName returns a user's display name, or "" when they have not set one (or do not exist).
func (r *Repository) DisplayName(ctx context.Context, userID int) (string, error) {
	var name sql.NullString
	err := r.db.GetContext(ctx, &name, `SELECT display_name FROM user_accounts WHERE id = ?`, userID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return name.String, err
}

// SetDisplayName saves a user's display name; "" clears it, so the username is shown instead.
func (r *Repository) SetDisplayName(ctx context.Context, userID int, name string) error {
	var value any
	if name != "" {
		value = name
	}
	_, err := r.db.ExecContext(ctx, `UPDATE user_accounts SET display_name = ? WHERE id = ?`, value, userID)
	return err
}
