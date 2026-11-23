package repo

import "context"

// AllColumnsViewID stands for the built-in "All Columns" view, which has no table_views row.
const AllColumnsViewID = -1

// SetDefaultExportView saves the view a table's Export page starts on: a view id, AllColumnsViewID,
// or 0 for the table's default view.
func (r *Repository) SetDefaultExportView(ctx context.Context, tableConfiguration string, viewID int) error {
	var value any
	if viewID != 0 {
		value = viewID
	}
	_, err := r.db.ExecContext(ctx, `UPDATE table_configurations SET default_export_view_id = ? WHERE name = ?`, value, tableConfiguration)
	return err
}
