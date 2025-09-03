package repo

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"
)

type Item struct {
	ID            string                 `db:"id"`
	Name          string                 `db:"name"`
	CreatedAtUnix int64                  `db:"created_at_unix"`
	Fields        map[string]interface{} `db:"-"` // Additional dynamic fields
}

type Repository struct {
	db *sqlx.DB
}

func NewRepository(db *sqlx.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) InsertTableConfiguration(ctx context.Context, name string) error {
	n := sanitizeTableName(name)
	switch r.db.DriverName() {
	case "mysql":
		_, err := r.db.ExecContext(ctx, "INSERT IGNORE INTO table_configurations (name) VALUES (?)", n)
		return err
	default:
		_, err := r.db.ExecContext(ctx, "INSERT OR IGNORE INTO table_configurations (name) VALUES (?)", n)
		return err
	}
}

func (r *Repository) ListTableConfigurations(ctx context.Context) ([]string, error) {
	rows, err := r.db.QueryxContext(ctx, "SELECT name FROM table_configurations ORDER BY name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var names []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, err
		}
		names = append(names, n)
	}
	return names, rows.Err()
}

func (r *Repository) EnsureSchema(ctx context.Context) error {
	log.Infof("Ensuring schema")
	var schema string
	switch r.db.DriverName() {
	case "mysql":
		schema = `
CREATE TABLE IF NOT EXISTS table_configurations (
    id INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(191) NOT NULL UNIQUE
);
`
	default:
		schema = `
CREATE TABLE IF NOT EXISTS table_configurations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE
);
`
	}
	_, err := r.db.ExecContext(ctx, schema)
	return err
}

// sanitizeTableName ensures the table name is a safe SQL identifier: [a-zA-Z0-9_]+
func sanitizeTableName(input string) string {
	if input == "" {
		return "items"
	}
	re := regexp.MustCompile(`[^a-zA-Z0-9_]`)
	sanitized := re.ReplaceAllString(input, "")
	if sanitized == "" {
		return "items"
	}
	return sanitized
}

// EnsureSchemaForTable creates the table if it doesn't exist.
func (r *Repository) EnsureSchemaForTable(ctx context.Context, table string) error {
	t := sanitizeTableName(table)
	var schema string
	switch r.db.DriverName() {
	case "mysql":
		schema = fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %s (
    id INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    name TEXT NOT NULL,
    created_at_unix BIGINT NOT NULL
);`, t)
	default:
		schema = fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %s (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    created_at_unix BIGINT NOT NULL
);`, t)
	}
	_, err := r.db.ExecContext(ctx, schema)
	return err
}

func (r *Repository) AddColumn(ctx context.Context, table string, field FieldSpec) error {
	t := sanitizeTableName(table)
	col := sanitizeTableName(field.Name)
	typ := "TEXT"
	switch field.Type {
	case "int64":
		typ = "BIGINT"
	case "string":
		typ = "TEXT"
	default:
		typ = "TEXT"
	}
	notNull := ""
	if field.Required {
		notNull = " NOT NULL"
	}
	query := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s%s", t, col, typ, notNull)
	_, err := r.db.ExecContext(ctx, query)
	return err
}

type FieldSpec struct {
	Name     string
	Type     string
	Required bool
}

