package repo

import (
	"context"
	"fmt"

	log "github.com/sirupsen/logrus"
)

func (r *Repository) applyPhysicalForeignKey(
	ctx context.Context,
	sourceTC, referencedTC *TableConfig,
	sourceColumn, referencedColumn, onDelete, onUpdate, constraintName string,
) error {
	switch r.db.DriverName() {
	case "mysql":
		alterQuery := fmt.Sprintf(
			"ALTER TABLE %s.%s ADD CONSTRAINT %s FOREIGN KEY (%s) REFERENCES %s.%s(%s) ON DELETE %s ON UPDATE %s",
			sourceTC.Db.String, sourceTC.Table.String, constraintName,
			sourceColumn, referencedTC.Db.String, referencedTC.Table.String, referencedColumn,
			normalizeRelationAction(onDelete), normalizeRelationAction(onUpdate),
		)
		log.Infof("Applying database foreign key (mysql): %s", alterQuery)
		_, err := r.db.ExecContext(ctx, alterQuery)
		return err
	default:
		return r.createForeignKeySQLite(ctx, sourceTC.Table.String, sourceColumn, referencedTC.Table.String, referencedColumn, onDelete, onUpdate)
	}
}

func (r *Repository) dropPhysicalForeignKey(ctx context.Context, rel *TableRelation) error {
	if rel == nil {
		return fmt.Errorf("relation is required")
	}
	sourceTC, err := r.GetTableConfiguration(ctx, rel.SourceTableKey)
	if err != nil {
		return err
	}
	switch r.db.DriverName() {
	case "mysql":
		alterQuery := fmt.Sprintf(
			"ALTER TABLE %s.%s DROP FOREIGN KEY %s",
			sourceTC.Db.String, sourceTC.Table.String, rel.ConstraintName,
		)
		log.Infof("Dropping database foreign key (mysql): %s", alterQuery)
		_, err := r.db.ExecContext(ctx, alterQuery)
		return err
	default:
		return r.syncSQLitePhysicalForeignKeysForSource(ctx, rel.SourceTableKey)
	}
}

func (r *Repository) syncSQLitePhysicalForeignKeysForSource(ctx context.Context, sourceTableKey string) error {
	sourceTC, err := r.GetTableConfiguration(ctx, sourceTableKey)
	if err != nil {
		return err
	}
	relations, err := r.listTableRelationsForSourceConfiguration(ctx, sourceTableKey)
	if err != nil {
		return err
	}
	return r.rebuildSQLiteTableForeignKeys(ctx, sourceTC.Table.String, relations)
}

func (r *Repository) rebuildSQLiteTableForeignKeys(ctx context.Context, physicalTable string, relations []TableRelation) error {
	snapshot, err := r.loadSQLiteTableSnapshot(ctx, physicalTable)
	if err != nil {
		return err
	}

	fkClauses := make([]string, 0, len(relations))
	for _, rel := range relations {
		refTC, err := r.GetTableConfiguration(ctx, rel.ReferencedTableKey)
		if err != nil {
			return err
		}
		fkClauses = append(fkClauses, sqliteForeignKeyClause(
			rel.SourceColumn,
			refTC.Table.String,
			rel.ReferencedColumn,
			normalizeSQLiteFKAction(rel.OnDeleteAction),
			normalizeSQLiteFKAction(rel.OnUpdateAction),
		))
	}

	return r.rebuildSQLiteTable(ctx, physicalTable, snapshot, fkClauses)
}
