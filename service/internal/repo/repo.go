package repo

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jamesread/golure/pkg/redact"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"
)

type Item struct {
	ID            string                 `db:"id"`
	CreatedAtUnix int64                  `db:"created_at_unix"`
	Fields        map[string]interface{} `db:"-"` // All dynamic fields including name
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

type TableConfig struct {
	Name             string
	Title            string
	Ordinal          int
	Icon             sql.NullString
	CreateButtonText sql.NullString
}

func (r *Repository) ListTableConfigurations(ctx context.Context) ([]string, error) {
	configs, err := r.ListTableConfigurationsWithDetails(ctx)
	if err != nil {
		return nil, err
	}

	// Extract just the names for backward compatibility
	names := make([]string, len(configs))
	for i, config := range configs {
		names[i] = config.Name
	}
	return names, nil
}

func (r *Repository) ListTableConfigurationsWithDetails(ctx context.Context) ([]TableConfig, error) {
	rows, err := r.db.QueryxContext(ctx, "SELECT name, COALESCE(title, name) as title, COALESCE(ordinal, 0) as ordinal, create_button_text, icon FROM table_configurations ORDER BY ordinal, name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var configs []TableConfig
	for rows.Next() {
		var config TableConfig
		if err := rows.Scan(&config.Name, &config.Title, &config.Ordinal, &config.CreateButtonText, &config.Icon); err != nil {
			return nil, err
		}
		configs = append(configs, config)
	}
	return configs, rows.Err()
}

func (r *Repository) EnsureSchema(ctx context.Context) error {
	log.Infof("Ensuring schema")
	var schema string
	switch r.db.DriverName() {
	case "mysql":
		schema = `
CREATE TABLE IF NOT EXISTS table_configurations (
    id INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(191) NOT NULL UNIQUE,
    title VARCHAR(191),
    ordinal INT DEFAULT 0
);

CREATE TABLE IF NOT EXISTS table_conditional_formatting_rules (
    id INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    table_name VARCHAR(191) NOT NULL,
    column_name VARCHAR(191) NOT NULL,
    condition_type VARCHAR(50) NOT NULL,
    condition_value TEXT,
    format_type VARCHAR(50) NOT NULL,
    format_value TEXT,
    priority INT DEFAULT 0,
    is_active BOOLEAN DEFAULT TRUE,
    created_at_unix BIGINT DEFAULT (UNIX_TIMESTAMP()),
    updated_at_unix BIGINT DEFAULT (UNIX_TIMESTAMP())
);
`
	default:
		schema = `
CREATE TABLE IF NOT EXISTS table_configurations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    title TEXT,
    ordinal INTEGER DEFAULT 0
);

CREATE TABLE IF NOT EXISTS table_conditional_formatting_rules (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    table_name TEXT NOT NULL,
    column_name TEXT NOT NULL,
    condition_type TEXT NOT NULL,
    condition_value TEXT,
    format_type TEXT NOT NULL,
    format_value TEXT,
    priority INTEGER DEFAULT 0,
    is_active INTEGER DEFAULT 1,
    created_at_unix INTEGER DEFAULT (strftime('%s', 'now')),
    updated_at_unix INTEGER DEFAULT (strftime('%s', 'now'))
);
`
	}
	_, err := r.db.ExecContext(ctx, schema)
	if err != nil {
		return err
	}

	// Add ordinal column if it doesn't exist (migration)
	return r.migrateTableConfigurations(ctx)
}

// migrateTableConfigurations adds the ordinal and title columns to existing table_configurations
func (r *Repository) migrateTableConfigurations(ctx context.Context) error {
	if r.db.DriverName() == "sqlite3" {
		// SQLite doesn't have information_schema, so we'll try to add the columns and ignore errors
		_, _ = r.db.ExecContext(ctx, "ALTER TABLE table_configurations ADD COLUMN ordinal INTEGER DEFAULT 0")
		_, _ = r.db.ExecContext(ctx, "ALTER TABLE table_configurations ADD COLUMN title TEXT")
		return nil // Ignore errors as columns might already exist
	}

	// For MySQL, check if columns exist before adding
	var ordinalCount, titleCount int
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM information_schema.columns WHERE table_name = 'table_configurations' AND column_name = 'ordinal'").Scan(&ordinalCount)
	if err != nil {
		return err
	}

	err = r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM information_schema.columns WHERE table_name = 'table_configurations' AND column_name = 'title'").Scan(&titleCount)
	if err != nil {
		return err
	}

	if ordinalCount == 0 {
		// Ordinal column doesn't exist, add it
		_, err = r.db.ExecContext(ctx, "ALTER TABLE table_configurations ADD COLUMN ordinal INT DEFAULT 0")
		if err != nil {
			return err
		}
	}

	if titleCount == 0 {
		// Title column doesn't exist, add it
		_, err = r.db.ExecContext(ctx, "ALTER TABLE table_configurations ADD COLUMN title VARCHAR(191)")
		if err != nil {
			return err
		}
	}

	return nil
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
    created_at_unix BIGINT NOT NULL
);`, t)
	default:
		schema = fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %s (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
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
	defaultClause := ""

	switch field.Type {
	case "int64":
		typ = "BIGINT"
	case "string":
		typ = "TEXT"
	case "datetime":
		// Use BIGINT to store Unix timestamps for datetime columns
		typ = "BIGINT"
		if field.DefaultToCurrentTimestamp {
			// Add default value for current timestamp
			if r.db.DriverName() == "mysql" {
				defaultClause = " DEFAULT (UNIX_TIMESTAMP())"
			} else {
				// SQLite doesn't support DEFAULT (UNIX_TIMESTAMP()), so we'll handle this in application logic
				defaultClause = ""
			}
		}
	default:
		typ = "TEXT"
	}

	notNull := ""
	if field.Required {
		notNull = " NOT NULL"
	}

	query := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s%s%s", t, col, typ, notNull, defaultClause)
	_, err := r.db.ExecContext(ctx, query)
	return err
}

type FieldSpec struct {
	Name                      string
	Type                      string
	Required                  bool
	DefaultToCurrentTimestamp bool
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
	columnNames = append(columnNames, "created_at_unix")
	for _, col := range columns {
		columnNames = append(columnNames, col.Name)
	}

	query := fmt.Sprintf("SELECT %s FROM %s ORDER BY created_at_unix DESC", strings.Join(columnNames, ", "), t)
	log.Infof("Executing query: %s", query)

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
			log.Infof("id field found: %v (type: %T)", id, id)
			if idStr, ok := id.(string); ok {
				item.ID = idStr
			} else if idInt, ok := id.(int64); ok {
				item.ID = strconv.FormatInt(idInt, 10)
			}
		} else {
			log.Warnf("id field not found in rowMap")
		}
		// name field is now handled as a dynamic field
		if createdAt, ok := rowMap["created_at_unix"]; ok {
			log.Infof("created_at_unix field found: %v (type: %T)", createdAt, createdAt)
			if createdAtInt, ok := createdAt.(int64); ok {
				item.CreatedAtUnix = createdAtInt
			} else {
				log.Warnf("created_at_unix field is not int64, got type: %T, value: %v", createdAt, createdAt)
			}
		} else {
			log.Warnf("created_at_unix field not found in rowMap")
		}

		// Add all other fields to the dynamic Fields map (including name now)
		for colName, value := range rowMap {
			if colName != "id" && colName != "created_at_unix" {
				// Handle MySQL byte slice conversion for all fields
				if valueBytes, ok := value.([]uint8); ok {
					item.Fields[colName] = string(valueBytes)
					log.Infof("Converted field %s from []uint8 to string: %s", colName, string(valueBytes))
				} else {
					item.Fields[colName] = value
				}
			}
		}

		log.Infof("sql item: %+v", rowMap)

		items = append(items, item)
	}

	return items, rows.Err()
}

func (r *Repository) CreateItemInTable(ctx context.Context, table string, additionalFields map[string]string) (Item, error) {
	now := time.Now().Unix()
	return r.CreateItemInTableWithTimestamp(ctx, table, additionalFields, now)
}

func (r *Repository) CreateItemInTableWithTimestamp(ctx context.Context, table string, additionalFields map[string]string, timestamp int64) (Item, error) {
	t := sanitizeTableName(table)

	// Build dynamic INSERT query
	columns := []string{"created_at_unix"}
	placeholders := []string{"?"}
	values := []interface{}{timestamp}

	for key, value := range additionalFields {
		columns = append(columns, key)
		placeholders = append(placeholders, "?")
		values = append(values, value)
	}

	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", t, strings.Join(columns, ", "), strings.Join(placeholders, ", "))
	log.Infof("Creating item in table %s with fields: %+v, timestamp: %d, query: %s", t, additionalFields, timestamp, query)

	res, err := r.db.ExecContext(ctx, query, values...)
	if err != nil {
		log.Errorf("Failed to create item: %v", err)
		return Item{}, err
	}
	lastID, _ := res.LastInsertId()

	// Convert additionalFields to interface{} map for the Item
	fields := make(map[string]interface{})
	for key, value := range additionalFields {
		fields[key] = value
	}

	item := Item{ID: strconv.FormatInt(lastID, 10), CreatedAtUnix: timestamp, Fields: fields}
	log.Infof("Created item: %+v", item)
	return item, nil
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
	// name field is now handled as a dynamic field
	if createdAt, ok := rowMap["created_at_unix"]; ok {
		if createdAtInt, ok := createdAt.(int64); ok {
			item.CreatedAtUnix = createdAtInt
		}
	}

	// Add all other fields to the dynamic Fields map (including name now)
	for colName, value := range rowMap {
		if colName != "id" && colName != "created_at_unix" {
			// Handle MySQL byte slice conversion for all fields
			if valueBytes, ok := value.([]uint8); ok {
				item.Fields[colName] = string(valueBytes)
			} else {
				item.Fields[colName] = value
			}
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

func (r *Repository) EditItemInTableWithFields(ctx context.Context, table string, id string, name string, additionalFields map[string]string) (Item, error) {
	t := sanitizeTableName(table)

	// Build dynamic UPDATE query
	setParts := []string{"name = ?"}
	args := []interface{}{name}

	for fieldName, fieldValue := range additionalFields {
		// Sanitize field name to prevent SQL injection
		sanitizedFieldName := sanitizeTableName(fieldName)
		setParts = append(setParts, fmt.Sprintf("%s = ?", sanitizedFieldName))
		args = append(args, fieldValue)
	}

	args = append(args, id) // Add id for WHERE clause

	query := fmt.Sprintf("UPDATE %s SET %s WHERE id = ?", t, strings.Join(setParts, ", "))
	log.Infof("Executing update query: %s with args: %v", query, args)

	if _, err := r.db.ExecContext(ctx, query, args...); err != nil {
		log.Errorf("Failed to update item: %v", err)
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
func (r *Repository) CreateItem(ctx context.Context, additionalFields map[string]string) (Item, error) {
	return r.CreateItemInTable(ctx, "items", additionalFields)
}
func (r *Repository) GetItem(ctx context.Context, id string) (Item, error) {
	return r.GetItemInTable(ctx, "items", id)
}
func (r *Repository) EditItem(ctx context.Context, id string, additionalFields map[string]string) (Item, error) {
	return r.EditItemInTableWithFields(ctx, "items", id, "", additionalFields)
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

		log.Infof("DB_HOST: %s", host)
		log.Infof("DB_PORT: %s", port)
		log.Infof("DB_USER: %s", user)
		log.Infof("DB_PASS: %s", redact.RedactString(pass))
		log.Infof("DB_NAME: %s", name)

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
			if strings.Contains(dt, "tinyint") {
				typ = "tinyint"
			} else if strings.Contains(dt, "int") {
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

// TableStructure represents the structure of a table
type TableStructure struct {
	CreateButtonText string
}

// GetTableStructure returns the structure information for a table
func (r *Repository) GetTableStructure(ctx context.Context, table string) (*TableStructure, error) {
	// For now, return a default structure
	// In the future, this could be enhanced to read from table_configurations
	// or other metadata sources
	return &TableStructure{
		CreateButtonText: "Add Item",
	}, nil
}