func (r *Repository) ListItemsInTable(ctx context.Context, table string) ([]Item, error) {
	t := sanitizeTableName(table)

	// First get the column names for this table
	columns, err := r.ListColumns(ctx, table)
	if err != nil {
		return nil, fmt.Errorf("failed to get columns for table %s: %w", table, err)
	}

	// Build dynamic SELECT query with all columns
	columnNames := make([]string, 0, len(columns))
	for _, col := range columns {
		columnNames = append(columnNames, col.Name)
	}

	query := fmt.Sprintf("SELECT %s FROM %s ORDER BY created_at_unix DESC", strings.Join(columnNames, ", "), t)

	// Use QueryxContext to get raw rows and manually map them
	rows, err := r.db.QueryxContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []Item
	for rows.Next() {
		// Get the row as a map
		rowMap := make(map[string]interface{})
		if err := rows.MapScan(rowMap); err != nil {
			return nil, err
		}

		// Create Item with dynamic fields
		item := Item{
			Fields: make(map[string]interface{}),
		}

		// Map known fields
		if id, ok := rowMap["id"]; ok {
			if idStr, ok := id.(string); ok {
				item.ID = idStr
			} else if idInt, ok := id.(int64); ok {
				item.ID = strconv.FormatInt(idInt, 10)
			}
		}
		if name, ok := rowMap["name"]; ok {
			log.Infof("name: %v", name)
			if nameStr, ok := name.(string); ok {
				item.Name = nameStr
			}
		}
		if createdAt, ok := rowMap["created_at_unix"]; ok {
			if createdAtInt, ok := createdAt.(int64); ok {
				item.CreatedAtUnix = createdAtInt
			}
		}

		// Add all other fields to the dynamic Fields map
		for colName, value := range rowMap {
			if colName != "id" && colName != "name" && colName != "created_at_unix" {
				item.Fields[colName] = value
			}
		}

		log.Infof("sql item: %+v", rowMap)

		items = append(items, item)
	}

	return items, rows.Err()
}

func (r *Repository) CreateItemInTable(ctx context.Context, table string, name string) (Item, error) {
	t := sanitizeTableName(table)
	now := time.Now().Unix()
	query := fmt.Sprintf("INSERT INTO %s (name, created_at_unix) VALUES (?, ?)", t)
	res, err := r.db.ExecContext(ctx, query, name, now)
	if err != nil {
		return Item{}, err
	}
	lastID, _ := res.LastInsertId()
	return Item{ID: strconv.FormatInt(lastID, 10), Name: name, CreatedAtUnix: now}, nil
}

func (r *Repository) GetItemInTable(ctx context.Context, table string, id string) (Item, error) {
	t := sanitizeTableName(table)

	// First get the column names for this table
	columns, err := r.ListColumns(ctx, table)
	if err != nil {
		return Item{}, fmt.Errorf("failed to get columns for table %s: %w", table, err)
	}

	// Build dynamic SELECT query with all columns
	columnNames := make([]string, 0, len(columns))
	for _, col := range columns {
		columnNames = append(columnNames, col.Name)
	}

	query := fmt.Sprintf("SELECT %s FROM %s WHERE id = ?", strings.Join(columnNames, ", "), t)

	// Use QueryxContext to get raw row and manually map it
	rows, err := r.db.QueryxContext(ctx, query, id)
	if err != nil {
		return Item{}, err
	}
	defer rows.Close()

	if !rows.Next() {
		return Item{}, fmt.Errorf("item not found")
	}

	// Get the row as a map
	rowMap := make(map[string]interface{})
	if err := rows.MapScan(rowMap); err != nil {
		return Item{}, err
	}

	// Create Item with dynamic fields
	item := Item{
		Fields: make(map[string]interface{}),
	}

	// Map known fields
	if idVal, ok := rowMap["id"]; ok {
		if idStr, ok := idVal.(string); ok {
			item.ID = idStr
		} else if idInt, ok := idVal.(int64); ok {
			item.ID = strconv.FormatInt(idInt, 10)
		}
	}
	if name, ok := rowMap["name"]; ok {
		if nameStr, ok := name.(string); ok {
			item.Name = nameStr
		}
	}
	if createdAt, ok := rowMap["created_at_unix"]; ok {
		if createdAtInt, ok := createdAt.(int64); ok {
			item.CreatedAtUnix = createdAtInt
		}
	}

	// Add all other fields to the dynamic Fields map
	for colName, value := range rowMap {
		if colName != "id" && colName != "name" && colName != "created_at_unix" {
			item.Fields[colName] = value
		}
	}

	return item, nil
}

