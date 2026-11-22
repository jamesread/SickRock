package repo

import (
	"context"
	"database/sql"
	"errors"
)

// UIUsageTracking reports whether a user has opted in to UI usage tracking.
func (r *Repository) UIUsageTracking(ctx context.Context, userID int) (bool, error) {
	var enabled int
	err := r.db.GetContext(ctx, &enabled, `SELECT ui_usage_tracking FROM user_accounts WHERE id = ?`, userID)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return enabled != 0, err
}

// SetUIUsageTracking turns UI usage tracking on or off for a user.
func (r *Repository) SetUIUsageTracking(ctx context.Context, userID int, enabled bool) error {
	value := 0
	if enabled {
		value = 1
	}
	// RowsAffected is not checked: MySQL reports 0 when the value does not change.
	_, err := r.db.ExecContext(ctx, `UPDATE user_accounts SET ui_usage_tracking = ? WHERE id = ?`, value, userID)
	return err
}
