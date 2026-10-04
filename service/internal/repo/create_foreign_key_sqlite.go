package repo

import (
	"context"
	"fmt"
	"regexp"
	"strings"

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
	Cid       int     `db:"cid"`
	Name      string  `db:"name"`
	Type      string  `db:"type"`
	NotNull   int     `db:"notnull"`
	DfltValue *string `db:"dflt_value"`
	Pk        int     `db:"pk"`
}

type sqliteForeignKeyRow struct {
	ID       int    `db:"id"`
	Seq      int    `db:"seq"`
	Table    string `db:"table"`
	From     string `db:"from"`
	To       string `db:"to"`
	OnUpdate string `db:"on_update"`
	OnDelete string `db:"on_delete"`
	Match    string `db:"match"`
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
	defer rows.Close()

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
	byID := make(map[int]sqliteForeignKeyRow)
	order := make([]int, 0, len(rows))
	for _, row := range rows {
		if _, seen := byID[row.ID]; !seen {
			order = append(order, row.ID)
		}
		byID[row.ID] = row
	}
	out := make([]sqliteForeignKeyRow, 0, len(order))
	for _, id := range order {
		out = append(out, byID[id])
	}
	return out
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
		if fk.From == fromCol && fk.Table == refTable && fk.To == refCol {
			return true
		}
	}
	return false
}

func (r *Repository) rebuildSQLiteTableAddForeignKey(
	ctx context.Context,
	physicalTable, columnName, referencedPhysicalTable, referencedColumn,
	onDelete, onUpdate string,
) error {
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

	existingFKs, err := r.sqliteForeignKeyList(ctx, physicalTable)
	if err != nil {
		return fmt.Errorf("foreign_key_list: %w", err)
	}
	if sqliteForeignKeyExists(existingFKs, columnName, referencedPhysicalTable, referencedColumn) {
		return nil
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
	colDefs := make([]string, 0, len(columns)+len(existingFKs)+1)
	for _, col := range columns {
		colDefs = append(colDefs, sqliteColumnDefinition(col, tableDDL))
	}
	for _, fk := range groupSQLiteForeignKeys(existingFKs) {
		colDefs = append(colDefs, sqliteForeignKeyClause(
			fk.From, fk.Table, fk.To, normalizeSQLiteFKAction(fk.OnDelete), normalizeSQLiteFKAction(fk.OnUpdate)))
	}
	colDefs = append(colDefs, sqliteForeignKeyClause(columnName, referencedPhysicalTable, referencedColumn, onDelete, onUpdate))

	createSQL := fmt.Sprintf("CREATE TABLE %s (%s)", quoteSQLiteIdent(tempTable), strings.Join(colDefs, ", "))
	log.Debugf("sqlite rebuild create: %s", createSQL)

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
		strings.Join(colNames, ", "),
		strings.Join(colNames, ", "),
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
