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
	var tableDDL string
	if err := r.db.GetContext(ctx, &tableDDL,
		"SELECT sql FROM sqlite_master WHERE type = 'table' AND name = ?", physicalTable); err != nil {
		return fmt.Errorf("load table ddl: %w", err)
	}
	if tableDDL == "" {
		return fmt.Errorf("table %q not found", physicalTable)
	}

	var columns []sqliteTableInfoRow
	if err := r.db.SelectContext(ctx, &columns, fmt.Sprintf("PRAGMA table_info(%s)", physicalTable)); err != nil {
		return fmt.Errorf("table_info: %w", err)
	}
	if len(columns) == 0 {
		return fmt.Errorf("table %q has no columns", physicalTable)
	}

	type indexRow struct {
		Name string `db:"name"`
		SQL  string `db:"sql"`
	}
	var indexes []indexRow
	if err := r.db.SelectContext(ctx, &indexes,
		"SELECT name, sql FROM sqlite_master WHERE type = 'index' AND tbl_name = ? AND sql IS NOT NULL",
		physicalTable); err != nil {
		return fmt.Errorf("list indexes: %w", err)
	}

	tempTable := "_sr_fk_" + physicalTable
	colDefs := make([]string, 0, len(columns)+len(relations))
	for _, col := range columns {
		colDefs = append(colDefs, sqliteColumnDefinition(col, tableDDL))
	}
	for _, rel := range relations {
		refTC, err := r.GetTableConfiguration(ctx, rel.ReferencedTableKey)
		if err != nil {
			return err
		}
		colDefs = append(colDefs, sqliteForeignKeyClause(
			rel.SourceColumn,
			refTC.Table.String,
			rel.ReferencedColumn,
			normalizeSQLiteFKAction(rel.OnDeleteAction),
			normalizeSQLiteFKAction(rel.OnUpdateAction),
		))
	}

	createSQL := fmt.Sprintf("CREATE TABLE %s (%s)", quoteSQLiteIdent(tempTable), joinSQLiteDefs(colDefs))

	if _, err := r.db.ExecContext(ctx, "PRAGMA foreign_keys=OFF"); err != nil {
		return err
	}
	defer func() { _, _ = r.db.ExecContext(ctx, "PRAGMA foreign_keys=ON") }()

	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, fmt.Sprintf("DROP TABLE IF EXISTS %s", quoteSQLiteIdent(tempTable))); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, createSQL); err != nil {
		return fmt.Errorf("create rebuild table: %w", err)
	}

	colNames := make([]string, len(columns))
	for i, col := range columns {
		colNames[i] = quoteSQLiteIdent(col.Name)
	}
	copySQL := fmt.Sprintf(
		"INSERT INTO %s (%s) SELECT %s FROM %s",
		quoteSQLiteIdent(tempTable),
		joinSQLiteDefs(colNames),
		joinSQLiteDefs(colNames),
		quoteSQLiteIdent(physicalTable),
	)
	if _, err := tx.ExecContext(ctx, copySQL); err != nil {
		return fmt.Errorf("copy table data: %w", err)
	}
	if _, err := tx.ExecContext(ctx, fmt.Sprintf("DROP TABLE %s", quoteSQLiteIdent(physicalTable))); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, fmt.Sprintf("ALTER TABLE %s RENAME TO %s", quoteSQLiteIdent(tempTable), quoteSQLiteIdent(physicalTable))); err != nil {
		return err
	}
	for _, idx := range indexes {
		if idx.SQL == "" {
			continue
		}
		if _, err := tx.ExecContext(ctx, idx.SQL); err != nil {
			return fmt.Errorf("recreate index %s: %w", idx.Name, err)
		}
	}
	return tx.Commit()
}

func joinSQLiteDefs(parts []string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += ", "
		}
		out += p
	}
	return out
}
