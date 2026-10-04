package repo

import (
	"context"
	"fmt"
	"strings"

	log "github.com/sirupsen/logrus"
)

// TableRelation is the authoritative SickRock definition of a foreign-key-style link between tables.
// Keys are table configuration names (table_configurations.name), not physical table names.
type TableRelation struct {
	ID                   int    `db:"id"`
	ConstraintName       string `db:"constraint_name"`
	SourceTableKey       string `db:"source_table_key"`
	SourceColumn         string `db:"source_column"`
	ReferencedTableKey   string `db:"referenced_table_key"`
	ReferencedColumn     string `db:"referenced_column"`
	OnDeleteAction       string `db:"on_delete_action"`
	OnUpdateAction       string `db:"on_update_action"`
}

func makeForeignKeyConstraintName(sourceTableKey, sourceColumn, referencedTableKey, referencedColumn string) string {
	return fmt.Sprintf(
		"fk_%s_%s_%s_%s",
		sanitizeDatabaseIdentifier(sourceTableKey),
		sanitizeDatabaseIdentifier(sourceColumn),
		sanitizeDatabaseIdentifier(referencedTableKey),
		sanitizeDatabaseIdentifier(referencedColumn),
	)
}

func normalizeRelationAction(action string) string {
	action = strings.TrimSpace(strings.ToUpper(action))
	switch action {
	case "CASCADE", "SET NULL", "RESTRICT", "NO ACTION":
		return action
	default:
		return "NO ACTION"
	}
}

func (r *Repository) insertTableRelation(ctx context.Context, rel TableRelation) error {
	rel.OnDeleteAction = normalizeRelationAction(rel.OnDeleteAction)
	rel.OnUpdateAction = normalizeRelationAction(rel.OnUpdateAction)
	const q = `
		INSERT INTO table_relations (
			constraint_name, source_table_key, source_column,
			referenced_table_key, referenced_column, on_delete_action, on_update_action
		) VALUES (?, ?, ?, ?, ?, ?, ?)`
	_, err := r.db.ExecContext(ctx, q,
		rel.ConstraintName, rel.SourceTableKey, rel.SourceColumn,
		rel.ReferencedTableKey, rel.ReferencedColumn, rel.OnDeleteAction, rel.OnUpdateAction,
	)
	return err
}

