package repo

import (
	"context"
	"crypto/rand"
	"database/sql"
	"fmt"
	"math/big"
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
	"golang.org/x/crypto/bcrypt"
	_ "modernc.org/sqlite"
)

// sanitizeDatabaseIdentifier ensures the table name is a safe SQL identifier: [a-zA-Z0-9_]+
func sanitizeDatabaseIdentifier(input string) string {
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

type Item struct {
	ID        string                 `db:"id"`
	SrCreated time.Time              `db:"sr_created"`
	SrUpdated time.Time              `db:"sr_updated"`
	Fields    map[string]interface{} `db:"-"` // All dynamic fields including name
}

type Repository struct {
	db *sqlx.DB
}

// DB returns the underlying database connection
func (r *Repository) DB() *sqlx.DB {
	return r.db
}

// Dashboard represents a row in table_dashboards
type Dashboard struct {
	ID   int    `db:"id"`
	Name string `db:"name"`
}

// ListDashboards returns all dashboards
func (r *Repository) ListDashboards(ctx context.Context) ([]Dashboard, error) {
	rows, err := r.db.QueryxContext(ctx, "SELECT id, name FROM table_dashboards ORDER BY id ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	dashboards := make([]Dashboard, 0, 8)
	for rows.Next() {
		var d Dashboard
		if err := rows.Scan(&d.ID, &d.Name); err != nil {
			return nil, err
		}
		dashboards = append(dashboards, d)
	}
	return dashboards, rows.Err()
}

type DashboardComponent struct {
	ID         int            `db:"id"`
	Name       string         `db:"name"`
	TcID       sql.NullInt32  `db:"tc_id"`
	QueryType  sql.NullString `db:"query_type"`
	ColumnName sql.NullString `db:"column_name"`
	Formula    sql.NullString `db:"formula"`
}

// ListDashboardComponents returns components for a given dashboard id
func (r *Repository) ListDashboardComponents(ctx context.Context, dashboardID int) ([]DashboardComponent, error) {
	rows, err := r.db.QueryxContext(ctx, "SELECT id, name, tc_id, query_type, column_name, formula FROM table_dashboard_components WHERE dashboard = ? ORDER BY ordinal ASC, id ASC", dashboardID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]DashboardComponent, 0, 8)
	for rows.Next() {
		var c DashboardComponent
		if err := rows.Scan(&c.ID, &c.Name, &c.TcID, &c.QueryType, &c.ColumnName, &c.Formula); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// DashboardComponentRule represents a rule for a dashboard component
type DashboardComponentRule struct {
	ID        int    `db:"id"`
	Component int    `db:"dashboard_component"`
	Ordinal   int    `db:"ordinal"`
	Operation string `db:"operation"`
	Operand   string `db:"operand"`
}

// GetDashboardComponentRules lists rules, optionally filtered by component id
func (r *Repository) GetDashboardComponentRules(ctx context.Context, component *int) ([]DashboardComponentRule, error) {
	var (
		rows *sqlx.Rows
		err  error
	)
	if component != nil {
		rows, err = r.db.QueryxContext(ctx, "SELECT id, dashboard_component, COALESCE(ordinal, 0) as ordinal, operation, operand FROM table_dashboard_component_rules WHERE dashboard_component = ? ORDER BY ordinal ASC, id ASC", *component)
	} else {
		rows, err = r.db.QueryxContext(ctx, "SELECT id, dashboard_component, COALESCE(ordinal, 0) as ordinal, operation, operand FROM table_dashboard_component_rules ORDER BY dashboard_component ASC, ordinal ASC, id ASC")
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	rules := make([]DashboardComponentRule, 0, 8)
	for rows.Next() {
		var rle DashboardComponentRule
		if err := rows.StructScan(&rle); err != nil {
			return nil, err
		}
		rules = append(rules, rle)
	}
	return rules, rows.Err()
}

// CreateDashboardComponentRule inserts a new rule and returns it
func (r *Repository) CreateDashboardComponentRule(ctx context.Context, component int, ordinal int, operation, operand string) (DashboardComponentRule, error) {
	res, err := r.db.ExecContext(ctx, "INSERT INTO table_dashboard_component_rules (dashboard_component, ordinal, operation, operand) VALUES (?, ?, ?, ?)", component, ordinal, operation, operand)
	if err != nil {
		return DashboardComponentRule{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return DashboardComponentRule{}, err
	}
	return DashboardComponentRule{ID: int(id), Component: component, Ordinal: ordinal, Operation: operation, Operand: operand}, nil
}

func NewRepository(db *sqlx.DB) *Repository {
	return &Repository{db: db}
}

type TableConfig struct {
	Name             string
	Title            string
	Ordinal          int
	Icon             sql.NullString
	CreateButtonText sql.NullString `db:"create_button_text"`
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
	rows, err := r.db.QueryxContext(ctx, "SELECT name, COALESCE(title, name) as title, COALESCE(ordinal,0) as ordinal, create_button_text, icon, `db` FROM table_configurations ORDER BY name, ordinal ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var configs []TableConfig
	for rows.Next() {
		var config TableConfig
		if err := rows.Scan(&config.Name, &config.Title, &config.Ordinal, &config.CreateButtonText, &config.Icon, &config.Db); err != nil {
			return nil, err
		}
		configs = append(configs, config)
	}
	return configs, rows.Err()
}

type NavigationItem struct {
	ID                 int
	Ordinal            int
	TableConfiguration sql.NullInt64
	TableName          sql.NullString
	TableTitle         sql.NullString
	Icon               sql.NullString
	TableView          sql.NullString
	DashboardID        sql.NullInt64
	DashboardName      sql.NullString
	Navigation         sql.NullString
	WorkflowID         sql.NullInt64
	WorkflowName       sql.NullString
}

func (r *Repository) GetNavigation(ctx context.Context) ([]NavigationItem, error) {
	query := `
		SELECT
			tn.id,
			tn.ordinal,
			tn.table_configuration,
			tc.name as table_name,
			COALESCE(tc.title, tc.name) as table_title,
			tc.icon as icon,
            tn.dashboard_id as dashboard_id,
            td.name as dashboard_name,
            tn.name as navigation,
            tn.workflow_id as workflow_id,
            tw.name as workflow_name
		FROM table_navigation tn
		LEFT JOIN table_configurations tc ON tn.table_configuration = tc.id
        LEFT JOIN table_dashboards td ON tn.dashboard_id = td.id
        LEFT JOIN table_workflows tw ON tn.workflow_id = tw.id
		ORDER BY tn.ordinal ASC, tc.name ASC
	`

	rows, err := r.db.QueryxContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []NavigationItem
	for rows.Next() {
		var item NavigationItem
		if err := rows.Scan(
			&item.ID,
			&item.Ordinal,
			&item.TableConfiguration,
			&item.TableName,
			&item.TableTitle,
			&item.Icon,
			&item.DashboardID,
			&item.DashboardName,
			&item.Navigation,
			&item.WorkflowID,
			&item.WorkflowName,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	return items, rows.Err()
}

type Workflow struct {
	ID      int
	Name    string
	Ordinal int
	Icon    sql.NullString
}

func (r *Repository) GetWorkflows(ctx context.Context) ([]Workflow, error) {
	query := `
		SELECT
			id,
			name,
			ordinal,
			icon
		FROM table_workflows
		ORDER BY ordinal ASC, name ASC
	`

	rows, err := r.db.QueryxContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var workflows []Workflow
	for rows.Next() {
		var workflow Workflow
		if err := rows.Scan(
			&workflow.ID,
			&workflow.Name,
			&workflow.Ordinal,
			&workflow.Icon,
		); err != nil {
			return nil, err
		}
		workflows = append(workflows, workflow)
	}

	return workflows, rows.Err()
}

type User struct {
	ID           int
	Username     string
	Password     string
	InitialRoute string
}

func (r *Repository) GetUserByUsername(ctx context.Context, username string) (*User, error) {
	query := "SELECT id, username, password, initial_route FROM table_users WHERE username = ?"

	var user User
	err := r.db.QueryRowxContext(ctx, query, username).Scan(&user.ID, &user.Username, &user.Password, &user.InitialRoute)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // User not found
		}
		return nil, err
	}

	return &user, nil
}

// GetUserByID retrieves a user by their ID
func (r *Repository) GetUserByID(ctx context.Context, userID int) (*User, error) {
	query := "SELECT id, username, password, initial_route FROM table_users WHERE id = ?"

	var user User
	err := r.db.QueryRowxContext(ctx, query, userID).Scan(&user.ID, &user.Username, &user.Password, &user.InitialRoute)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // User not found
		}
		return nil, err
	}

	return &user, nil
}

func (r *Repository) HasUsers(ctx context.Context) (bool, error) {
	query := "SELECT COUNT(*) FROM table_users"
	var count int
	err := r.db.QueryRowxContext(ctx, query).Scan(&count)
	if err != nil {
		return false, err
	}
	hasUsers := count > 0
	log.Debugf("User count in database: %d, has users: %v", count, hasUsers)
	return hasUsers, nil
}

func (r *Repository) CreateDefaultAdminUser(ctx context.Context) error {
	// Check if admin user already exists
	existingUser, err := r.GetUserByUsername(ctx, "admin")
	if err != nil {
		return err
	}
	if existingUser != nil {
		// Admin user already exists, nothing to do
		log.Debug("Admin user already exists, skipping creation")
		return nil
	}

	log.Info("Creating default admin user")

	// Hash the password "admin"
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte("admin"), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	// Insert the admin user
	query := "INSERT INTO table_users (username, password, initial_route) VALUES (?, ?, ?)"
	_, err = r.db.ExecContext(ctx, query, "admin", string(hashedPassword), "/")
	if err != nil {
		return err
	}

	log.Info("Default admin user created successfully")
	return nil
}

// UpdateUserPassword sets a new bcrypt-hashed password for a given username.
func (r *Repository) UpdateUserPassword(ctx context.Context, username, newPassword string) error {
	if username == "" || newPassword == "" {
		return fmt.Errorf("username and new password are required")
	}
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	query := "UPDATE table_users SET password = ? WHERE username = ?"
	res, err := r.db.ExecContext(ctx, query, string(hashedPassword), username)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("user not found")
	}
	return nil
}

type Session struct {
	ID           int
	SessionID    string
	Username     string
	CreatedAt    time.Time
	ExpiresAt    time.Time
	LastAccessed time.Time
	UserAgent    sql.NullString
	IPAddress    sql.NullString
}

func (r *Repository) CreateSession(ctx context.Context, sessionID, username string, expiresAt time.Time, userAgent, ipAddress string) error {
	query := `
		INSERT INTO table_sessions (session_id, username, expires_at, user_agent, ip_address)
		VALUES (?, ?, ?, ?, ?)
	`
	_, err := r.db.ExecContext(ctx, query, sessionID, username, expiresAt, userAgent, ipAddress)
	return err
}

func (r *Repository) GetSession(ctx context.Context, sessionID string) (*Session, error) {
	query := `
		SELECT id, session_id, username, created_at, expires_at, last_accessed, user_agent, ip_address
		FROM table_sessions
		WHERE session_id = ? AND expires_at > CURRENT_TIMESTAMP
	`

	var session Session
	err := r.db.QueryRowxContext(ctx, query, sessionID).Scan(
		&session.ID,
		&session.SessionID,
		&session.Username,
		&session.CreatedAt,
		&session.ExpiresAt,
		&session.LastAccessed,
		&session.UserAgent,
		&session.IPAddress,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Session not found or expired
		}
		return nil, err
	}

	return &session, nil
}

func (r *Repository) GetSessionByUsername(ctx context.Context, username string) (*Session, error) {
	query := `
		SELECT session_id, username, created_at, expires_at, last_accessed, user_agent, ip_address
		FROM table_sessions
		WHERE username = ? AND expires_at > CURRENT_TIMESTAMP
		ORDER BY last_accessed DESC
		LIMIT 1
	`

	var session Session
	err := r.db.QueryRowxContext(ctx, query, username).Scan(
		&session.SessionID,
		&session.Username,
		&session.CreatedAt,
		&session.ExpiresAt,
		&session.LastAccessed,
		&session.UserAgent,
		&session.IPAddress,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Session not found
		}
		return nil, err
	}

	return &session, nil
}

func (r *Repository) UpdateSessionLastAccessed(ctx context.Context, sessionID string) error {
	query := `
		UPDATE table_sessions
		SET last_accessed = CURRENT_TIMESTAMP
		WHERE session_id = ? AND expires_at > CURRENT_TIMESTAMP
	`
	_, err := r.db.ExecContext(ctx, query, sessionID)
	return err
}

func (r *Repository) DeleteSession(ctx context.Context, sessionID string) error {
	query := "DELETE FROM table_sessions WHERE session_id = ?"
	_, err := r.db.ExecContext(ctx, query, sessionID)
	return err
}

func (r *Repository) DeleteUserSessions(ctx context.Context, username string) error {
	query := "DELETE FROM table_sessions WHERE username = ?"
	_, err := r.db.ExecContext(ctx, query, username)
	return err
}

func (r *Repository) CleanupExpiredSessions(ctx context.Context) error {
	query := "DELETE FROM table_sessions WHERE expires_at <= CURRENT_TIMESTAMP"
	result, err := r.db.ExecContext(ctx, query)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		log.Warnf("Could not get rows affected count: %v", err)
	} else {
		log.Infof("Cleaned up %d expired sessions", rowsAffected)
	}

	return nil
}

