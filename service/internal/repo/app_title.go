package repo

import (
	"context"
	"database/sql"
	"errors"
)

// AppTitleSettingKey is the table_settings row holding the instance's app title.
const AppTitleSettingKey = "appTitle"

// GetAppTitle returns the app title setting, or "" when it is not set.
func (r *Repository) GetAppTitle(ctx context.Context) (string, error) {
	var title sql.NullString
	err := r.db.GetContext(ctx, &title, `SELECT string_val FROM table_settings WHERE setting_key = ?`, AppTitleSettingKey)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return title.String, nil
}