func (r *Repository) deleteTableRelationByConstraintName(ctx context.Context, constraintName string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM table_relations WHERE constraint_name = ?`, constraintName)
	return err
}

func (r *Repository) getTableRelationByConstraintName(ctx context.Context, constraintName string) (*TableRelation, error) {
	var rel TableRelation
	err := r.db.GetContext(ctx, &rel, `
		SELECT id, constraint_name, source_table_key, source_column,
			referenced_table_key, referenced_column, on_delete_action, on_update_action
		FROM table_relations WHERE constraint_name = ?`, constraintName)
	if err != nil {
		return nil, err
	}
	return &rel, nil
}

func (r *Repository) listTableRelationsForConfiguration(ctx context.Context, tableConfigName string) ([]TableRelation, error) {
	var rows []TableRelation
	err := r.db.SelectContext(ctx, &rows, `
		SELECT id, constraint_name, source_table_key, source_column,
			referenced_table_key, referenced_column, on_delete_action, on_update_action
		FROM table_relations
		WHERE source_table_key = ? OR referenced_table_key = ?
		ORDER BY constraint_name`, tableConfigName, tableConfigName)
	return rows, err
}

func (r *Repository) listTableRelationsForSourceConfiguration(ctx context.Context, sourceTableKey string) ([]TableRelation, error) {
	var rows []TableRelation
	err := r.db.SelectContext(ctx, &rows, `
		SELECT id, constraint_name, source_table_key, source_column,
			referenced_table_key, referenced_column, on_delete_action, on_update_action
		FROM table_relations
		WHERE source_table_key = ?
		ORDER BY constraint_name`, sourceTableKey)
	return rows, err
}

func (r *Repository) tableRelationToForeignKey(ctx context.Context, rel TableRelation) (ForeignKey, error) {
	sourceTC, err := r.GetTableConfiguration(ctx, rel.SourceTableKey)
	if err != nil {
		return ForeignKey{}, err
	}
	refTC, err := r.GetTableConfiguration(ctx, rel.ReferencedTableKey)
	if err != nil {
		return ForeignKey{}, err
	}
	return ForeignKey{
		ConstraintName:   rel.ConstraintName,
		TableSchema:      sourceTC.Db.String,
		TableName:        sourceTC.Table.String,
		ColumnName:       rel.SourceColumn,
		ReferencedSchema: refTC.Db.String,
		ReferencedTable:  refTC.Table.String,
		ReferencedColumn: rel.ReferencedColumn,
		OnDeleteAction:   rel.OnDeleteAction,
		OnUpdateAction:   rel.OnUpdateAction,
	}, nil
}

func (r *Repository) tableRelationsToForeignKeys(ctx context.Context, relations []TableRelation) ([]ForeignKey, error) {
	out := make([]ForeignKey, 0, len(relations))
	for _, rel := range relations {
		fk, err := r.tableRelationToForeignKey(ctx, rel)
		if err != nil {
			return nil, err
		}
		out = append(out, fk)
	}
	return out, nil
}

// BackfillTableRelationsFromPhysicalFKs imports existing database FK constraints into table_relations once.
func (r *Repository) BackfillTableRelationsFromPhysicalFKs(ctx context.Context) error {
	var count int
	if err := r.db.GetContext(ctx, &count, `SELECT COUNT(*) FROM table_relations`); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	configs, err := r.ListTableConfigurationsWithDetails(ctx)
	if err != nil {
		return err
	}

	for _, cfg := range configs {
		tc := &cfg
		physicalFKs, err := r.introspectOutgoingPhysicalForeignKeys(ctx, tc)
		if err != nil {
			log.WithError(err).WithField("table", cfg.Name).Warn("backfill physical foreign keys")
			continue
		}
		for _, fk := range physicalFKs {
			refKey, err := r.lookupTableConfigKeyByPhysical(ctx, fk.ReferencedSchema, fk.ReferencedTable)
			if err != nil || refKey == "" {
				log.WithFields(log.Fields{
					"source":    cfg.Name,
					"ref_table": fk.ReferencedTable,
				}).Debug("skip backfill FK: referenced table has no configuration")
				continue
			}
			rel := TableRelation{
				ConstraintName:     fk.ConstraintName,
				SourceTableKey:     cfg.Name,
				SourceColumn:       fk.ColumnName,
				ReferencedTableKey: refKey,
				ReferencedColumn:   fk.ReferencedColumn,
				OnDeleteAction:     fk.OnDeleteAction,
				OnUpdateAction:     fk.OnUpdateAction,
			}
			if rel.ConstraintName == "" {
				rel.ConstraintName = makeForeignKeyConstraintName(
					rel.SourceTableKey, rel.SourceColumn, rel.ReferencedTableKey, rel.ReferencedColumn,
				)
			}
			if err := r.insertTableRelation(ctx, rel); err != nil {
				log.WithError(err).WithField("constraint", rel.ConstraintName).Debug("backfill insert relation")
			}
		}
	}
	return nil
}

func (r *Repository) lookupTableConfigKeyByPhysical(ctx context.Context, database, physicalTable string) (string, error) {
	dbName := strings.TrimSpace(database)
	if dbName == "" {
		dbName = "main"
	}
	names, err := r.LookupTableConfigNames(ctx, []DbTablePair{{Db: dbName, Table: physicalTable}})
	if err != nil {
		return "", err
	}
	return names[DbTableLookupKey(dbName, physicalTable)], nil
}

func (r *Repository) introspectOutgoingPhysicalForeignKeys(ctx context.Context, tc *TableConfig) ([]ForeignKey, error) {
	if tc == nil {
		return nil, fmt.Errorf("table configuration is required")
	}
	var foreignKeys []ForeignKey
	switch r.db.DriverName() {
	case "mysql":
		query := `
			SELECT
				kcu.CONSTRAINT_NAME as constraint_name,
				kcu.TABLE_SCHEMA as table_schema,
				kcu.TABLE_NAME as table_name,
				kcu.COLUMN_NAME as column_name,
				kcu.REFERENCED_TABLE_SCHEMA as referenced_schema,
				kcu.REFERENCED_TABLE_NAME as referenced_table,
				kcu.REFERENCED_COLUMN_NAME as referenced_column,
				COALESCE(rc.DELETE_RULE, 'NO ACTION') as on_delete_action,
				COALESCE(rc.UPDATE_RULE, 'NO ACTION') as on_update_action
			FROM INFORMATION_SCHEMA.KEY_COLUMN_USAGE kcu
			LEFT JOIN INFORMATION_SCHEMA.REFERENTIAL_CONSTRAINTS rc
				ON kcu.CONSTRAINT_NAME = rc.CONSTRAINT_NAME
				AND kcu.TABLE_SCHEMA = rc.CONSTRAINT_SCHEMA
			WHERE kcu.TABLE_SCHEMA = ? AND kcu.TABLE_NAME = ?
			AND kcu.REFERENCED_TABLE_NAME IS NOT NULL
			ORDER BY kcu.CONSTRAINT_NAME`
		rows, err := r.db.QueryxContext(ctx, query, tc.Db.String, tc.Table.String)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var fk ForeignKey
			if err := rows.StructScan(&fk); err != nil {
				return nil, err
			}
			foreignKeys = append(foreignKeys, fk)
		}
		return foreignKeys, rows.Err()
	default:
		list, err := r.sqliteForeignKeyList(ctx, tc.Table.String)
		if err != nil {
			return nil, err
		}
		for _, row := range list {
			foreignKeys = append(foreignKeys, ForeignKey{
				ConstraintName:   fmt.Sprintf("sqlite_fk_%s_%s", tc.Table.String, row.From),
				TableSchema:      tc.Db.String,
				TableName:        tc.Table.String,
				ColumnName:       row.From,
				ReferencedSchema: tc.Db.String,
				ReferencedTable:  row.Table,
				ReferencedColumn: row.To,
				OnDeleteAction:   row.OnDelete,
				OnUpdateAction:   row.OnUpdate,
			})
		}
		return foreignKeys, nil
	}
}
