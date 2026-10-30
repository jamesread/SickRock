package repo

import (
	"context"
	"fmt"

	log "github.com/sirupsen/logrus"
)

// systemTableRelation is a built-in relation between system table configurations, ensured at startup.
type systemTableRelation struct {
	displayName      string
	pullThroughJSON  string
	sourceTableKey   string
	sourceColumn     string
	referencedTable  string
	referencedColumn string
	onDelete         string
}

// systemTableRelations are inserted in order (a pull-through needs its fk_column's relation first).
var systemTableRelations = []systemTableRelation{
	{
		sourceTableKey: "table_app_navigation_links", sourceColumn: "navigation_item_id",
		referencedTable: "table_navigation", referencedColumn: "id", onDelete: "CASCADE",
	},
	{
		// Shows an app's links on its item page, named after their navigation item.
		sourceTableKey: "table_app_navigation_links", sourceColumn: "app_id",
		referencedTable: "table_apps", referencedColumn: "id", onDelete: "CASCADE",
		displayName:     "Navigation links",
		pullThroughJSON: `[{"fk_column":"navigation_item_id","field":"name"}]`,
	},
}

// EnsureSystemTableRelations adds the built-in relations between system tables when no relation is
// recorded for their source column yet, and fills in their display name and pull-through only while
// those are unset, so user edits are kept. Run after BackfillTableRelationsFromPhysicalFKs, which
// only imports physical foreign keys while table_relations is empty.
func (r *Repository) EnsureSystemTableRelations(ctx context.Context) error {
	for _, rel := range systemTableRelations {
		if err := r.ensureSystemTableRelation(ctx, rel); err != nil {
			return fmt.Errorf("system relation %s.%s: %w", rel.sourceTableKey, rel.sourceColumn, err)
		}
	}
	return nil
}

func (r *Repository) ensureSystemTableRelation(ctx context.Context, rel systemTableRelation) error {
	var count int
	if err := r.db.GetContext(ctx, &count, `
		SELECT COUNT(*) FROM table_relations WHERE source_table_key = ? AND source_column = ?`,
		rel.sourceTableKey, rel.sourceColumn,
	); err != nil {
		return err
	}
	if count == 0 {
		err := r.insertTableRelation(ctx, TableRelation{
			ConstraintName:     makeForeignKeyConstraintName(rel.sourceTableKey, rel.sourceColumn, rel.referencedTable, rel.referencedColumn),
			SourceTableKey:     rel.sourceTableKey,
			SourceColumn:       rel.sourceColumn,
			ReferencedTableKey: rel.referencedTable,
			ReferencedColumn:   rel.referencedColumn,
			OnDeleteAction:     rel.onDelete,
			OnUpdateAction:     "NO ACTION",
		})
		if err != nil {
			return err
		}
		log.Infof("Added system table relation %s.%s -> %s", rel.sourceTableKey, rel.sourceColumn, rel.referencedTable)
	}
	_, err := r.db.ExecContext(ctx, `
		UPDATE table_relations
		SET display_name = COALESCE(display_name, ?), pull_through_json = COALESCE(pull_through_json, ?)
		WHERE source_table_key = ? AND source_column = ?`,
		nilIfEmpty(rel.displayName), nilIfEmpty(rel.pullThroughJSON),
		rel.sourceTableKey, rel.sourceColumn,
	)
	return err
}
