package repo

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jamesread/golure/pkg/redact"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"
	_ "modernc.org/sqlite"
)

type Item struct {
	ID        string                 `db:"id"`
	SrCreated time.Time              `db:"sr_created"`
	Fields    map[string]interface{} `db:"-"` // All dynamic fields including name
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
		_, err := r.db.ExecContext(ctx, "INSERT IGNORE INTO table_configurations (name, title, ordinal, db) VALUES (?, ?, 0, DATABASE())", n, n)
		return err
	default:
		_, err := r.db.ExecContext(ctx, "INSERT OR IGNORE INTO table_configurations (name, title, ordinal, db) VALUES (?, ?, 0, '')", n, n)
		return err
	}
}

type TableConfig struct {
	Name             string
	Title            string
	Ordinal          int
	Icon             sql.NullString
	CreateButtonText sql.NullString `db:"create_button_text"`
	View             sql.NullString `db:"view"`
	Table            sql.NullString `db:"table"`
	Db               sql.NullString `db:"db"`
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
	rows, err := r.db.QueryxContext(ctx, "SELECT name, COALESCE(title, name) as title, COALESCE(ordinal,0) as ordinal, icon, view, db FROM table_configurations ORDER BY name, ordinal ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var configs []TableConfig
	for rows.Next() {
		var config TableConfig
		if err := rows.Scan(&config.Name, &config.Title, &config.Ordinal, &config.CreateButtonText, &config.Icon, &config.View); err != nil {
			return nil, err
		}
		configs = append(configs, config)
	}
	return configs, rows.Err()
}

func (r *Repository) EnsureSchema(ctx context.Context) error {
	log.Infof("Using database driver: %s", r.db.DriverName())
	log.Infof("Ensuring schema")
	var schema string
	switch r.db.DriverName() {
	case "mysql":
		schema = `
CREATE TABLE IF NOT EXISTS table_configurations (
    id INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(191) NOT NULL UNIQUE,
    title VARCHAR(191),
    ordinal INT DEFAULT 0,
    db VARCHAR(191)
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
    sr_created DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at_unix BIGINT DEFAULT (UNIX_TIMESTAMP())
);

CREATE TABLE IF NOT EXISTS table_views (
    id INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    table_name VARCHAR(191) NOT NULL,
    view_name VARCHAR(191) NOT NULL,
    is_default BOOLEAN DEFAULT FALSE,
    sr_created DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at_unix BIGINT DEFAULT (UNIX_TIMESTAMP()),
    UNIQUE KEY unique_table_view (table_name, view_name)
);

CREATE TABLE IF NOT EXISTS table_view_columns (
    id INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    view_id INT NOT NULL,
    column_name VARCHAR(191) NOT NULL,
    is_visible BOOLEAN DEFAULT TRUE,
    column_order INT DEFAULT 0,
    column_width INT,
    sort_order VARCHAR(10),
    sr_created DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at_unix BIGINT DEFAULT (UNIX_TIMESTAMP()),
    FOREIGN KEY (view_id) REFERENCES table_views(id) ON DELETE CASCADE,
    UNIQUE KEY unique_view_column (view_id, column_name)
);

CREATE TABLE IF NOT EXISTS table_recently_viewed (
    id INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    table_id BIGINT NOT NULL,
    sr_created DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at_unix BIGINT DEFAULT (UNIX_TIMESTAMP())
);
`
	default:
		schema = `
CREATE TABLE IF NOT EXISTS table_configurations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    title TEXT,
    ordinal INTEGER DEFAULT 0,
    db TEXT
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
    sr_created DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at_unix INTEGER DEFAULT (strftime('%s', 'now'))
);

CREATE TABLE IF NOT EXISTS table_views (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    table_name TEXT NOT NULL,
    view_name TEXT NOT NULL,
    is_default INTEGER DEFAULT 0,
    sr_created DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at_unix INTEGER DEFAULT (strftime('%s', 'now')),
    UNIQUE(table_name, view_name)
);

CREATE TABLE IF NOT EXISTS table_view_columns (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    view_id INTEGER NOT NULL,
    column_name TEXT NOT NULL,
    is_visible INTEGER DEFAULT 1,
    column_order INTEGER DEFAULT 0,
    column_width INTEGER,
    sort_order TEXT,
    sr_created DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at_unix INTEGER DEFAULT (strftime('%s', 'now')),
    FOREIGN KEY (view_id) REFERENCES table_views(id) ON DELETE CASCADE,
    UNIQUE(view_id, column_name)
);

CREATE TABLE IF NOT EXISTS table_recently_viewed (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    table_id INTEGER NOT NULL,
    sr_created DATETIME DEFAULT CURRENT_TIMESTAMP,
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
	var ordinalCount, titleCount, dbColCount int
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

	err = r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM information_schema.columns WHERE table_name = 'table_configurations' AND column_name = 'db'").Scan(&dbColCount)
	if err != nil {
		return err
	}
	if dbColCount == 0 {
		_, err = r.db.ExecContext(ctx, "ALTER TABLE table_configurations ADD COLUMN db VARCHAR(191)")
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
    sr_created DATETIME DEFAULT CURRENT_TIMESTAMP
);`, t)
	default:
		schema = fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %s (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    sr_created DATETIME DEFAULT CURRENT_TIMESTAMP
);`, t)
	}
	_, err := r.db.ExecContext(ctx, schema)
	if err != nil {
		return err
	}

	// Add sr_created column if it doesn't exist (for existing tables)
	switch r.db.DriverName() {
	case "mysql":
		alterQuery := fmt.Sprintf("ALTER TABLE %s ADD COLUMN IF NOT EXISTS sr_created DATETIME DEFAULT CURRENT_TIMESTAMP", t)
		_, err = r.db.ExecContext(ctx, alterQuery)
		if err != nil {
			log.Warnf("Failed to add sr_created column to table %s: %v", t, err)
		}
	default:
		// SQLite doesn't support IF NOT EXISTS in ALTER TABLE, so we'll check if column exists first
		var count int
		checkQuery := fmt.Sprintf("SELECT COUNT(*) FROM pragma_table_info('%s') WHERE name='sr_created'", t)
		err = r.db.GetContext(ctx, &count, checkQuery)
		if err == nil && count == 0 {
			alterQuery := fmt.Sprintf("ALTER TABLE %s ADD COLUMN sr_created DATETIME DEFAULT CURRENT_TIMESTAMP", t)
			_, err = r.db.ExecContext(ctx, alterQuery)
			if err != nil {
				log.Warnf("Failed to add sr_created column to table %s: %v", t, err)
			}
		}
	}

	return nil
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
		// Use native SQL datetime format
		if r.db.DriverName() == "mysql" {
			typ = "DATETIME"
			if field.DefaultToCurrentTimestamp {
				defaultClause = " DEFAULT CURRENT_TIMESTAMP"
			}
		} else {
			// SQLite uses TEXT for datetime with ISO8601 format
			typ = "TEXT"
			if field.DefaultToCurrentTimestamp {
				defaultClause = " DEFAULT (datetime('now'))"
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

func (r *Repository) ListItemsInTable(ctx context.Context, tcName string) ([]Item, error) {
	tc, err := r.GetTableConfiguration(ctx, tcName)

	if err != nil {
		log.Errorf("failed to get table structure for table %s: %v", tcName, err)
		return nil, fmt.Errorf("failed to get table structure for table %s: %w", tcName, err)
	}

	// First get the column names for this table
	columns, err := r.ListColumns(ctx, tc)
	if err != nil {
		log.Errorf("failed to get columns for table %s: %v", tcName, err)
		return nil, fmt.Errorf("failed to get columns for table %s: %w", tcName, err)
	}

	// Build dynamic SELECT query with all columns
	columnNames := make([]string, 0, len(columns))
	for _, col := range columns {
		columnNames = append(columnNames, col.Name)
	}

	log.Infof("ListItems columnNames: %v", columnNames)

	sortColumn := "sr_created"

	if !slices.Contains(columnNames, sortColumn) {
		sortColumn = "id"
	}

	query := fmt.Sprintf("SELECT `%s` FROM `%s`.`%s` ORDER BY `%s` DESC", strings.Join(columnNames, "`, `"), tc.Db.String, tc.Table.String, sortColumn)
	log.Infof("ListItems SQL Query: %s db:%v tbl:%v", query, tc.Db, tc.Table)

	// Use QueryxContext to get raw rows and manually map them
	rows, err := r.db.QueryxContext(ctx, query)
	if err != nil {
		log.Errorf("Failed to list items in table %s: %v", tcName, err)
		return nil, err
	}
	defer rows.Close()

	// rows iteration follows

	var items []Item
	for rows.Next() {
		log.Infof("Row iteration")
		// Get the row as a map
		rowMap := make(map[string]interface{})
		if err := rows.MapScan(rowMap); err != nil {
			log.Errorf("Failed to map scan row: %v", err)
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
		if createdAt, ok := rowMap["sr_created"]; ok {
			log.Infof("sr_created field found: %v (type: %T)", createdAt, createdAt)
			if createdAtTime, ok := createdAt.(time.Time); ok {
				item.SrCreated = createdAtTime
			} else if createdAtStr, ok := createdAt.(string); ok {
				// Handle string datetime from MySQL
				if parsedTime, err := time.Parse("2006-01-02 15:04:05", createdAtStr); err == nil {
					item.SrCreated = parsedTime
				} else {
					log.Warnf("failed to parse sr_created datetime string: %v", err)
				}
			} else {
				log.Warnf("sr_created field is not time.Time or string, got type: %T, value: %v", createdAt, createdAt)
			}
		} else {
			log.Warnf("sr_created field not found in rowMap")
		}

		// Add all other fields to the dynamic Fields map (including name now)
		for colName, value := range rowMap {
			if colName != "id" && colName != "sr_created" {
				// Handle MySQL byte slice conversion for all fields
				if valueBytes, ok := value.([]uint8); ok {
					item.Fields[colName] = string(valueBytes)
				} else {
					item.Fields[colName] = value
				}
			}
		}

		items = append(items, item)
	}

	log.Infof("ListItems: %d items found", len(items))

	return items, rows.Err()
}

func (r *Repository) CreateItemInTable(ctx context.Context, table string, additionalFields map[string]string) (Item, error) {
	now := time.Now()
	return r.CreateItemInTableWithTimestamp(ctx, table, additionalFields, now)
}

func (r *Repository) CreateItemInTableWithTimestamp(ctx context.Context, table string, additionalFields map[string]string, timestamp time.Time) (Item, error) {
	t := sanitizeTableName(table)

	// Build dynamic INSERT query
	columns := []string{"sr_created"}
	placeholders := []string{"?"}
	values := []interface{}{timestamp}

	for key, value := range additionalFields {
		columns = append(columns, key)
		placeholders = append(placeholders, "?")
		values = append(values, value)
	}

	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", t, strings.Join(columns, ", "), strings.Join(placeholders, ", "))
	log.Infof("Creating item in table %s with fields: %+v, timestamp: %v, query: %s", t, additionalFields, timestamp, query)

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

	item := Item{ID: strconv.FormatInt(lastID, 10), SrCreated: timestamp, Fields: fields}
	log.Infof("Created item: %+v", item)
	return item, nil
}

func (r *Repository) GetItemInTable(ctx context.Context, table string, id string) (Item, error) {
	tc, err := r.GetTableConfiguration(ctx, table)

	if err != nil {
		return Item{}, fmt.Errorf("failed to get table configuration for table %s: %w", table, err)
	}

	// First get the column names for this table
	columns, err := r.ListColumns(ctx, tc)
	if err != nil {
		return Item{}, fmt.Errorf("failed to get columns for table %s: %w", table, err)
	}

	// Build dynamic SELECT query with all columns
	columnNames := make([]string, 0, len(columns))
	for _, col := range columns {
		columnNames = append(columnNames, col.Name)
	}

	query := fmt.Sprintf("SELECT `%s` FROM `%s`.`%s` WHERE `id` = ?", strings.Join(columnNames, "`, `"), tc.Db.String, tc.Table.String)

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
	if createdAt, ok := rowMap["sr_created"]; ok {
		if createdAtTime, ok := createdAt.(time.Time); ok {
			item.SrCreated = createdAtTime
		} else if createdAtStr, ok := createdAt.(string); ok {
			// Handle string datetime from MySQL
			if parsedTime, err := time.Parse("2006-01-02 15:04:05", createdAtStr); err == nil {
				item.SrCreated = parsedTime
			}
		}
	}

	// Add all other fields to the dynamic Fields map (including name now)
	for colName, value := range rowMap {
		if colName != "id" && colName != "sr_created" {
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

func (r *Repository) EditItemInTableWithFields(ctx context.Context, table string, id string, name string, additionalFields map[string]string) (Item, error) {
	t := sanitizeTableName(table)

	// Build dynamic UPDATE query
	setParts := []string{"name = ?"}
	args := []interface{}{name}

	for fieldName, fieldValue := range additionalFields {
		// Sanitize field name to prevent SQL injection
		sanitizedFieldName := sanitizeTableName(fieldName)
		setParts = append(setParts, fmt.Sprintf("`%s` = ?", sanitizedFieldName))
		args = append(args, fieldValue)
	}

	args = append(args, id) // Add id for WHERE clause

	query := fmt.Sprintf("UPDATE `%s` SET %s WHERE `id` = ?", t, strings.Join(setParts, ", "))
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

// ConnectDatabase returns a database connection using MySQL if DB_HOST is set,
// otherwise falls back to sqlite using the provided defaultSQLiteDSN.
func ConnectDatabase(defaultSQLiteDSN string) (*sqlx.DB, error) {
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

func (r *Repository) ListColumns(ctx context.Context, tc *TableConfig) ([]FieldSpec, error) {
	driver := r.db.DriverName()
	specs := make([]FieldSpec, 0, 8)
	switch driver {
	case "mysql":
		type row struct {
			ColumnName string `db:"COLUMN_NAME"`
			DataType   string `db:"DATA_TYPE"`
			IsNullable string `db:"IS_NULLABLE"`
		}
		rows := []row{}
		q := `SELECT COLUMN_NAME, DATA_TYPE, IS_NULLABLE FROM INFORMATION_SCHEMA.COLUMNS WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ? ORDER BY ORDINAL_POSITION`
		if err := r.db.SelectContext(ctx, &rows, q, tc.Db, tc.Table); err != nil {
			return nil, err
		}
		for _, r := range rows {
			// Return the native database type instead of mapping to internal types
			typ := r.DataType
			specs = append(specs, FieldSpec{Name: r.ColumnName, Type: typ, Required: strings.ToUpper(r.IsNullable) == "NO"})
		}
	default: // sqlite
		type srow struct {
			Cid       int     `db:"cid"`
			Name      string  `db:"name"`
			Type      string  `db:"type"`
			NotNull   int     `db:"notnull"`
			DfltValue *string `db:"dflt_value"`
		}
		var rows []srow
		q := fmt.Sprintf("PRAGMA table_info(%s)", tc.Table)
		if err := r.db.SelectContext(ctx, &rows, q); err != nil {
			return nil, err
		}
		for _, r := range rows {
			// Return the native database type instead of mapping to internal types
			typ := r.Type
			specs = append(specs, FieldSpec{Name: r.Name, Type: typ, Required: r.NotNull == 1})
		}
	}
	return specs, nil
}

// GetTableConfiguration returns the structure information for a table
func (r *Repository) GetTableConfiguration(ctx context.Context, tcName string) (*TableConfig, error) {
	t := sanitizeTableName(tcName)

	log.WithFields(log.Fields{
		"tcName": tcName,
	}).Infof("Getting TableConfiguration")

	// Query table_configurations for this table's metadata
	var config TableConfig
	query := "SELECT name, `db`, `table`, COALESCE(title, name) as title, COALESCE(ordinal, 0) as ordinal, create_button_text, icon, view FROM table_configurations WHERE name = ?"
	err := r.db.GetContext(ctx, &config, query, t)

	if err != nil {
		log.Errorf("Failed to get table configuration for table %s: %v", t, err)

		if err == sql.ErrNoRows {
			// Table not found in configurations, return default structure
			return nil, fmt.Errorf("table not found in configurations")
		}
		return nil, err
	}

	log.Infof("TableConfiguration: %+v", config)

	if !config.View.Valid || !config.Table.Valid || !config.Db.Valid {
		return nil, fmt.Errorf("table structure is invalid, missing view, table or db: %+v", config)
	}

	return &config, nil
}

// TableViewColumn represents a column configuration in a table view
type TableViewColumn struct {
	ColumnName  string `db:"column_name"`
	IsVisible   bool   `db:"is_visible"`
	ColumnOrder int    `db:"column_order"`
	SortOrder   string `db:"sort_order"`
}

// TableView represents a saved table view
type TableView struct {
	ID        int               `db:"id"`
	TableName string            `db:"table_name"`
	ViewName  string            `db:"view_name"`
	IsDefault bool              `db:"is_default"`
	Columns   []TableViewColumn `db:"-"`
}

// CreateTableView creates a new table view with its column configurations
func (r *Repository) CreateTableView(ctx context.Context, tableName, viewName string, columns []TableViewColumn) error {
	t := sanitizeTableName(tableName)

	// Start a transaction
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Insert the table view
	var viewID int64
	switch r.db.DriverName() {
	case "mysql":
		result, err := tx.ExecContext(ctx,
			"INSERT INTO table_views (table_name, view_name, is_default) VALUES (?, ?, ?)",
			t, viewName, false)
		if err != nil {
			return err
		}
		viewID, err = result.LastInsertId()
		if err != nil {
			return err
		}
	default:
		result, err := tx.ExecContext(ctx,
			"INSERT INTO table_views (table_name, view_name, is_default) VALUES (?, ?, ?)",
			t, viewName, false)
		if err != nil {
			return err
		}
		viewID, err = result.LastInsertId()
		if err != nil {
			return err
		}
	}

	// Insert column configurations
	for _, col := range columns {
		if !col.IsVisible {
			continue // Only save visible columns
		}

		_, err := tx.ExecContext(ctx,
			"INSERT INTO table_view_columns (view_id, column_name, is_visible, column_order, sort_order) VALUES (?, ?, ?, ?, ?)",
			viewID, col.ColumnName, col.IsVisible, col.ColumnOrder, col.SortOrder)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// UpdateTableView updates an existing table view with its column configurations
func (r *Repository) UpdateTableView(ctx context.Context, viewID int, tableName, viewName string, columns []TableViewColumn) error {
	t := sanitizeTableName(tableName)

	// Start a transaction
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Update the table view
	_, err = tx.ExecContext(ctx,
		"UPDATE table_views SET view_name = ? WHERE id = ? AND table_name = ?",
		viewName, viewID, t)
	if err != nil {
		return err
	}

	// Delete existing column configurations
	_, err = tx.ExecContext(ctx,
		"DELETE FROM table_view_columns WHERE view_id = ?",
		viewID)
	if err != nil {
		return err
	}

	// Insert new column configurations
	for _, col := range columns {
		if !col.IsVisible {
			continue // Only save visible columns
		}

		_, err := tx.ExecContext(ctx,
			"INSERT INTO table_view_columns (view_id, column_name, is_visible, column_order, sort_order) VALUES (?, ?, ?, ?, ?)",
			viewID, col.ColumnName, col.IsVisible, col.ColumnOrder, col.SortOrder)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// GetTableViews retrieves all views for a given table
func (r *Repository) GetTableViews(ctx context.Context, tableName string) ([]TableView, error) {
	t := sanitizeTableName(tableName)

	// Get all views for the table
	rows, err := r.db.QueryxContext(ctx,
		"SELECT id, table_name, view_name, is_default FROM table_views WHERE table_name = ? ORDER BY view_name",
		t)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var views []TableView
	for rows.Next() {
		var view TableView
		err := rows.Scan(&view.ID, &view.TableName, &view.ViewName, &view.IsDefault)
		if err != nil {
			return nil, err
		}

		// Get columns for this view
		columnRows, err := r.db.QueryxContext(ctx,
			"SELECT column_name, is_visible, column_order, sort_order FROM table_view_columns WHERE view_id = ? ORDER BY column_order",
			view.ID)
		if err != nil {
			return nil, err
		}

		var columns []TableViewColumn
		for columnRows.Next() {
			var col TableViewColumn
			err := columnRows.Scan(&col.ColumnName, &col.IsVisible, &col.ColumnOrder, &col.SortOrder)
			if err != nil {
				columnRows.Close()
				return nil, err
			}
			columns = append(columns, col)
		}
		columnRows.Close()

		view.Columns = columns
		views = append(views, view)
	}

	return views, rows.Err()
}

// DeleteTableView deletes a table view and its associated columns
func (r *Repository) DeleteTableView(ctx context.Context, viewID int) error {
	// Start a transaction to ensure atomicity
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Delete the view columns first (due to foreign key constraint)
	_, err = tx.ExecContext(ctx, "DELETE FROM table_view_columns WHERE view_id = ?", viewID)
	if err != nil {
		return err
	}

	// Delete the view
	result, err := tx.ExecContext(ctx, "DELETE FROM table_views WHERE id = ?", viewID)
	if err != nil {
		return err
	}

	// Check if any rows were affected
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return fmt.Errorf("view with ID %d not found", viewID)
	}

	// Commit the transaction
	return tx.Commit()
}

// ForeignKey represents a foreign key constraint
type ForeignKey struct {
	ConstraintName   string `db:"constraint_name"`
	TableName        string `db:"table_name"`
	ColumnName       string `db:"column_name"`
	ReferencedTable  string `db:"referenced_table"`
	ReferencedColumn string `db:"referenced_column"`
	OnDeleteAction   string `db:"on_delete_action"`
	OnUpdateAction   string `db:"on_update_action"`
}

// CreateForeignKey creates a foreign key constraint
func (r *Repository) CreateForeignKey(ctx context.Context, tableName, columnName, referencedTable, referencedColumn, onDeleteAction, onUpdateAction string) error {
	t := sanitizeTableName(tableName)
	refTable := sanitizeTableName(referencedTable)
	col := sanitizeTableName(columnName)
	refCol := sanitizeTableName(referencedColumn)

	// Generate constraint name
	constraintName := fmt.Sprintf("fk_%s_%s_%s_%s", t, col, refTable, refCol)

	// Build the ALTER TABLE statement
	var alterQuery string
	switch r.db.DriverName() {
	case "mysql":
		alterQuery = fmt.Sprintf(
			"ALTER TABLE %s ADD CONSTRAINT %s FOREIGN KEY (%s) REFERENCES %s(%s) ON DELETE %s ON UPDATE %s",
			t, constraintName, col, refTable, refCol, onDeleteAction, onUpdateAction,
		)

	default: // SQLite
		// SQLite has limited foreign key support, but we can still create the constraint
		alterQuery = fmt.Sprintf(
			"ALTER TABLE %s ADD CONSTRAINT %s FOREIGN KEY (%s) REFERENCES %s(%s) ON DELETE %s ON UPDATE %s",
			t, constraintName, col, refTable, refCol, onDeleteAction, onUpdateAction,
		)
	}

	log.Infof("Creating foreign key: %s", alterQuery)

	_, err := r.db.ExecContext(ctx, alterQuery)
	return err
}

// GetForeignKeys retrieves all foreign keys for a given table (bidirectional)
func (r *Repository) GetForeignKeys(ctx context.Context, tableName string) ([]ForeignKey, error) {
	t := sanitizeTableName(tableName)
	var foreignKeys []ForeignKey

	switch r.db.DriverName() {
	case "mysql":
		// Query MySQL information schema for foreign keys in both directions
		query := `
			SELECT
				kcu.CONSTRAINT_NAME as constraint_name,
				kcu.TABLE_NAME as table_name,
				kcu.COLUMN_NAME as column_name,
				kcu.REFERENCED_TABLE_NAME as referenced_table,
				kcu.REFERENCED_COLUMN_NAME as referenced_column,
				COALESCE(rc.DELETE_RULE, 'NO ACTION') as on_delete_action,
				COALESCE(rc.UPDATE_RULE, 'NO ACTION') as on_update_action
			FROM INFORMATION_SCHEMA.KEY_COLUMN_USAGE kcu
			LEFT JOIN INFORMATION_SCHEMA.REFERENTIAL_CONSTRAINTS rc
				ON kcu.CONSTRAINT_NAME = rc.CONSTRAINT_NAME
				AND kcu.TABLE_SCHEMA = rc.CONSTRAINT_SCHEMA
			WHERE kcu.TABLE_SCHEMA = DATABASE()
			AND (kcu.TABLE_NAME = ? OR kcu.REFERENCED_TABLE_NAME = ?)
			AND kcu.REFERENCED_TABLE_NAME IS NOT NULL
			ORDER BY kcu.CONSTRAINT_NAME`

		log.Tracef("GetForeignKeys Query: %v", query)

		var dbName string
		if err := r.db.GetContext(ctx, &dbName, "SELECT DATABASE()"); err != nil {
			return nil, err
		}

		rows, err := r.db.QueryxContext(ctx, query, t, t)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		for rows.Next() {
			var fk ForeignKey
			err := rows.StructScan(&fk)
			if err != nil {
				return nil, err
			}
			foreignKeys = append(foreignKeys, fk)
		}
	default: // SQLite
		// SQLite doesn't have a comprehensive information schema for foreign keys
		// We'll return an empty list for now, but in a real implementation
		// you might want to parse the CREATE TABLE statements
		foreignKeys = []ForeignKey{}
	}

	log.Infof("Foreign keys for table: %v = %v", tableName, foreignKeys)

	return foreignKeys, nil
}

// DeleteForeignKey removes a foreign key constraint
func (r *Repository) DeleteForeignKey(ctx context.Context, constraintName string) error {
	// For MySQL, we need to know the table name to drop the constraint
	// For SQLite, we can drop by constraint name
	var alterQuery string
	switch r.db.DriverName() {
	case "mysql":
		// We need to find the table name first
		var tableName string
		query := `
			SELECT TABLE_NAME
			FROM INFORMATION_SCHEMA.KEY_COLUMN_USAGE
			WHERE TABLE_SCHEMA = DATABASE()
			AND CONSTRAINT_NAME = ?
			LIMIT 1`

		err := r.db.GetContext(ctx, &tableName, query, constraintName)
		if err != nil {
			return err
		}

		alterQuery = fmt.Sprintf("ALTER TABLE %s DROP FOREIGN KEY %s", tableName, constraintName)
	default: // SQLite
		alterQuery = fmt.Sprintf("ALTER TABLE %s DROP CONSTRAINT %s", constraintName)
	}

	_, err := r.db.ExecContext(ctx, alterQuery)
	return err
}

// ChangeColumnType changes the data type of a column
func (r *Repository) ChangeColumnType(ctx context.Context, tableName, columnName, newType string) error {
	t := sanitizeTableName(tableName)
	col := sanitizeTableName(columnName)

	// Use the newType directly as it's now a native database type
	dbType := newType

	// Build the ALTER TABLE statement
	alterQuery := fmt.Sprintf("ALTER TABLE %s MODIFY COLUMN %s %s", t, col, dbType)

	// For SQLite, we need to use a different approach since it doesn't support MODIFY COLUMN
	if r.db.DriverName() == "sqlite" {
		// SQLite doesn't support MODIFY COLUMN directly
		// We would need to create a new table, copy data, drop old table, and rename
		// This is a complex operation that requires careful handling
		// For now, we'll return an error indicating this feature isn't fully supported in SQLite
		return fmt.Errorf("column type changes are not fully supported in SQLite. Please recreate the table with the desired column types")
	}

	_, err := r.db.ExecContext(ctx, alterQuery)
	return err
}

// DropColumn drops a column from a table
func (r *Repository) DropColumn(ctx context.Context, tableName, columnName string) error {
	t := sanitizeTableName(tableName)
	col := sanitizeTableName(columnName)

	// Build the ALTER TABLE statement
	var alterQuery string
	switch r.db.DriverName() {
	case "mysql":
		alterQuery = fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s", t, col)
	default: // SQLite
		// SQLite doesn't support DROP COLUMN directly in older versions
		// For now, we'll return an error indicating this feature isn't fully supported in SQLite
		return fmt.Errorf("column dropping is not fully supported in SQLite. Please recreate the table without the unwanted column")
	}

	_, err := r.db.ExecContext(ctx, alterQuery)
	return err
}

// ChangeColumnName renames a column in a table
func (r *Repository) ChangeColumnName(ctx context.Context, tableName, oldColumnName, newColumnName string) error {
	t := sanitizeTableName(tableName)
	oldCol := sanitizeTableName(oldColumnName)
	newCol := sanitizeTableName(newColumnName)

	if oldCol == "id" || oldCol == "sr_created" {
		return fmt.Errorf("cannot rename system columns (id, sr_created)")
	}

	var alterQuery string
	switch r.db.DriverName() {
	case "mysql":
		alterQuery = fmt.Sprintf("ALTER TABLE %s RENAME COLUMN %s TO %s", t, oldCol, newCol)
	default: // SQLite
		return fmt.Errorf("column renaming is not fully supported in SQLite in this implementation")
	}

	_, err := r.db.ExecContext(ctx, alterQuery)
	return err
}

// InsertRecentlyViewed adds a table and item ID to the recently viewed tracking table
func (r *Repository) InsertRecentlyViewed(ctx context.Context, tableName, itemID string) error {
	// First, try to update existing record if it exists
	var updateQuery string
	switch r.db.DriverName() {
	case "mysql":
		updateQuery = `UPDATE table_recently_viewed
			SET updated_at_unix = UNIX_TIMESTAMP()
			WHERE name = ? AND table_id = ?`
	default: // SQLite
		updateQuery = `UPDATE table_recently_viewed
			SET updated_at_unix = strftime('%s', 'now')
			WHERE name = ? AND table_id = ?`
	}

	result, err := r.db.ExecContext(ctx, updateQuery, tableName, itemID)
	if err != nil {
		return err
	}

	// Check if any rows were updated
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	// If no rows were updated, insert a new record
	if rowsAffected == 0 {
		var insertQuery string
		switch r.db.DriverName() {
		case "mysql":
			insertQuery = `INSERT INTO table_recently_viewed (name, table_id, sr_created, updated_at_unix)
				VALUES (?, ?, NOW(), UNIX_TIMESTAMP())`
		default: // SQLite
			insertQuery = `INSERT INTO table_recently_viewed (name, table_id, sr_created, updated_at_unix)
				VALUES (?, ?, datetime('now'), strftime('%s', 'now'))`
		}

		_, err = r.db.ExecContext(ctx, insertQuery, tableName, itemID)
		return err
	}

	return nil
}

// RecentlyViewedItem represents a recently viewed item with table configuration
type RecentlyViewedItem struct {
	Name          string `db:"name"`
	TableID       string `db:"table_id"`
	Icon          string `db:"icon"`
	UpdatedAtUnix int64  `db:"updated_at_unix"`
	ItemName      string // The actual name/title of the item from the table
	TableTitle    string `db:"title"`
}

// GetMostRecentlyViewed returns the most recently viewed items with table configuration details
func (r *Repository) GetMostRecentlyViewed(ctx context.Context, limit int) ([]RecentlyViewedItem, error) {
	if limit <= 0 {
		limit = 10 // Default limit
	}

	query := `
		SELECT
			rv.name,
			rv.table_id,
			COALESCE(tc.icon, 'DatabaseIcon') as icon,
            rv.updated_at_unix,
            COALESCE(tc.title, rv.name) as title
		FROM table_recently_viewed rv
		LEFT JOIN table_configurations tc ON rv.name = tc.name
		ORDER BY rv.updated_at_unix DESC
		LIMIT ?
	`

	var items []RecentlyViewedItem
	err := r.db.SelectContext(ctx, &items, query, limit)
	if err != nil {
		return nil, err
	}

	// Now fetch the item names for each item
	for i := range items {
		itemName, err := r.getItemName(ctx, items[i].Name, items[i].TableID)
		if err != nil {
			// If we can't get the item name, use the table ID as fallback
			items[i].ItemName = items[i].TableID
			log.WithError(err).WithFields(log.Fields{
				"table": items[i].Name,
				"id":    items[i].TableID,
			}).Warn("Failed to get item name for recently viewed item")
		} else {
			items[i].ItemName = itemName
		}
	}

	return items, nil
}

// getItemName fetches the name field from a specific item in a table
func (r *Repository) getItemName(ctx context.Context, tableName, itemID string) (string, error) {
	t := sanitizeTableName(tableName)

	// Try to get the 'name' field first, fallback to 'title' if it doesn't exist
	var name string
	err := r.db.GetContext(ctx, &name, fmt.Sprintf("SELECT name FROM %s WHERE id = ?", t), itemID)
	if err != nil {
		// If 'name' doesn't exist, try 'title'
		err = r.db.GetContext(ctx, &name, fmt.Sprintf("SELECT title FROM %s WHERE id = ?", t), itemID)
		if err != nil {
			// If neither exists, return the ID as fallback
			return itemID, fmt.Errorf("no name or title field found")
		}
	}

	return name, nil
}

// GetApproxTotalRows returns an approximate total row count across all user tables
func (r *Repository) GetApproxTotalRows(ctx context.Context) (int64, error) {
	switch r.db.DriverName() {
	case "mysql":
		// Use information_schema for approximate row counts
		var total sql.NullInt64
		// If DB name is available via connection, prefer DATABASE() to match current schema
		err := r.db.GetContext(ctx, &total, `
            SELECT COALESCE(SUM(table_rows), 0) AS total_rows
            FROM information_schema.tables
            WHERE table_schema = DATABASE()
              AND table_type = 'BASE TABLE'
        `)
		if err != nil {
			return 0, err
		}
		if total.Valid {
			return total.Int64, nil
		}
		return 0, nil
	default:
		// SQLite (or others): fall back to summing counts from sqlite_master
		// This is more expensive; keep it simple and approximate with 0 if not supported.
		// Optional: Could iterate tables and SUM COUNT(*) but that would be heavy.
		return 0, nil
	}
}