func (r *Repository) EditItemInTable(ctx context.Context, table string, id string, name string) (Item, error) {
	t := sanitizeTableName(table)
	query := fmt.Sprintf("UPDATE %s SET name = ? WHERE id = ?", t)
	if _, err := r.db.ExecContext(ctx, query, name, id); err != nil {
		return Item{}, err
	}
	return r.GetItemInTable(ctx, t, id)
}

func (r *Repository) DeleteItemInTable(ctx context.Context, table string, id string) (bool, error) {
	t := sanitizeTableName(table)
	query := fmt.Sprintf("DELETE FROM %s WHERE id = ?", t)
	res, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// Backwards-compatible wrappers defaulting to "items" table
func (r *Repository) ListItems(ctx context.Context, _ string) ([]Item, error) {
	return r.ListItemsInTable(ctx, "items")
}
func (r *Repository) CreateItem(ctx context.Context, name string) (Item, error) {
	return r.CreateItemInTable(ctx, "items", name)
}
func (r *Repository) GetItem(ctx context.Context, id string) (Item, error) {
	return r.GetItemInTable(ctx, "items", id)
}
func (r *Repository) EditItem(ctx context.Context, id string, name string) (Item, error) {
	return r.EditItemInTable(ctx, "items", id, name)
}
func (r *Repository) DeleteItem(ctx context.Context, id string) (bool, error) {
	return r.DeleteItemInTable(ctx, "items", id)
}

// OpenFromEnv returns a database connection using MySQL if DB_HOST is set,
// otherwise falls back to sqlite using the provided defaultSQLiteDSN.
func OpenFromEnv(defaultSQLiteDSN string) (*sqlx.DB, error) {
	host := os.Getenv("DB_HOST")
	if host != "" {
		port := os.Getenv("DB_PORT")
		if port == "" {
			port = "3306"
		}
		user := os.Getenv("DB_USER")
		pass := os.Getenv("DB_PASS")
		name := os.Getenv("DB_NAME")
		dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true&multiStatements=true", user, pass, host, port, name)
		return sqlx.Open("mysql", dsn)
	}
	return sqlx.Open("sqlite", defaultSQLiteDSN)
}

func (r *Repository) ListColumns(ctx context.Context, table string) ([]FieldSpec, error) {
	t := sanitizeTableName(table)
	driver := r.db.DriverName()
	specs := make([]FieldSpec, 0, 8)
	switch driver {
	case "mysql":
		var dbName string
		if err := r.db.GetContext(ctx, &dbName, "SELECT DATABASE()"); err != nil {
			return nil, err
		}
		type row struct {
			ColumnName string `db:"COLUMN_NAME"`
			DataType   string `db:"DATA_TYPE"`
			IsNullable string `db:"IS_NULLABLE"`
		}
		rows := []row{}
		q := `SELECT COLUMN_NAME, DATA_TYPE, IS_NULLABLE FROM INFORMATION_SCHEMA.COLUMNS WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ? ORDER BY ORDINAL_POSITION`
		if err := r.db.SelectContext(ctx, &rows, q, dbName, t); err != nil {
			return nil, err
		}
		for _, r := range rows {
			typ := "string"
			dt := strings.ToLower(r.DataType)
			if strings.Contains(dt, "int") {
				typ = "int64"
			}
			specs = append(specs, FieldSpec{Name: r.ColumnName, Type: typ, Required: strings.ToUpper(r.IsNullable) == "NO"})
		}
	default: // sqlite
		type srow struct {
			Cid     int    `db:"cid"`
			Name    string `db:"name"`
			Type    string `db:"type"`
			NotNull int    `db:"notnull"`
		}
		var rows []srow
		q := fmt.Sprintf("PRAGMA table_info(%s)", t)
		if err := r.db.SelectContext(ctx, &rows, q); err != nil {
			return nil, err
		}
		for _, r := range rows {
			typ := "string"
			tt := strings.ToLower(r.Type)
			if strings.Contains(tt, "int") {
				typ = "int64"
			}
			specs = append(specs, FieldSpec{Name: r.Name, Type: typ, Required: r.NotNull == 1})
		}
	}
	return specs, nil
}