type DeviceCode struct {
	ID        int
	Code      string
	CreatedAt time.Time
	ExpiresAt time.Time
	ClaimedBy sql.NullString
	ClaimedAt sql.NullTime
}

func (r *Repository) CreateDeviceCode(ctx context.Context, code string, expiresAt time.Time) error {
	query := "INSERT INTO device_codes (code, expires_at) VALUES (?, ?)"
	_, err := r.db.ExecContext(ctx, query, code, expiresAt)
	return err
}

func (r *Repository) GetDeviceCode(ctx context.Context, code string) (*DeviceCode, error) {
	query := `
		SELECT id, code, created_at, expires_at, claimed_by, claimed_at
		FROM device_codes
		WHERE code = ? AND expires_at > CURRENT_TIMESTAMP
	`

	var deviceCode DeviceCode
	err := r.db.QueryRowxContext(ctx, query, code).Scan(
		&deviceCode.ID,
		&deviceCode.Code,
		&deviceCode.CreatedAt,
		&deviceCode.ExpiresAt,
		&deviceCode.ClaimedBy,
		&deviceCode.ClaimedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Device code not found or expired
		}
		return nil, err
	}

	return &deviceCode, nil
}

func (r *Repository) ClaimDeviceCode(ctx context.Context, code, username string) error {
	query := `
		UPDATE device_codes
		SET claimed_by = ?, claimed_at = CURRENT_TIMESTAMP
		WHERE code = ? AND expires_at > CURRENT_TIMESTAMP AND claimed_by IS NULL
	`
	result, err := r.db.ExecContext(ctx, query, username, code)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return fmt.Errorf("device code not found, expired, or already claimed")
	}

	return nil
}

