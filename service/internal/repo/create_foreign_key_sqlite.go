package repo

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"
)

var sqliteSimpleDefaultLiteral = regexp.MustCompile(`^(-?\d+(\.\d+)?|NULL|'(?:''|[^'])*')$`)

func quoteSQLiteIdent(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

func normalizeSQLiteReferentialAction(action string) string {
	action = strings.TrimSpace(strings.ToUpper(action))
	switch action {
	case "CASCADE", "SET NULL", "SET DEFAULT", "RESTRICT", "NO ACTION":
		return action
	default:
		return "NO ACTION"
	}
}

func (r *Repository) createForeignKeySQLite(
	ctx context.Context,
	physicalTable, columnName, referencedPhysicalTable, referencedColumn,
	onDeleteAction, onUpdateAction string,
) error {
	onDelete := normalizeSQLiteReferentialAction(onDeleteAction)
	onUpdate := normalizeSQLiteReferentialAction(onUpdateAction)

	alterQuery := fmt.Sprintf(
		"ALTER TABLE %s ADD FOREIGN KEY (%s) REFERENCES %s(%s) ON DELETE %s ON UPDATE %s",
		quoteSQLiteIdent(physicalTable),
		quoteSQLiteIdent(columnName),
		quoteSQLiteIdent(referencedPhysicalTable),
		quoteSQLiteIdent(referencedColumn),
		onDelete,
		onUpdate,
	)
	log.Infof("Creating foreign key (sqlite): %s", alterQuery)

	_, err := r.db.ExecContext(ctx, alterQuery)
	if err == nil {
		return nil
	}
	log.WithError(err).Debug("sqlite ALTER ADD FOREIGN KEY failed, rebuilding table")
	return r.rebuildSQLiteTableAddForeignKey(ctx, physicalTable, columnName, referencedPhysicalTable, referencedColumn, onDelete, onUpdate)
}

type sqliteTableInfoRow struct {
	DfltValue *string `db:"dflt_value"`
	Name      string  `db:"name"`
	Type      string  `db:"type"`
	Cid       int     `db:"cid"`
	NotNull   int     `db:"notnull"`
	Pk        int     `db:"pk"`
}

type sqliteForeignKeyRow struct {
	Table    string `db:"table"`
	From     string `db:"from"`
	To       string `db:"to"`
	OnUpdate string `db:"on_update"`
	OnDelete string `db:"on_delete"`
	Match    string `db:"match"`
	ID       int    `db:"id"`
	Seq      int    `db:"seq"`
}

func sqliteColumnDefinition(col sqliteTableInfoRow, tableDDL string) string {
	parts := []string{quoteSQLiteIdent(col.Name), col.Type}
	if col.Pk > 0 {
		if strings.Contains(strings.ToUpper(tableDDL), "AUTOINCREMENT") &&
			strings.Contains(strings.ToUpper(tableDDL), strings.ToUpper(col.Name)) {
			parts = append(parts, "PRIMARY KEY AUTOINCREMENT")
		} else {
			parts = append(parts, "PRIMARY KEY")
		}
	} else if col.NotNull == 1 {
		parts = append(parts, "NOT NULL")
	}
	if col.DfltValue != nil && *col.DfltValue != "" {
		parts = append(parts, sqliteDefaultClause(*col.DfltValue))
	}
	return strings.Join(parts, " ")
}

// sqliteDefaultClause formats a PRAGMA table_info dflt_value for CREATE TABLE.
func sqliteDefaultClause(dflt string) string {
	dflt = strings.TrimSpace(dflt)
	if dflt == "" {
		return ""
	}
	if strings.HasPrefix(dflt, "(") {
		return "DEFAULT " + dflt
	}
	if sqliteSimpleDefaultLiteral.MatchString(dflt) {
		return "DEFAULT " + dflt
	}
	return "DEFAULT (" + dflt + ")"
}

func sqliteForeignKeyClause(fromCol, refTable, refCol, onDelete, onUpdate string) string {
	return fmt.Sprintf(
		"FOREIGN KEY (%s) REFERENCES %s(%s) ON DELETE %s ON UPDATE %s",
		quoteSQLiteIdent(fromCol),
		quoteSQLiteIdent(refTable),
		quoteSQLiteIdent(refCol),
		onDelete,
		onUpdate,
	)
}

func (r *Repository) sqliteForeignKeyList(ctx context.Context, physicalTable string) ([]sqliteForeignKeyRow, error) {
	rows, err := r.db.QueryxContext(ctx, fmt.Sprintf("PRAGMA foreign_key_list(%s)", physicalTable))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []sqliteForeignKeyRow
	for rows.Next() {
		var fk sqliteForeignKeyRow
		if err := rows.Scan(&fk.ID, &fk.Seq, &fk.Table, &fk.From, &fk.To, &fk.OnUpdate, &fk.OnDelete, &fk.Match); err != nil {
			return nil, err
		}
		out = append(out, fk)
	}
	return out, rows.Err()
}

// groupSQLiteForeignKeys collapses composite-key pragma rows to one entry per constraint id.
func groupSQLiteForeignKeys(rows []sqliteForeignKeyRow) []sqliteForeignKeyRow {
	if len(rows) == 0 {
		return rows
	}
	byID, order := indexSQLiteForeignKeys(rows)
	out := make([]sqliteForeignKeyRow, 0, len(order))
	for _, id := range order {
		out = append(out, byID[id])
	}
	return out
}

func indexSQLiteForeignKeys(rows []sqliteForeignKeyRow) (map[int]sqliteForeignKeyRow, []int) {
	byID := make(map[int]sqliteForeignKeyRow)
	order := make([]int, 0, len(rows))
	for _, row := range rows {
		if _, seen := byID[row.ID]; !seen {
			order = append(order, row.ID)
		}
		byID[row.ID] = row
	}
	return byID, order
}

func normalizeSQLiteFKAction(action string) string {
	action = strings.TrimSpace(strings.ToUpper(action))
	if action == "" {
		return "NO ACTION"
	}
	return normalizeSQLiteReferentialAction(action)
}

func sqliteForeignKeyExists(fks []sqliteForeignKeyRow, fromCol, refTable, refCol string) bool {
	for _, fk := range fks {
		if sqliteForeignKeyMatches(fk, fromCol, refTable, refCol) {
			return true
		}
	}
	return false
}

func sqliteForeignKeyMatches(fk sqliteForeignKeyRow, fromCol, refTable, refCol string) bool {
	return fk.From == fromCol && fk.Table == refTable && fk.To == refCol
}

func (r *Repository) rebuildSQLiteTableAddForeignKey(
	ctx context.Context,
	physicalTable, columnName, referencedPhysicalTable, referencedColumn,
	onDelete, onUpdate string,
) error {
	snapshot, err := r.loadSQLiteTableSnapshot(ctx, physicalTable)
	if err != nil {
		return err
	}

	existingFKs, err := r.sqliteForeignKeyList(ctx, physicalTable)
	if err != nil {
		return fmt.Errorf("foreign_key_list: %w", err)
	}
	if sqliteForeignKeyExists(existingFKs, columnName, referencedPhysicalTable, referencedColumn) {
		return nil
	}

	fkClauses := make([]string, 0, len(existingFKs)+1)
	for _, fk := range groupSQLiteForeignKeys(existingFKs) {
		fkClauses = append(fkClauses, sqliteForeignKeyClause(
			fk.From, fk.Table, fk.To, normalizeSQLiteFKAction(fk.OnDelete), normalizeSQLiteFKAction(fk.OnUpdate)))
	}
	fkClauses = append(fkClauses, sqliteForeignKeyClause(columnName, referencedPhysicalTable, referencedColumn, onDelete, onUpdate))

	return r.rebuildSQLiteTable(ctx, physicalTable, snapshot, fkClauses)
}

type sqliteIndexRow struct {
	Name string `db:"name"`
	SQL  string `db:"sql"`
}

// sqliteTableSnapshot is what is needed to recreate a SQLite table: its DDL, columns and indexes.
type sqliteTableSnapshot struct {
	ddl     string
	columns []sqliteTableInfoRow
	indexes []sqliteIndexRow
}

func (r *Repository) loadSQLiteTableSnapshot(ctx context.Context, physicalTable string) (*sqliteTableSnapshot, error) {
	snapshot := &sqliteTableSnapshot{}
	if err := r.db.GetContext(ctx, &snapshot.ddl,
		"SELECT sql FROM sqlite_master WHERE type = 'table' AND name = ?", physicalTable); err != nil {
		return nil, fmt.Errorf("load table ddl: %w", err)
	}
	if snapshot.ddl == "" {
		return nil, fmt.Errorf("table %q not found", physicalTable)
	}

	if err := r.db.SelectContext(ctx, &snapshot.columns, fmt.Sprintf("PRAGMA table_info(%s)", physicalTable)); err != nil {
		return nil, fmt.Errorf("table_info: %w", err)
	}
	if len(snapshot.columns) == 0 {
		return nil, fmt.Errorf("table %q has no columns", physicalTable)
	}

	if err := r.db.SelectContext(ctx, &snapshot.indexes,
		"SELECT name, sql FROM sqlite_master WHERE type = 'index' AND tbl_name = ? AND sql IS NOT NULL",
		physicalTable); err != nil {
		return nil, fmt.Errorf("list indexes: %w", err)
	}
	return snapshot, nil
}

// rebuildSQLiteTable recreates physicalTable with its existing columns and exactly the given foreign
// key clauses (SQLite cannot alter constraints in place), copying the data and recreating its indexes.
func (r *Repository) rebuildSQLiteTable(ctx context.Context, physicalTable string, snapshot *sqliteTableSnapshot, fkClauses []string) error {
	steps := sqliteRebuildSteps(physicalTable, snapshot, fkClauses)

	if _, err := r.db.ExecContext(ctx, "PRAGMA foreign_keys=OFF"); err != nil {
		return err
	}
	defer func() { _, _ = r.db.ExecContext(ctx, "PRAGMA foreign_keys=ON") }()

	return r.inTx(ctx, func(tx *sqlx.Tx) error {
		for _, step := range steps {
			if _, err := tx.ExecContext(ctx, step.sql); err != nil {
				return wrapErrIf(step.errPrefix, err)
			}
		}
		return nil
	})
}

// sqlStep is one statement of a multi-statement change; errPrefix (optional) labels its failure.
type sqlStep struct{ sql, errPrefix string }

// sqliteRebuildSteps returns the statements that recreate physicalTable through a temporary table
// with its columns plus fkClauses, copying its data and recreating its indexes.
func sqliteRebuildSteps(physicalTable string, snapshot *sqliteTableSnapshot, fkClauses []string) []sqlStep {
	tempTable := "_sr_fk_" + physicalTable
	colDefs := make([]string, 0, len(snapshot.columns)+len(fkClauses))
	colNames := make([]string, len(snapshot.columns))
	for i, col := range snapshot.columns {
		colDefs = append(colDefs, sqliteColumnDefinition(col, snapshot.ddl))
		colNames[i] = quoteSQLiteIdent(col.Name)
	}
	colDefs = append(colDefs, fkClauses...)

	createSQL := fmt.Sprintf("CREATE TABLE %s (%s)", quoteSQLiteIdent(tempTable), strings.Join(colDefs, ", "))
	log.Debugf("sqlite rebuild create: %s", createSQL)
	copySQL := fmt.Sprintf(
		"INSERT INTO %s (%s) SELECT %s FROM %s",
		quoteSQLiteIdent(tempTable),
		strings.Join(colNames, ", "),
		strings.Join(colNames, ", "),
		quoteSQLiteIdent(physicalTable),
	)

	steps := []sqlStep{
		{fmt.Sprintf("DROP TABLE IF EXISTS %s", quoteSQLiteIdent(tempTable)), ""},
		{createSQL, "create rebuild table"},
		{copySQL, "copy table data"},
		{fmt.Sprintf("DROP TABLE %s", quoteSQLiteIdent(physicalTable)), ""},
		{fmt.Sprintf("ALTER TABLE %s RENAME TO %s", quoteSQLiteIdent(tempTable), quoteSQLiteIdent(physicalTable)), ""},
	}
	for _, idx := range snapshot.indexes {
		if idx.SQL != "" {
			steps = append(steps, sqlStep{idx.SQL, "recreate index " + idx.Name})
		}
	}
	return steps
}

// wrapErrIf prefixes err with prefix, or returns it unchanged when prefix is empty.
func wrapErrIf(prefix string, err error) error {
	if prefix == "" {
		return err
	}
	return fmt.Errorf("%s: %w", prefix, err)
}
