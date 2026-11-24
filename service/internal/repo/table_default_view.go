package repo

import (
	"context"
	"fmt"
)

// SetTableDefaultView makes viewID the table's default view for everyone (table_views.is_default)
// and every other view of the table not the default; viewID 0 leaves the table without one.
func (r *Repository) SetTableDefaultView(ctx context.Context, tableName string, viewID int) error {
	t := sanitizeDatabaseIdentifier(tableName)
	if viewID > 0 {
		if err := r.validateUserDefaultTableView(ctx, t, viewID); err != nil {
			return err
		}
	} else if viewID < 0 {
		return fmt.Errorf("view id must be a view of %s, or 0 for none", tableName)
	}
	_, err := r.db.ExecContext(ctx, "UPDATE table_views SET is_default = (id = ?) WHERE table_name = ?", viewID, t)
	return err
}