func (r *Repository) CleanupExpiredDeviceCodes(ctx context.Context) error {
	query := "DELETE FROM device_codes WHERE expires_at <= CURRENT_TIMESTAMP"
	result, err := r.db.ExecContext(ctx, query)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		log.Warnf("Could not get rows affected count: %v", err)
	} else {
		log.Infof("Cleaned up %d expired device codes", rowsAffected)
	}

	return nil
}

func (r *Repository) GenerateDeviceCode() (string, error) {
	// Generate a 4-digit random number
	n, err := rand.Int(rand.Reader, big.NewInt(10000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%04d", n.Int64()), nil
}

func (r *Repository) AddColumn(ctx context.Context, db, table string, field FieldSpec) error {
	t := sanitizeDatabaseIdentifier(table)
	col := sanitizeDatabaseIdentifier(field.Name)
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

	query := fmt.Sprintf("ALTER TABLE %s.%s ADD COLUMN %s %s%s%s", db, t, col, typ, notNull, defaultClause)
	_, err := r.db.ExecContext(ctx, query)
	return err
}

type FieldSpec struct {
	Name                      string
	Type                      string
	Required                  bool
	DefaultToCurrentTimestamp bool
}

func (r *Repository) ListItemsInTable(ctx context.Context, tcName string, where map[string]string) ([]Item, error) {
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

	// Build WHERE clause from provided filters.
	// If a value contains '%' we treat it as a LIKE pattern for server-assisted contains search.
	var whereClause string
	var args []interface{}
	if len(where) > 0 {
		parts := make([]string, 0, len(where))
		for k, v := range where {
			col := sanitizeDatabaseIdentifier(k)
			if strings.Contains(v, "%") {
				parts = append(parts, fmt.Sprintf("`%s` LIKE ?", col))
				args = append(args, v)
			} else {
				parts = append(parts, fmt.Sprintf("`%s` = ?", col))
				args = append(args, v)
			}
		}
		whereClause = " WHERE " + strings.Join(parts, " AND ")
	}

	query := fmt.Sprintf("SELECT `%s` FROM `%s`.`%s`%s ORDER BY `%s` DESC", strings.Join(columnNames, "`, `"), tc.Db.String, tc.Table.String, whereClause, sortColumn)
	log.Infof("ListItems SQL Query: %s db:%v tbl:%v", query, tc.Db, tc.Table)

	// Use QueryxContext to get raw rows and manually map them
	rows, err := r.db.QueryxContext(ctx, query, args...)
	if err != nil {
		log.Errorf("Failed to list items in table %s: %v", tcName, err)
		return nil, err
	}
	defer rows.Close()

	// rows iteration follows

	var items []Item
	for rows.Next() {
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
			if idStr, ok := id.(string); ok {
				item.ID = idStr
			} else if idInt, ok := id.(int64); ok {
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
				} else {
					log.Warnf("failed to parse sr_created datetime string: %v", err)
				}
			} else {
				log.Warnf("sr_created field is not time.Time or string, got type: %T, value: %v", createdAt, createdAt)
			}
		}
		if updatedAt, ok := rowMap["sr_updated"]; ok {
			if updatedAtTime, ok := updatedAt.(time.Time); ok {
				item.SrUpdated = updatedAtTime
			} else if updatedAtStr, ok := updatedAt.(string); ok {
				// Handle string datetime from MySQL
				if parsedTime, err := time.Parse("2006-01-02 15:04:05", updatedAtStr); err == nil {
					item.SrUpdated = parsedTime
				} else {
					log.Warnf("failed to parse sr_updated datetime string: %v", err)
				}
			} else {
				log.Warnf("sr_updated field is not time.Time or string, got type: %T, value: %v", updatedAt, updatedAt)
			}
		}

		// Add all other fields to the dynamic Fields map (including name now)
		for colName, value := range rowMap {
			if colName != "id" && colName != "sr_created" && colName != "sr_updated" {
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
	return r.CreateItemInTableWithTimestamp(ctx, table, additionalFields)
}

func (r *Repository) CreateItemInTableWithTimestamp(ctx context.Context, table string, additionalFields map[string]string) (Item, error) {
	tc, err := r.GetTableConfiguration(ctx, table)
	if err != nil {
		return Item{}, fmt.Errorf("failed to get table configuration for table %s: %w", table, err)
	}

	// Check if sr_created and sr_updated columns exist
	columns, err := r.ListColumns(ctx, tc)
	if err != nil {
		return Item{}, fmt.Errorf("failed to get columns for table %s: %w", table, err)
	}

	hasSrCreated := false
	hasSrUpdated := false
	for _, col := range columns {
		if col.Name == "sr_created" {
			hasSrCreated = true
		}
		if col.Name == "sr_updated" {
			hasSrUpdated = true
		}
	}

	// Build dynamic INSERT query
	insertColumns := []string{}
	placeholders := []string{}
	values := []interface{}{}

	// Add sr_created if the column exists
	if hasSrCreated {
		insertColumns = append(insertColumns, "`sr_created`")
		placeholders = append(placeholders, "NOW()")
	}

	// Add sr_updated if the column exists (set to same value as sr_created)
	if hasSrUpdated {
		insertColumns = append(insertColumns, "`sr_updated`")
		placeholders = append(placeholders, "NOW()")
	}

	// Add additional fields
	for key, value := range additionalFields {
		insertColumns = append(insertColumns, fmt.Sprintf("`%s`", key))
		placeholders = append(placeholders, "?")
		values = append(values, value)
	}

	query := fmt.Sprintf("INSERT INTO %s.%s (%s) VALUES (%s)", tc.Db.String, tc.Table.String, strings.Join(insertColumns, ", "), strings.Join(placeholders, ", "))
	// Log the SQL used (without parameter values)
	log.WithFields(log.Fields{"table": tc.Table.String}).Infof("CreateItem SQL: %s", query)

	res, err := r.db.ExecContext(ctx, query, values...)
	if err != nil {
		log.Errorf("Failed to create item: %v", err)
		return Item{}, err
	}
	lastID, _ := res.LastInsertId()

	// Fetch the created item to get the populated timestamp fields
	createdItem, err := r.GetItemInTable(ctx, tc, strconv.FormatInt(lastID, 10))
	if err != nil {
		log.Errorf("Failed to fetch created item: %v", err)
		return Item{}, err
	}

	log.Infof("Created item: %+v", createdItem)
	return createdItem, nil
}

func (r *Repository) GetLastItem(ctx context.Context, tcID int) (Item, error) {
	log.Infof("GetLastItem: %d", tcID)
	tc, err := r.GetTableConfigurationByID(ctx, tcID)

	if err != nil {
		return Item{}, fmt.Errorf("failed to get table configuration for table %d: %w", tcID, err)
	}

	// First get the column names for this table
	columns, err := r.ListColumns(ctx, tc)
	if err != nil {
		return Item{}, fmt.Errorf("failed to get columns for table %d: %w", tcID, err)
	}

	// Build dynamic SELECT query with all columns
	columnNames := make([]string, 0, len(columns))
	for _, col := range columns {
		columnNames = append(columnNames, col.Name)
	}

	query := fmt.Sprintf("SELECT `%s` FROM `%s`.`%s` ORDER BY `id` DESC LIMIT 1", strings.Join(columnNames, "`, `"), tc.Db.String, tc.Table.String)
	log.Infof("GetLastItem SQL Query: %s db:%v tbl:%v", query, tc.Db.String, tc.Table.String)
	rows, err := r.db.QueryxContext(ctx, query)
	if err != nil {
		return Item{}, fmt.Errorf("failed to get last item for table %d: %w", tcID, err)
	}
	defer rows.Close()

	if !rows.Next() {
		return Item{}, fmt.Errorf("no items found for table %d", tcID)
	}

	item := Item{
		Fields: make(map[string]interface{}),
	}

	rowMap := make(map[string]interface{})
	if err := rows.MapScan(rowMap); err != nil {
		return Item{}, fmt.Errorf("failed to scan last item for table %d: %w", tcID, err)
	}

	for colName, value := range rowMap {
		if valueBytes, ok := value.([]uint8); ok {
			item.Fields[colName] = string(valueBytes)
		} else {
			item.Fields[colName] = value
		}
	}

	return item, rows.Err()
}

func (r *Repository) GetItemInTable(ctx context.Context, tc *TableConfig, id string) (Item, error) {
	// First get the column names for this table
	columns, err := r.ListColumns(ctx, tc)
	if err != nil {
		return Item{}, fmt.Errorf("failed to get columns for table %s: %w", tc.Table.String, err)
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
	if updatedAt, ok := rowMap["sr_updated"]; ok {
		if updatedAtTime, ok := updatedAt.(time.Time); ok {
			item.SrUpdated = updatedAtTime
		} else if updatedAtStr, ok := updatedAt.(string); ok {
			// Handle string datetime from MySQL
			if parsedTime, err := time.Parse("2006-01-02 15:04:05", updatedAtStr); err == nil {
				item.SrUpdated = parsedTime
			}
		}
	}

	// Add all other fields to the dynamic Fields map (including name now)
	for colName, value := range rowMap {
		if colName != "id" && colName != "sr_created" && colName != "sr_updated" {
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
	tc, err := r.GetTableConfiguration(ctx, table)

	if err != nil {
		return Item{}, fmt.Errorf("failed to get table configuration for table %s: %w", table, err)
	}

	// Check if sr_updated column exists
	columns, err := r.ListColumns(ctx, tc)
	if err != nil {
		return Item{}, fmt.Errorf("failed to get columns for table %s: %w", table, err)
	}

	hasSrUpdated := false
	hasName := false
	for _, col := range columns {
		if col.Name == "sr_updated" {
			hasSrUpdated = true
		}
		if col.Name == "name" {
			hasName = true
		}
	}

	// Build dynamic UPDATE query
	setParts := []string{}
	args := []interface{}{}

	// Add name field if the column exists and name is provided
	if hasName && name != "" {
		setParts = append(setParts, "`name` = ?")
		args = append(args, name)
	}

	// Add sr_updated if the column exists
	if hasSrUpdated {
		setParts = append(setParts, "`sr_updated` = NOW()")
	}

	for fieldName, fieldValue := range additionalFields {
		// Sanitize field name to prevent SQL injection
		sanitizedFieldName := sanitizeDatabaseIdentifier(fieldName)

		// Check if this field is nullable and if the value is empty or "0"
		// If so, set it to NULL instead of the empty string
		shouldSetNull := false

		// Special handling for workflow_id (nullable foreign key)
		if fieldName == "workflow_id" && (fieldValue == "" || fieldValue == "0") {
			shouldSetNull = true
		} else {
			// Check column metadata for other nullable fields
			var col *FieldSpec
			for i := range columns {
				// Try exact match first
				if columns[i].Name == fieldName {
					col = &columns[i]
					break
				}
				// Try case-insensitive match
				if strings.EqualFold(columns[i].Name, fieldName) {
					col = &columns[i]
					break
				}
			}

			// If field is nullable and value is empty or "0", set to NULL
			if col != nil && !col.Required && (fieldValue == "" || fieldValue == "0") {
				shouldSetNull = true
			}
		}

		if shouldSetNull {
			setParts = append(setParts, fmt.Sprintf("`%s` = NULL", sanitizedFieldName))
		} else {
			setParts = append(setParts, fmt.Sprintf("`%s` = ?", sanitizedFieldName))
			args = append(args, fieldValue)
		}
	}

	// Ensure we have at least one field to update
	if len(setParts) == 0 {
		return Item{}, fmt.Errorf("no fields to update")
	}

	args = append(args, id) // Add id for WHERE clause

	query := fmt.Sprintf("UPDATE `%s`.`%s` SET %s WHERE `id` = ?", tc.Db.String, tc.Table.String, strings.Join(setParts, ", "))
	log.Infof("Executing update query: %s with args: %v", query, args)

	if _, err := r.db.ExecContext(ctx, query, args...); err != nil {
		log.Errorf("Failed to update item: %v", err)
		return Item{}, err
	}

	return r.GetItemInTable(ctx, tc, id)
}

func (r *Repository) DeleteItemInTable(ctx context.Context, table string, id string) (bool, error) {
	tc, err := r.GetTableConfiguration(ctx, table)
	if err != nil {
		return false, fmt.Errorf("failed to get table configuration for table %s: %w", table, err)
	}

	query := fmt.Sprintf("DELETE FROM %s.%s WHERE id = ?", tc.Db.String, tc.Table.String)
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
		q := fmt.Sprintf("PRAGMA table_info(%s)", tc.Table.String)
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

func (r *Repository) GetTableConfigurationByID(ctx context.Context, tcID int) (*TableConfig, error) {
	query := "SELECT name, `db`, `table`, COALESCE(title, name) as title, COALESCE(ordinal, 0) as ordinal, create_button_text, icon FROM table_configurations WHERE id = ?"
	var config TableConfig
	err := r.db.GetContext(ctx, &config, query, tcID)

	if err != nil {
		return nil, fmt.Errorf("failed to get table configuration for table %d: %w", tcID, err)
	}

	return &config, nil
}

// DatabaseTableInfo contains information about a table and its configuration status
type DatabaseTableInfo struct {
	TableName         string         `db:"table_name"`
	HasConfiguration  bool           `db:"has_configuration"`
	ConfigurationName sql.NullString `db:"configuration_name"`
}

// GetDatabaseTables returns all tables in the specified database with their configuration status
func (r *Repository) GetDatabaseTables(ctx context.Context, database string) ([]DatabaseTableInfo, error) {
	log.WithFields(log.Fields{
		"database": database,
	}).Infof("Getting database tables")

	if database == "" {
		database = "main"
	}

	// Query to get all tables from information_schema and join with table_configurations
	query := `
		SELECT
			t.TABLE_NAME as table_name,
			CASE WHEN tc.name IS NOT NULL THEN 1 ELSE 0 END as has_configuration,
			tc.name as configuration_name
		FROM information_schema.TABLES t
		LEFT JOIN table_configurations tc ON t.TABLE_NAME = tc.table AND tc.db = ?
		WHERE t.TABLE_SCHEMA = ?
		AND t.TABLE_TYPE = 'BASE TABLE'
		ORDER BY t.TABLE_NAME
	`

	var tables []DatabaseTableInfo
	err := r.db.SelectContext(ctx, &tables, query, database, database)
	if err != nil {
		log.Errorf("Failed to get database tables: %v", err)
		return nil, fmt.Errorf("failed to get database tables: %w", err)
	}

	log.Infof("Found %d tables in database %s", len(tables), database)
	return tables, nil
}

// CreateTable creates a physical table in the database
func (r *Repository) CreateTable(ctx context.Context, database, table string) error {
	log.WithFields(log.Fields{
		"database": database,
		"table":    table,
	}).Infof("Creating physical table")

	if database == "" {
		database = "main"
	}

	if table == "" {
		return fmt.Errorf("table name is required")
	}

	t := sanitizeDatabaseIdentifier(table)

	// Check if table already exists
	var exists int
	var err error
	if r.db.DriverName() == "mysql" {
		checkQuery := "SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?"
		err = r.db.GetContext(ctx, &exists, checkQuery, database, t)
	} else {
		// SQLite
		checkQuery := "SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?"
		err = r.db.GetContext(ctx, &exists, checkQuery, t)
	}
	if err != nil {
		return fmt.Errorf("failed to check if table exists: %w", err)
	}
	if exists > 0 {
		return fmt.Errorf("table '%s' already exists in database '%s'", t, database)
	}

	// Create table with id, sr_created, and sr_updated columns
	var createQuery string
	if r.db.DriverName() == "mysql" {
		createQuery = fmt.Sprintf(
			"CREATE TABLE %s.%s (id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY, sr_created DATETIME DEFAULT CURRENT_TIMESTAMP, sr_updated DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP)",
			database, t)
	} else {
		// SQLite - doesn't support database qualifiers in CREATE TABLE
		createQuery = fmt.Sprintf(
			"CREATE TABLE %s (id INTEGER PRIMARY KEY AUTOINCREMENT, sr_created TEXT DEFAULT (datetime('now')), sr_updated TEXT DEFAULT (datetime('now')))",
			t)
	}

	_, err = r.db.ExecContext(ctx, createQuery)
	if err != nil {
		log.Errorf("Failed to create table: %v", err)
		return fmt.Errorf("failed to create table: %w", err)
	}

	log.Infof("Created physical table: %s.%s", database, t)
	return nil
}

// CreateTableConfiguration creates a new table configuration entry
func (r *Repository) CreateTableConfiguration(ctx context.Context, name, database, table string) error {
	log.WithFields(log.Fields{
		"name":     name,
		"database": database,
		"table":    table,
	}).Infof("Creating table configuration")

	// If table is empty, use name as table
	if table == "" {
		table = name
	}

	// Check if configuration already exists
	var exists int
	checkQuery := "SELECT COUNT(*) FROM table_configurations WHERE name = ?"
	err := r.db.GetContext(ctx, &exists, checkQuery, name)
	if err != nil {
		return fmt.Errorf("failed to check existing configuration: %w", err)
	}
	if exists > 0 {
		return fmt.Errorf("table configuration '%s' already exists", name)
	}

	// Insert the new configuration
	query := "INSERT INTO table_configurations (name, `db`, `table`) VALUES (?, ?, ?)"
	result, err := r.db.ExecContext(ctx, query, name, database, table)
	if err != nil {
		log.Errorf("Failed to create table configuration: %v", err)
		return fmt.Errorf("failed to insert table configuration: %w", err)
	}

	// Get the ID of the newly created configuration
	configID, err := result.LastInsertId()
	if err != nil {
		log.Errorf("Failed to get last insert ID: %v", err)
		return fmt.Errorf("failed to get configuration ID: %w", err)
	}

	// Create a navigation entry for the new table configuration
	navQuery := "INSERT INTO table_navigation (ordinal, table_configuration) VALUES (99, ?)"
	_, err = r.db.ExecContext(ctx, navQuery, configID)
	if err != nil {
		log.Warnf("Failed to create navigation entry for table configuration %s: %v", name, err)
		// Don't fail the whole operation if navigation entry creation fails
		// The table configuration was created successfully
	} else {
		log.Infof("Created navigation entry for table configuration: %s", name)
	}

	log.Infof("Created table configuration: %s (db: %s, table: %s, view: table)", name, database, table)
	return nil
}

// GetTableConfiguration returns the structure information for a table
func (r *Repository) GetTableConfiguration(ctx context.Context, tcName string) (*TableConfig, error) {
	log.WithFields(log.Fields{
		"tcName": tcName,
	}).Infof("Getting TableConfiguration")

	// Query table_configurations for this table's metadata
	var config TableConfig
	query := "SELECT name, `db`, `table`, COALESCE(title, name) as title, COALESCE(ordinal, 0) as ordinal, create_button_text, icon FROM table_configurations WHERE name = ?"
	err := r.db.GetContext(ctx, &config, query, tcName)

	if err != nil {
		log.Errorf("Failed to get table configuration for table %s: %v", tcName, err)

		if err == sql.ErrNoRows {
			// Table not found in configurations, return default structure
			return nil, fmt.Errorf("table not found in configurations")
		}
		return nil, err
	}

	log.Infof("TableConfiguration: %+v", config)

	if !config.Table.Valid || !config.Db.Valid {
		return nil, fmt.Errorf("table structure is invalid, missing table or db: %+v", config)
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
	ViewType  string            `db:"view_type"` // "table" or "calendar", defaults to "table"
	Columns   []TableViewColumn `db:"-"`
}

// CreateTableView creates a new table view with its column configurations
func (r *Repository) CreateTableView(ctx context.Context, tableName, viewName, viewType string, columns []TableViewColumn) error {
	t := sanitizeDatabaseIdentifier(tableName)

	// Default to "table" if viewType is empty
	if viewType == "" {
		viewType = "table"
	}

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
			"INSERT INTO table_views (table_name, view_name, is_default, view_type) VALUES (?, ?, ?, ?)",
			t, viewName, false, viewType)
		if err != nil {
			return err
		}
		viewID, err = result.LastInsertId()
		if err != nil {
			return err
		}
	default:
		result, err := tx.ExecContext(ctx,
			"INSERT INTO table_views (table_name, view_name, is_default, view_type) VALUES (?, ?, ?, ?)",
			t, viewName, false, viewType)
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
func (r *Repository) UpdateTableView(ctx context.Context, viewID int, tableName, viewName, viewType string, columns []TableViewColumn) error {
	t := sanitizeDatabaseIdentifier(tableName)

	// Default to "table" if viewType is empty
	if viewType == "" {
		viewType = "table"
	}

	// Start a transaction
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Update the table view
	_, err = tx.ExecContext(ctx,
		"UPDATE table_views SET view_name = ?, view_type = ? WHERE id = ? AND table_name = ?",
		viewName, viewType, viewID, t)
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
	t := sanitizeDatabaseIdentifier(tableName)

	// Get all views for the table
	rows, err := r.db.QueryxContext(ctx,
		"SELECT id, table_name, view_name, is_default, COALESCE(view_type, 'table') as view_type FROM table_views WHERE table_name = ? ORDER BY view_name",
		t)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var views []TableView
	for rows.Next() {
		var view TableView
		err := rows.Scan(&view.ID, &view.TableName, &view.ViewName, &view.IsDefault, &view.ViewType)
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
	TableSchema      string `db:"table_schema"`
	TableName        string `db:"table_name"`
	ColumnName       string `db:"column_name"`
	ReferencedSchema string `db:"referenced_schema"`
	ReferencedTable  string `db:"referenced_table"`
	ReferencedColumn string `db:"referenced_column"`
	OnDeleteAction   string `db:"on_delete_action"`
	OnUpdateAction   string `db:"on_update_action"`
}

// CreateForeignKey creates a foreign key constraint
func (r *Repository) CreateForeignKey(ctx context.Context, tableName, columnName, referencedTable, referencedColumn, onDeleteAction, onUpdateAction string) error {
	t := sanitizeDatabaseIdentifier(tableName)
	refTable := sanitizeDatabaseIdentifier(referencedTable)
	col := sanitizeDatabaseIdentifier(columnName)
	refCol := sanitizeDatabaseIdentifier(referencedColumn)

	tc, err := r.GetTableConfiguration(ctx, tableName)
	if err != nil {
		return err
	}

	tcRef, err := r.GetTableConfiguration(ctx, referencedTable)
	if err != nil {
		return err
	}

	// Generate constraint name
	constraintName := fmt.Sprintf("fk_%s_%s_%s_%s", t, col, refTable, refCol)

	// Build the ALTER TABLE statement
	var alterQuery string
	switch r.db.DriverName() {
	case "mysql":
		alterQuery = fmt.Sprintf(
			"ALTER TABLE %s.%s ADD CONSTRAINT %s FOREIGN KEY (%s) REFERENCES %s.%s(%s) ON DELETE %s ON UPDATE %s",
			tc.Db.String, tc.Table.String, constraintName, col, tcRef.Db.String, tcRef.Table.String, refCol, onDeleteAction, onUpdateAction,
		)

	default: // SQLite
		// SQLite has limited foreign key support, but we can still create the constraint
		alterQuery = fmt.Sprintf(
			"ALTER TABLE %s.%s ADD CONSTRAINT %s FOREIGN KEY (%s) REFERENCES %s.%s(%s) ON DELETE %s ON UPDATE %s",
			tc.Db.String, tc.Table.String, constraintName, col, tcRef.Db.String, tcRef.Table.String, refCol, onDeleteAction, onUpdateAction,
		)
	}

	log.Infof("Creating foreign key: %s", alterQuery)

	_, err = r.db.ExecContext(ctx, alterQuery)
	return err
}

// GetForeignKeys retrieves all foreign keys for a given table (bidirectional)
func (r *Repository) GetForeignKeys(ctx context.Context, tableName string) ([]ForeignKey, error) {
	tc, err := r.GetTableConfiguration(ctx, tableName)
	if err != nil {
		return nil, err
	}

	var foreignKeys []ForeignKey

	switch r.db.DriverName() {
	case "mysql":
		// Query MySQL information schema for foreign keys in both directions
		// We need to find foreign keys where the current table is either the source or target
		// Foreign keys can span across different databases, so we search globally
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
			WHERE ((kcu.TABLE_SCHEMA = ? AND kcu.TABLE_NAME = ?)
			OR (kcu.REFERENCED_TABLE_SCHEMA = ? AND kcu.REFERENCED_TABLE_NAME = ?))
			AND kcu.REFERENCED_TABLE_NAME IS NOT NULL
			ORDER BY kcu.CONSTRAINT_NAME`

		log.Tracef("GetForeignKeys Query: %v", query)

		rows, err := r.db.QueryxContext(ctx, query, tc.Db.String, tc.Table.String, tc.Db.String, tc.Table.String)
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
			WHERE CONSTRAINT_NAME = ?
			LIMIT 1`

		err := r.db.GetContext(ctx, &tableName, query, constraintName)
		if err != nil {
			return err
		}

		alterQuery = fmt.Sprintf("ALTER TABLE %s DROP FOREIGN KEY %s", tableName, constraintName)
	default: // SQLite
		return fmt.Errorf("dropping foreign keys is not supported on SQLite in this implementation")
	}

	_, err := r.db.ExecContext(ctx, alterQuery)
	return err
}

// ChangeColumnType changes the data type of a column
func (r *Repository) ChangeColumnType(ctx context.Context, tableName, columnName, newType string) error {
	tc, err := r.GetTableConfiguration(ctx, tableName)
	if err != nil {
		return err
	}

	col := sanitizeDatabaseIdentifier(columnName)

	// Use the newType directly as it's now a native database type
	dbType := newType

	// Build the ALTER TABLE statement
	alterQuery := fmt.Sprintf("ALTER TABLE %s.%s MODIFY COLUMN %s %s", tc.Db.String, tc.Table.String, col, dbType)

	// For SQLite, we need to use a different approach since it doesn't support MODIFY COLUMN
	if r.db.DriverName() == "sqlite" {
		// SQLite doesn't support MODIFY COLUMN directly
		// We would need to create a new table, copy data, drop old table, and rename
		// This is a complex operation that requires careful handling
		// For now, we'll return an error indicating this feature isn't fully supported in SQLite
		return fmt.Errorf("column type changes are not fully supported in SQLite. Please recreate the table with the desired column types")
	}

	_, err = r.db.ExecContext(ctx, alterQuery)
	return err
}

// DropColumn drops a column from a table
func (r *Repository) DropColumn(ctx context.Context, tableName, columnName string) error {
	tc, err := r.GetTableConfiguration(ctx, tableName)
	if err != nil {
		return err
	}

	col := sanitizeDatabaseIdentifier(columnName)

	// Build the ALTER TABLE statement
	var alterQuery string
	switch r.db.DriverName() {
	case "mysql":
		alterQuery = fmt.Sprintf("ALTER TABLE %s.%s DROP COLUMN %s", tc.Db.String, tc.Table.String, col)
	default: // SQLite
		// SQLite doesn't support DROP COLUMN directly in older versions
		// For now, we'll return an error indicating this feature isn't fully supported in SQLite
		return fmt.Errorf("column dropping is not fully supported in SQLite. Please recreate the table without the unwanted column")
	}

	_, err = r.db.ExecContext(ctx, alterQuery)
	return err
}

// ChangeColumnName renames a column in a table
func (r *Repository) ChangeColumnName(ctx context.Context, tableName, oldColumnName, newColumnName string) error {
	tc, err := r.GetTableConfiguration(ctx, tableName)
	if err != nil {
		return err
	}

	oldCol := sanitizeDatabaseIdentifier(oldColumnName)
	newCol := sanitizeDatabaseIdentifier(newColumnName)

	if oldCol == "id" || oldCol == "sr_created" {
		return fmt.Errorf("cannot rename system columns (id, sr_created)")
	}

	var alterQuery string
	switch r.db.DriverName() {
	case "mysql":
		alterQuery = fmt.Sprintf("ALTER TABLE %s.%s RENAME COLUMN %s TO %s", tc.Db.String, tc.Table.String, oldCol, newCol)
	default: // SQLite
		return fmt.Errorf("column renaming is not fully supported in SQLite in this implementation")
	}

	_, err = r.db.ExecContext(ctx, alterQuery)
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
	t := sanitizeDatabaseIdentifier(tableName)

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

// UserBookmark represents a user bookmark
type UserBookmark struct {
	ID               int
	UserID           int
	NavigationItemID int
	NavigationItem   *NavigationItem
	Title            sql.NullString
}

// GetUserBookmarks retrieves all bookmarks for a specific user
func (r *Repository) GetUserBookmarks(ctx context.Context, userID int) ([]UserBookmark, error) {
	query := `
		SELECT
			ub.id,
			ub.user,
			ub.navigation_item,
			tn.id as nav_id,
			tn.ordinal,
			tn.table_configuration,
			tc.name as table_name,
			tc.icon as table_icon,
			tn.dashboard_id as dashboard_id,
			td.name as dashboard_name,
			tn.name as title
		FROM table_user_bookmarks ub
		LEFT JOIN table_navigation tn ON ub.navigation_item = tn.id
		LEFT JOIN table_configurations tc ON tn.table_configuration = tc.id
		LEFT JOIN table_dashboards td ON tn.dashboard_id = td.id
		WHERE ub.user = ?
		ORDER BY ub.id DESC
	`

	rows, err := r.db.QueryxContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var bookmarks []UserBookmark
	for rows.Next() {
		var bookmark UserBookmark
		var navItem NavigationItem
		err := rows.Scan(
			&bookmark.ID,
			&bookmark.UserID,
			&bookmark.NavigationItemID,
			&navItem.ID,
			&navItem.Ordinal,
			&navItem.TableConfiguration,
			&navItem.TableName,
			&navItem.Icon,
			&navItem.TableView,
			&navItem.DashboardID,
			&navItem.DashboardName,
			&bookmark.Title,
		)
		if err != nil {
			return nil, err
		}
		bookmark.NavigationItem = &navItem
		bookmarks = append(bookmarks, bookmark)
	}

	return bookmarks, rows.Err()
}

// CreateUserBookmark creates a new bookmark for a user
func (r *Repository) CreateUserBookmark(ctx context.Context, userID, navigationItemID int) (*UserBookmark, error) {
	// Check if bookmark already exists
	var count int
	err := r.db.GetContext(ctx, &count,
		"SELECT COUNT(*) FROM table_user_bookmarks WHERE user = ? AND navigation_item = ?",
		userID, navigationItemID)
	if err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, fmt.Errorf("bookmark already exists")
	}

	// Insert new bookmark
	result, err := r.db.ExecContext(ctx,
		"INSERT INTO table_user_bookmarks (user, navigation_item) VALUES (?, ?)",
		userID, navigationItemID)
	if err != nil {
		return nil, err
	}

	bookmarkID, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	// Get the created bookmark with navigation item details
	bookmarks, err := r.GetUserBookmarks(ctx, userID)
	if err != nil {
		return nil, err
	}

	for _, bookmark := range bookmarks {
		if bookmark.ID == int(bookmarkID) {
			return &bookmark, nil
		}
	}

	return nil, fmt.Errorf("failed to retrieve created bookmark")
}

// DeleteUserBookmark removes a bookmark for a user
func (r *Repository) DeleteUserBookmark(ctx context.Context, userID, bookmarkID int) error {
	result, err := r.db.ExecContext(ctx,
		"DELETE FROM table_user_bookmarks WHERE id = ? AND user = ?",
		bookmarkID, userID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return fmt.Errorf("bookmark not found or not owned by user")
	}

	return nil
}

// API Key related methods

type APIKey struct {
	ID         int
	UserID     int
	Name       string
	KeyHash    string
	CreatedAt  time.Time
	LastUsedAt *time.Time
	ExpiresAt  *time.Time
	IsActive   bool
}

type ConditionalFormattingRule struct {
	ID             int
	TableName      string
	ColumnName     string
	ConditionType  string
	ConditionValue string
	FormatType     string
	FormatValue    string
	Priority       int
	IsActive       bool
	SrCreated      time.Time
	UpdatedAtUnix  int64
}

// CreateAPIKey creates a new API key for a user
func (r *Repository) CreateAPIKey(ctx context.Context, userID int, name, keyHash string, expiresAt *time.Time) (*APIKey, error) {
	query := `
		INSERT INTO table_api_keys (user_id, name, key_hash, expires_at)
		VALUES (?, ?, ?, ?)
	`
	result, err := r.db.ExecContext(ctx, query, userID, name, keyHash, expiresAt)
	if err != nil {
		return nil, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	// Retrieve the created API key
	return r.GetAPIKeyByID(ctx, int(id))
}

// GetAPIKeyByID retrieves an API key by its ID
func (r *Repository) GetAPIKeyByID(ctx context.Context, id int) (*APIKey, error) {
	query := `
		SELECT id, user_id, name, key_hash, created_at, last_used_at, expires_at, is_active
		FROM table_api_keys
		WHERE id = ?
	`

	var apiKey APIKey
	err := r.db.QueryRowxContext(ctx, query, id).Scan(
		&apiKey.ID,
		&apiKey.UserID,
		&apiKey.Name,
		&apiKey.KeyHash,
		&apiKey.CreatedAt,
		&apiKey.LastUsedAt,
		&apiKey.ExpiresAt,
		&apiKey.IsActive,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	return &apiKey, nil
}

// GetAPIKeyByHash retrieves an API key by its hash
func (r *Repository) GetAPIKeyByHash(ctx context.Context, keyHash string) (*APIKey, error) {
	query := `
		SELECT id, user_id, name, key_hash, created_at, last_used_at, expires_at, is_active
		FROM table_api_keys
		WHERE key_hash = ? AND is_active = 1
	`

	var apiKey APIKey
	err := r.db.QueryRowxContext(ctx, query, keyHash).Scan(
		&apiKey.ID,
		&apiKey.UserID,
		&apiKey.Name,
		&apiKey.KeyHash,
		&apiKey.CreatedAt,
		&apiKey.LastUsedAt,
		&apiKey.ExpiresAt,
		&apiKey.IsActive,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	// Check if the key has expired
	if apiKey.ExpiresAt != nil && apiKey.ExpiresAt.Before(time.Now()) {
		return nil, nil // Treat expired keys as not found
	}

	return &apiKey, nil
}

// GetUserAPIKeys retrieves all API keys for a user
func (r *Repository) GetUserAPIKeys(ctx context.Context, userID int) ([]APIKey, error) {
	query := `
		SELECT id, user_id, name, key_hash, created_at, last_used_at, expires_at, is_active
		FROM table_api_keys
		WHERE user_id = ?
		ORDER BY created_at DESC
	`

	rows, err := r.db.QueryxContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var apiKeys []APIKey
	for rows.Next() {
		var apiKey APIKey
		err := rows.Scan(
			&apiKey.ID,
			&apiKey.UserID,
			&apiKey.Name,
			&apiKey.KeyHash,
			&apiKey.CreatedAt,
			&apiKey.LastUsedAt,
			&apiKey.ExpiresAt,
			&apiKey.IsActive,
		)
		if err != nil {
			return nil, err
		}
		apiKeys = append(apiKeys, apiKey)
	}

	return apiKeys, nil
}

// UpdateAPIKeyLastUsed updates the last used timestamp for an API key
func (r *Repository) UpdateAPIKeyLastUsed(ctx context.Context, keyHash string) error {
	query := `
		UPDATE table_api_keys
		SET last_used_at = CURRENT_TIMESTAMP
		WHERE key_hash = ?
	`
	_, err := r.db.ExecContext(ctx, query, keyHash)
	return err
}

// DeactivateAPIKey deactivates an API key
func (r *Repository) DeactivateAPIKey(ctx context.Context, userID, apiKeyID int) error {
	query := `
		UPDATE table_api_keys
		SET is_active = 0
		WHERE id = ? AND user_id = ?
	`
	result, err := r.db.ExecContext(ctx, query, apiKeyID, userID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return fmt.Errorf("API key not found or not owned by user")
	}

	return nil
}

// DeleteAPIKey permanently deletes an API key
func (r *Repository) DeleteAPIKey(ctx context.Context, userID, apiKeyID int) error {
	query := `
		DELETE FROM table_api_keys
		WHERE id = ? AND user_id = ?
	`
	result, err := r.db.ExecContext(ctx, query, apiKeyID, userID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return fmt.Errorf("API key not found or not owned by user")
	}

	return nil
}

// Conditional Formatting Rule related methods

// GetConditionalFormattingRules retrieves conditional formatting rules
func (r *Repository) GetConditionalFormattingRules(ctx context.Context, userID int, tableName string) ([]*ConditionalFormattingRule, error) {
	// First check if sr_created column exists in table_conditional_formatting_rules
	hasSrCreated := false
	if r.db.DriverName() == "mysql" {
		var count int
		checkQuery := `SELECT COUNT(*) FROM information_schema.COLUMNS
			WHERE TABLE_SCHEMA = DATABASE()
			AND TABLE_NAME = 'table_conditional_formatting_rules'
			AND COLUMN_NAME = 'sr_created'`
		err := r.db.GetContext(ctx, &count, checkQuery)
		if err == nil && count > 0 {
			hasSrCreated = true
		}
	} else {
		// SQLite - check using PRAGMA
		type colInfo struct {
			Cid  int    `db:"cid"`
			Name string  `db:"name"`
			Type string  `db:"type"`
		}
		var cols []colInfo
		err := r.db.SelectContext(ctx, &cols, "PRAGMA table_info(table_conditional_formatting_rules)")
		if err == nil {
			for _, col := range cols {
				if col.Name == "sr_created" {
					hasSrCreated = true
					break
				}
			}
		}
	}

	var query string
	var args []interface{}

	if hasSrCreated {
		// Query with sr_created column
		if tableName != "" {
			query = `
				SELECT id, table_name, column_name, condition_type, condition_value,
				       format_type, format_value, priority, is_active, sr_created, updated_at_unix
				FROM table_conditional_formatting_rules
				WHERE table_name = ?
				ORDER BY priority ASC, id ASC
			`
			args = []interface{}{tableName}
		} else {
			query = `
				SELECT id, table_name, column_name, condition_type, condition_value,
				       format_type, format_value, priority, is_active, sr_created, updated_at_unix
				FROM table_conditional_formatting_rules
				ORDER BY table_name ASC, priority ASC, id ASC
			`
			args = []interface{}{}
		}
	} else {
		// Query without sr_created column (for older schema versions)
		if tableName != "" {
			query = `
				SELECT id, table_name, column_name, condition_type, condition_value,
				       format_type, format_value, priority, is_active, updated_at_unix
				FROM table_conditional_formatting_rules
				WHERE table_name = ?
				ORDER BY priority ASC, id ASC
			`
			args = []interface{}{tableName}
		} else {
			query = `
				SELECT id, table_name, column_name, condition_type, condition_value,
				       format_type, format_value, priority, is_active, updated_at_unix
				FROM table_conditional_formatting_rules
				ORDER BY table_name ASC, priority ASC, id ASC
			`
			args = []interface{}{}
		}
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rules []*ConditionalFormattingRule
	for rows.Next() {
		var rule ConditionalFormattingRule
		var srCreatedStr string

		if hasSrCreated {
			err := rows.Scan(
				&rule.ID,
				&rule.TableName,
				&rule.ColumnName,
				&rule.ConditionType,
				&rule.ConditionValue,
				&rule.FormatType,
				&rule.FormatValue,
				&rule.Priority,
				&rule.IsActive,
				&srCreatedStr,
				&rule.UpdatedAtUnix,
			)
			if err != nil {
				return nil, err
			}

			// Parse sr_created timestamp - try multiple formats
			if srCreatedStr != "" {
				// Try ISO 8601 format first (RFC3339) - used by SQLite
				rule.SrCreated, err = time.Parse(time.RFC3339, srCreatedStr)
				if err != nil {
					// Fallback to MySQL datetime format
					rule.SrCreated, err = time.Parse("2006-01-02 15:04:05", srCreatedStr)
					if err != nil {
						// Try ISO 8601 without timezone
						rule.SrCreated, err = time.Parse("2006-01-02T15:04:05", srCreatedStr)
						if err != nil {
							// Don't fail if parsing fails, just log and continue with zero time
							log.Warnf("Failed to parse sr_created timestamp '%s': %v", srCreatedStr, err)
						}
					}
				}
			}
		} else {
			// Scan without sr_created
			err := rows.Scan(
				&rule.ID,
				&rule.TableName,
				&rule.ColumnName,
				&rule.ConditionType,
				&rule.ConditionValue,
				&rule.FormatType,
				&rule.FormatValue,
				&rule.Priority,
				&rule.IsActive,
				&rule.UpdatedAtUnix,
			)
			if err != nil {
				return nil, err
			}
			// sr_created will remain as zero time
		}

		rules = append(rules, &rule)
	}

	return rules, nil
}

// CreateConditionalFormattingRule creates a new conditional formatting rule
func (r *Repository) CreateConditionalFormattingRule(ctx context.Context, userID int, rule *ConditionalFormattingRule) (int, error) {
	query := `
		INSERT INTO table_conditional_formatting_rules
		(table_name, column_name, condition_type, condition_value, format_type, format_value, priority, is_active, sr_created, updated_at_unix)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, NOW(), UNIX_TIMESTAMP())
	`

	result, err := r.db.ExecContext(ctx, query,
		rule.TableName,
		rule.ColumnName,
		rule.ConditionType,
		rule.ConditionValue,
		rule.FormatType,
		rule.FormatValue,
		rule.Priority,
		rule.IsActive,
	)
	if err != nil {
		return 0, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}

	return int(id), nil
}

// DeleteConditionalFormattingRule deletes a conditional formatting rule
func (r *Repository) DeleteConditionalFormattingRule(ctx context.Context, userID int, ruleID int) error {
	query := `DELETE FROM table_conditional_formatting_rules WHERE id = ?`

	result, err := r.db.ExecContext(ctx, query, ruleID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return fmt.Errorf("conditional formatting rule not found")
	}

	return nil
}

// UpdateConditionalFormattingRule updates an existing conditional formatting rule
func (r *Repository) UpdateConditionalFormattingRule(ctx context.Context, userID int, rule *ConditionalFormattingRule) error {
	query := `
		UPDATE table_conditional_formatting_rules
		SET table_name = ?, column_name = ?, condition_type = ?, condition_value = ?,
		    format_type = ?, format_value = ?, priority = ?, is_active = ?, updated_at_unix = UNIX_TIMESTAMP()
		WHERE id = ?
	`

	result, err := r.db.ExecContext(ctx, query,
		rule.TableName,
		rule.ColumnName,
		rule.ConditionType,
		rule.ConditionValue,
		rule.FormatType,
		rule.FormatValue,
		rule.Priority,
		rule.IsActive,
		rule.ID,
	)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return fmt.Errorf("conditional formatting rule not found")
	}

	return nil
}
