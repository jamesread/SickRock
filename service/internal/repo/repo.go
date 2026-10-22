package repo

import (
	"cmp"
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
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
	SrCreated time.Time      `db:"sr_created"`
	SrUpdated time.Time      `db:"sr_updated"`
	Fields    map[string]any `db:"-"`
	ID        string         `db:"id"`
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
	Name string `db:"name"`
	ID   int    `db:"id"`
}

// CreateDashboard inserts a dashboard and optionally a navigation entry. Returns the new dashboard id.
func (r *Repository) CreateDashboard(ctx context.Context, name string, createNavigation bool) (int, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return 0, fmt.Errorf("dashboard name is required")
	}

	var exists int
	if err := r.db.GetContext(ctx, &exists, "SELECT COUNT(*) FROM table_dashboards WHERE name = ?", name); err != nil {
		return 0, fmt.Errorf("failed to check existing dashboard: %w", err)
	}
	if exists > 0 {
		return 0, fmt.Errorf("dashboard '%s' already exists", name)
	}

	result, err := r.db.ExecContext(ctx, "INSERT INTO table_dashboards (name) VALUES (?)", name)
	if err != nil {
		return 0, fmt.Errorf("failed to insert dashboard: %w", err)
	}
	dashboardID, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("failed to get dashboard id: %w", err)
	}

	if createNavigation {
		r.createDashboardNavigation(ctx, dashboardID, name)
	}

	return int(dashboardID), nil
}

// createDashboardNavigation adds a sidebar link for a new dashboard; failures are logged, not returned.
func (r *Repository) createDashboardNavigation(ctx context.Context, dashboardID int64, name string) {
	navQuery := "INSERT INTO table_navigation (ordinal, dashboard_id, name) VALUES (99, ?, ?)"
	if _, err := r.db.ExecContext(ctx, navQuery, dashboardID, name); err != nil {
		log.Warnf("Failed to create navigation entry for dashboard %s: %v", name, err)
		return
	}
	log.Infof("Created navigation entry for dashboard: %s", name)
}

// ListDashboards returns all dashboards
func (r *Repository) ListDashboards(ctx context.Context) ([]Dashboard, error) {
	rows, err := r.db.QueryxContext(ctx, "SELECT id, name FROM table_dashboards ORDER BY id ASC")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

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
	Name       string         `db:"name"`
	QueryType  sql.NullString `db:"query_type"`
	ColumnName sql.NullString `db:"column_name"`
	Formula    sql.NullString `db:"formula"`
	ID         int            `db:"id"`
	TcID       sql.NullInt32  `db:"tc_id"`
}

// ListDashboardComponents returns components for a given dashboard id
func (r *Repository) ListDashboardComponents(ctx context.Context, dashboardID int) ([]DashboardComponent, error) {
	rows, err := r.db.QueryxContext(ctx, "SELECT id, name, tc_id, query_type, column_name, formula FROM table_dashboard_components WHERE dashboard = ? ORDER BY ordinal ASC, id ASC", dashboardID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
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
	Operation string `db:"operation"`
	Operand   string `db:"operand"`
	ID        int    `db:"id"`
	Component int    `db:"dashboard_component"`
	Ordinal   int    `db:"ordinal"`
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
	defer func() { _ = rows.Close() }()
	return scanDashboardComponentRules(rows)
}

func scanDashboardComponentRules(rows *sqlx.Rows) ([]DashboardComponentRule, error) {
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
	Name              string
	Title             string         `db:"title"`
	Icon              sql.NullString `db:"icon"`
	CreateButtonText  sql.NullString `db:"create_button_text"`
	CreateDelegate    sql.NullString `db:"create_delegate"`
	RowName           sql.NullString `db:"row_name"`
	RowNamePlural     sql.NullString `db:"row_name_plural"`
	Table             sql.NullString `db:"table"`
	Db                sql.NullString `db:"db"`
	PrimaryKeyColumn  sql.NullString `db:"primary_key_column"`
	DefaultSortColumn sql.NullString `db:"default_sort_column"`
	Ordinal           int            `db:"ordinal"`
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
	rows, err := r.db.QueryxContext(ctx, "SELECT "+tableConfigurationSelectColumns+" FROM table_configurations ORDER BY name, ordinal ASC")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var configs []TableConfig
	for rows.Next() {
		var config TableConfig
		if err := rows.StructScan(&config); err != nil {
			return nil, err
		}
		configs = append(configs, config)
	}
	return configs, rows.Err()
}

type NavigationItem struct {
	TableView          sql.NullString
	TableName          sql.NullString
	TableTitle         sql.NullString
	Icon               sql.NullString
	DashboardName      sql.NullString
	Navigation         sql.NullString
	TitleOverride      sql.NullString
	WorkflowName       sql.NullString
	TableConfiguration sql.NullInt64
	DashboardID        sql.NullInt64
	WorkflowID         sql.NullInt64
	Ordinal            int
	ID                 int
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
            tn.title as link_title,
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
	defer func() { _ = rows.Close() }()

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
			&item.TitleOverride,
			&item.WorkflowID,
			&item.WorkflowName,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	return items, rows.Err()
}

// WorkflowNavigationMemberInput is one navigation row assigned to a workflow hub.
type WorkflowNavigationMemberInput struct {
	NavigationItemID int
	Ordinal          int
}

// GetNavigationItemsForWorkflowMembership returns navigation rows grouped by workflow hub id.
// Each item's Ordinal reflects membership order (not global sidebar order).
func (r *Repository) GetNavigationItemsForWorkflowMembership(ctx context.Context) (map[int][]NavigationItem, error) {
	query := `
		SELECT
			wnm.workflow_id,
			wnm.ordinal as member_ordinal,
			tn.id,
			tn.ordinal,
			tn.table_configuration,
			tc.name as table_name,
			COALESCE(tc.title, tc.name) as table_title,
			tc.icon as icon,
            tn.dashboard_id as dashboard_id,
            td.name as dashboard_name,
            tn.name as navigation,
            tn.title as link_title,
            tn.workflow_id as nav_workflow_id,
            tw.name as workflow_name
		FROM workflow_navigation_members wnm
		INNER JOIN table_navigation tn ON wnm.navigation_item_id = tn.id
		LEFT JOIN table_configurations tc ON tn.table_configuration = tc.id
        LEFT JOIN table_dashboards td ON tn.dashboard_id = td.id
        LEFT JOIN table_workflows tw ON tn.workflow_id = tw.id
		ORDER BY wnm.workflow_id ASC, wnm.ordinal ASC, tn.id ASC
	`

	rows, err := r.db.QueryxContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	result := make(map[int][]NavigationItem)
	for rows.Next() {
		var bucketWorkflowID int
		var memberOrdinal int
		var item NavigationItem
		var navWorkflowID sql.NullInt64
		if err := rows.Scan(
			&bucketWorkflowID,
			&memberOrdinal,
			&item.ID,
			&item.Ordinal,
			&item.TableConfiguration,
			&item.TableName,
			&item.TableTitle,
			&item.Icon,
			&item.DashboardID,
			&item.DashboardName,
			&item.Navigation,
			&item.TitleOverride,
			&navWorkflowID,
			&item.WorkflowName,
		); err != nil {
			return nil, err
		}
		item.Ordinal = memberOrdinal
		item.WorkflowID = navWorkflowID
		result[bucketWorkflowID] = append(result[bucketWorkflowID], item)
	}
	return result, rows.Err()
}

func (r *Repository) SetWorkflowNavigationMembers(ctx context.Context, workflowID int, members []WorkflowNavigationMemberInput) error {
	if workflowID <= 0 {
		return fmt.Errorf("workflow_id is required")
	}

	return r.inTx(ctx, func(tx *sqlx.Tx) error {
		if _, err := tx.ExecContext(ctx, "DELETE FROM workflow_navigation_members WHERE workflow_id = ?", workflowID); err != nil {
			return fmt.Errorf("clear workflow members: %w", err)
		}
		for _, m := range members {
			if m.NavigationItemID <= 0 {
				continue
			}
			if _, err := tx.ExecContext(ctx,
				"INSERT INTO workflow_navigation_members (workflow_id, navigation_item_id, ordinal) VALUES (?, ?, ?)",
				workflowID, m.NavigationItemID, cmp.Or(max(m.Ordinal, 0), 99),
			); err != nil {
				return fmt.Errorf("insert workflow member: %w", err)
			}
		}
		return nil
	})
}

type Workflow struct {
	Name    string
	Icon    sql.NullString
	ID      int
	Ordinal int
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
	defer func() { _ = rows.Close() }()

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
	Username     string
	Password     string
	InitialRoute string
	ID           int
}

func (r *Repository) GetUserByUsername(ctx context.Context, username string) (*User, error) {
	query := "SELECT id, username, password, initial_route FROM table_users WHERE username = ?"

	var user User
	err := r.db.QueryRowxContext(ctx, query, username).Scan(&user.ID, &user.Username, &user.Password, &user.InitialRoute)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
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
		if errors.Is(err, sql.ErrNoRows) {
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
		log.Debug("Admin user already exists, skipping creation")
		return nil
	}
	return r.insertDefaultAdminUser(ctx)
}

func (r *Repository) insertDefaultAdminUser(ctx context.Context) error {
	log.Info("Creating default admin user")
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte("admin"), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	query := "INSERT INTO table_users (username, password, initial_route) VALUES (?, ?, ?)"
	if _, err = r.db.ExecContext(ctx, query, "admin", string(hashedPassword), "/"); err != nil {
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
	CreatedAt    time.Time
	ExpiresAt    time.Time
	LastAccessed time.Time
	SessionID    string
	Username     string
	UserAgent    sql.NullString
	IPAddress    sql.NullString
	ID           int
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
		if errors.Is(err, sql.ErrNoRows) {
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
		if errors.Is(err, sql.ErrNoRows) {
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
	CreatedAt time.Time
	ExpiresAt time.Time
	ClaimedAt sql.NullTime
	Code      string
	ClaimedBy sql.NullString
	ID        int
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
		if errors.Is(err, sql.ErrNoRows) {
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
	isMySQL := r.db.DriverName() == "mysql"
	typ, defaultClause := addColumnSQLType(field, isMySQL)

	notNull := ""
	if field.Required {
		notNull = " NOT NULL"
	}

	var query string
	if isMySQL {
		query = fmt.Sprintf("ALTER TABLE %s.%s ADD COLUMN %s %s%s%s", db, t, col, typ, notNull, defaultClause)
	} else {
		query = fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s%s%s", t, col, typ, notNull, defaultClause)
	}
	_, err := r.db.ExecContext(ctx, query)
	return err
}

// addColumnSQLType returns the SQL column type and DEFAULT clause for a new field.
func addColumnSQLType(field FieldSpec, isMySQL bool) (typ, defaultClause string) {
	switch field.Type {
	case "int64", "user_ref":
		return "BIGINT", ""
	case "tinyint":
		if isMySQL {
			return "TINYINT(1)", ""
		}
		return "INTEGER", ""
	case "datetime":
		// Declare DATETIME on SQLite too so the column is reported as a datetime.
		// ISO-8601 values are not integer literals, so SQLite still stores them as text.
		return "DATETIME", datetimeDefaultClause(field.DefaultToCurrentTimestamp, isMySQL)
	default: // including "string"
		return "TEXT", ""
	}
}

func datetimeDefaultClause(defaultToNow, isMySQL bool) string {
	switch {
	case !defaultToNow:
		return ""
	case isMySQL:
		return " DEFAULT CURRENT_TIMESTAMP"
	default:
		return " DEFAULT (datetime('now'))"
	}
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

	columnNames := fieldSpecNames(columns)
	log.Debugf("ListItems columnNames: %v", columnNames)

	// If a filter value contains '%' it is a LIKE pattern for server-assisted contains search.
	whereClause, args := exportWhereClause(where)

	query := fmt.Sprintf("SELECT `%s` FROM `%s`.`%s`%s ORDER BY `%s` DESC", strings.Join(columnNames, "`, `"), tc.Db.String, tc.Table.String, whereClause, tc.SortColumnName(columnNames))
	log.Debugf("ListItems SQL Query: %s db:%v tbl:%v", query, tc.Db, tc.Table)

	items, err := r.queryItems(ctx, query, args, tc.PrimaryKeyColumnName())
	if err != nil {
		log.Errorf("Failed to list items in table %s: %v", tcName, err)
		return nil, err
	}

	log.Debugf("ListItems: %d items found", len(items))
	return items, nil
}

func fieldSpecNames(columns []FieldSpec) []string {
	names := make([]string, 0, len(columns))
	for _, col := range columns {
		names = append(names, col.Name)
	}
	return names
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

	insertColumns, placeholders, values := itemInsertColumns(columns, additionalFields, r.sqlCurrentTimestampExpr())

	query := fmt.Sprintf("INSERT INTO %s.%s (%s) VALUES (%s)", tc.Db.String, tc.Table.String, strings.Join(insertColumns, ", "), strings.Join(placeholders, ", "))
	// Log the SQL used (without parameter values)
	log.WithFields(log.Fields{"table": tc.Table.String}).Debugf("CreateItem SQL: %s", query)

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

// itemInsertColumns builds the column list, placeholders and values for inserting an item: sr_created
// and sr_updated set to nowExpr when those columns exist, plus every additional field naming a real column.
func itemInsertColumns(columns []FieldSpec, additionalFields map[string]string, nowExpr string) (insertColumns, placeholders []string, values []any) {
	for _, tsColumn := range []string{"sr_created", "sr_updated"} {
		if hasColumn(columns, tsColumn) {
			insertColumns = append(insertColumns, "`"+tsColumn+"`")
			placeholders = append(placeholders, nowExpr)
		}
	}
	for key, value := range additionalFields {
		col := writableColumn(columns, key)
		if col == nil {
			continue
		}
		insertColumns = append(insertColumns, fmt.Sprintf("`%s`", col.Name))
		placeholders = append(placeholders, "?")
		values = append(values, value)
	}
	return insertColumns, placeholders, values
}

func hasColumn(columns []FieldSpec, name string) bool {
	return slices.ContainsFunc(columns, func(c FieldSpec) bool { return c.Name == name })
}

// writableColumn returns the column a submitted field name refers to (exact match first, then
// case-insensitive), or nil for relation synthetic columns and unknown names.
func writableColumn(columns []FieldSpec, fieldName string) *FieldSpec {
	if IsRelationSyntheticColumn(fieldName) {
		return nil
	}
	if i := slices.IndexFunc(columns, func(c FieldSpec) bool { return c.Name == fieldName }); i >= 0 {
		return &columns[i]
	}
	if i := slices.IndexFunc(columns, func(c FieldSpec) bool { return strings.EqualFold(c.Name, fieldName) }); i >= 0 {
		return &columns[i]
	}
	return nil
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

	columnNames := fieldSpecNames(columns)
	query := fmt.Sprintf("SELECT `%s` FROM `%s`.`%s` ORDER BY `%s` DESC LIMIT 1", strings.Join(columnNames, "`, `"), tc.Db.String, tc.Table.String, tc.SortColumnName(columnNames))
	log.Debugf("GetLastItem SQL Query: %s db:%v tbl:%v", query, tc.Db.String, tc.Table.String)
	rows, err := r.db.QueryxContext(ctx, query)
	if err != nil {
		return Item{}, fmt.Errorf("failed to get last item for table %d: %w", tcID, err)
	}
	defer func() { _ = rows.Close() }()

	if !rows.Next() {
		return Item{}, fmt.Errorf("no items found for table %d", tcID)
	}

	rowMap := make(map[string]any)
	if err := rows.MapScan(rowMap); err != nil {
		return Item{}, fmt.Errorf("failed to scan last item for table %d: %w", tcID, err)
	}

	// Unlike scanRowToItem, every column (including the key and timestamps) stays in Fields.
	item := Item{Fields: make(map[string]any, len(rowMap))}
	for colName, value := range rowMap {
		item.Fields[colName] = bytesToString(value)
	}

	return item, rows.Err()
}

// bytesToString converts a scanned []byte column value to a string, leaving other values unchanged.
func bytesToString(value any) any {
	if valueBytes, ok := value.([]uint8); ok {
		return string(valueBytes)
	}
	return value
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

	pkColumn := tc.PrimaryKeyColumnName()
	query := fmt.Sprintf("SELECT `%s` FROM `%s`.`%s` WHERE `%s` = ?", strings.Join(columnNames, "`, `"), tc.Db.String, tc.Table.String, pkColumn)

	// Use QueryxContext to get raw row and manually map it
	rows, err := r.db.QueryxContext(ctx, query, id)
	if err != nil {
		return Item{}, err
	}
	defer func() { _ = rows.Close() }()

	if !rows.Next() {
		return Item{}, fmt.Errorf("item not found")
	}

	// Get the row as a map
	rowMap := make(map[string]any)
	if err := rows.MapScan(rowMap); err != nil {
		return Item{}, err
	}

	return scanRowToItem(rowMap, pkColumn), nil
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

	setParts, args := itemUpdateSet(columns, name, additionalFields, r.sqlCurrentTimestampExpr())

	// Ensure we have at least one field to update
	if len(setParts) == 0 {
		return Item{}, fmt.Errorf("no fields to update")
	}

	pkColumn := tc.PrimaryKeyColumnName()
	args = append(args, id)

	query := fmt.Sprintf("UPDATE `%s`.`%s` SET %s WHERE `%s` = ?", tc.Db.String, tc.Table.String, strings.Join(setParts, ", "), pkColumn)
	log.Infof("Executing update query: %s with args: %v", query, args)

	if _, err := r.db.ExecContext(ctx, query, args...); err != nil {
		log.Errorf("Failed to update item: %v", err)
		return Item{}, err
	}

	return r.GetItemInTable(ctx, tc, id)
}

// itemUpdateSet builds the SET clauses and arguments for updating an item: name when that column
// exists and a name is given, sr_updated set to nowExpr when that column exists, and every additional
// field naming a real column.
func itemUpdateSet(columns []FieldSpec, name string, additionalFields map[string]string, nowExpr string) (setParts []string, args []any) {
	if hasColumn(columns, "name") && name != "" {
		setParts = append(setParts, "`name` = ?")
		args = append(args, name)
	}
	if hasColumn(columns, "sr_updated") {
		setParts = append(setParts, fmt.Sprintf("`sr_updated` = %s", nowExpr))
	}
	for fieldName, fieldValue := range additionalFields {
		col := writableColumn(columns, fieldName)
		if col == nil {
			continue
		}
		// Sanitize field name to prevent SQL injection
		sanitizedFieldName := sanitizeDatabaseIdentifier(fieldName)
		if updateClearsToNull(col, fieldName, fieldValue) {
			setParts = append(setParts, fmt.Sprintf("`%s` = NULL", sanitizedFieldName))
		} else {
			setParts = append(setParts, fmt.Sprintf("`%s` = ?", sanitizedFieldName))
			args = append(args, fieldValue)
		}
	}
	return setParts, args
}

// updateClearsToNull reports whether an empty or "0" value should be stored as NULL instead: for
// nullable columns, and always for the navigation workflow_id/dashboard_id FKs (explicit names for
// reliable clears from the UI).
func updateClearsToNull(col *FieldSpec, fieldName, fieldValue string) bool {
	if fieldValue != "" && fieldValue != "0" {
		return false
	}
	return fieldName == "workflow_id" || fieldName == "dashboard_id" || !col.Required
}

func (r *Repository) DeleteItemInTable(ctx context.Context, table string, id string) (bool, error) {
	tc, err := r.GetTableConfiguration(ctx, table)
	if err != nil {
		return false, fmt.Errorf("failed to get table configuration for table %s: %w", table, err)
	}

	pkColumn := tc.PrimaryKeyColumnName()
	query := fmt.Sprintf("DELETE FROM `%s`.`%s` WHERE `%s` = ?", tc.Db.String, tc.Table.String, pkColumn)
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
	if host := os.Getenv("DB_HOST"); host != "" {
		return openMySQLFromEnv(host)
	}

	db, err := sqlx.Open("sqlite", defaultSQLiteDSN)
	if err != nil {
		return nil, err
	}
	if err := configureSQLiteDB(db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

func openMySQLFromEnv(host string) (*sqlx.DB, error) {
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

// sqliteMaxOpenConns allows concurrent readers under WAL. A pool of one serializes every
// HTTP handler and IAM auth lookup on the same DSN, so parallel RPCs queue until auth hits
// its 30s provider timeout.
const sqliteMaxOpenConns = 8

// configureSQLiteDB sets pool and pragmas suited to concurrent HTTP handlers and dev reload (air).
func configureSQLiteDB(db *sqlx.DB) error {
	db.SetMaxOpenConns(sqliteMaxOpenConns)
	db.SetMaxIdleConns(sqliteMaxOpenConns)
	db.SetConnMaxLifetime(0)

	pragmas := []string{
		"PRAGMA foreign_keys = ON",
		"PRAGMA journal_mode = WAL",
		"PRAGMA busy_timeout = 10000",
		"PRAGMA synchronous = NORMAL",
	}
	for _, pragma := range pragmas {
		if _, err := db.ExecContext(context.Background(), pragma); err != nil {
			return fmt.Errorf("%s: %w", pragma, err)
		}
	}
	return nil
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
			DfltValue *string `db:"dflt_value"`
			Name      string  `db:"name"`
			Type      string  `db:"type"`
			Cid       int     `db:"cid"`
			NotNull   int     `db:"notnull"`
			PK        int     `db:"pk"`
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
	query := "SELECT " + tableConfigurationSelectColumns + " FROM table_configurations WHERE id = ?"
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
	ConfigurationName sql.NullString `db:"configuration_name"`
	HasConfiguration  bool           `db:"has_configuration"`
}

// GetDatabaseTables returns all tables in the specified database with their configuration status
func (r *Repository) GetDatabaseTables(ctx context.Context, database string) ([]DatabaseTableInfo, error) {
	log.WithFields(log.Fields{
		"database": database,
	}).Infof("Getting database tables")

	if database == "" {
		database = "main"
	}

	var tables []DatabaseTableInfo
	var err error
	switch r.db.DriverName() {
	case "mysql":
		query := `
			SELECT
				t.TABLE_NAME as table_name,
				CASE WHEN tc.name IS NOT NULL THEN 1 ELSE 0 END as has_configuration,
				tc.name as configuration_name
			FROM information_schema.TABLES t
			LEFT JOIN table_configurations tc ON t.TABLE_NAME = tc.` + "`table`" + ` AND tc.` + "`db`" + ` = ?
			WHERE t.TABLE_SCHEMA = ?
			AND t.TABLE_TYPE = 'BASE TABLE'
			ORDER BY t.TABLE_NAME
		`
		err = r.db.SelectContext(ctx, &tables, query, database, database)
	default: // sqlite
		query := `
			SELECT
				m.name as table_name,
				CASE WHEN tc.name IS NOT NULL THEN 1 ELSE 0 END as has_configuration,
				tc.name as configuration_name
			FROM sqlite_master m
			LEFT JOIN table_configurations tc ON m.name = tc."table" AND tc.db = ?
			WHERE m.type = 'table'
			AND m.name NOT LIKE 'sqlite_%'
			ORDER BY m.name
		`
		err = r.db.SelectContext(ctx, &tables, query, database)
	}
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

	database = cmp.Or(database, "main")

	if table == "" {
		return fmt.Errorf("table name is required")
	}

	t := sanitizeDatabaseIdentifier(table)

	exists, err := r.physicalTableExists(ctx, database, t)
	if err != nil {
		return fmt.Errorf("failed to check if table exists: %w", err)
	}
	if exists {
		return fmt.Errorf("table '%s' already exists in database '%s'", t, database)
	}

	if _, err = r.db.ExecContext(ctx, r.createTableSQL(database, t)); err != nil {
		log.Errorf("Failed to create table: %v", err)
		return fmt.Errorf("failed to create table: %w", err)
	}

	log.Infof("Created physical table: %s.%s", database, t)
	return nil
}

func (r *Repository) physicalTableExists(ctx context.Context, database, table string) (bool, error) {
	var exists int
	var err error
	if r.db.DriverName() == "mysql" {
		checkQuery := "SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?"
		err = r.db.GetContext(ctx, &exists, checkQuery, database, table)
	} else {
		checkQuery := "SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?"
		err = r.db.GetContext(ctx, &exists, checkQuery, table)
	}
	return exists > 0, err
}

// createTableSQL returns the DDL for a new table with id, sr_created and sr_updated columns.
func (r *Repository) createTableSQL(database, table string) string {
	if r.db.DriverName() == "mysql" {
		return fmt.Sprintf(
			"CREATE TABLE %s.%s (id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY, sr_created DATETIME DEFAULT CURRENT_TIMESTAMP, sr_updated DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP)",
			database, table)
	}
	// SQLite - doesn't support database qualifiers in CREATE TABLE
	return fmt.Sprintf(
		"CREATE TABLE %s (id INTEGER PRIMARY KEY AUTOINCREMENT, sr_created TEXT DEFAULT (datetime('now')), sr_updated TEXT DEFAULT (datetime('now')))",
		table)
}

// CreateTableConfiguration creates a new table configuration entry.
// When createNavigation is true, a row is added to table_navigation for the new configuration.
func (r *Repository) CreateTableConfiguration(ctx context.Context, name, database, table string, createNavigation bool, navigationAppID int) error {
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

	if createNavigation {
		r.createTableConfigurationNavigation(ctx, configID, name, navigationAppID)
	}

	log.Infof("Created table configuration: %s (db: %s, table: %s, view: table)", name, database, table)
	return nil
}

// createTableConfigurationNavigation adds a sidebar link for a new table configuration, assigned to
// the sidebar app when navigationAppID is set. Failures are logged, not returned.
func (r *Repository) createTableConfigurationNavigation(ctx context.Context, configID int64, name string, navigationAppID int) {
	navQuery := "INSERT INTO table_navigation (ordinal, table_configuration, name) VALUES (99, ?, ?)"
	navResult, err := r.db.ExecContext(ctx, navQuery, configID, name)
	if err != nil {
		log.Warnf("Failed to create navigation entry for table configuration %s: %v", name, err)
		return
	}
	log.Infof("Created navigation entry for table configuration: %s", name)
	if navigationAppID <= 0 {
		return
	}
	navID, err := navResult.LastInsertId()
	if err != nil {
		log.Warnf("Failed to get navigation item id for app assignment: %v", err)
		return
	}
	if err := r.AssignNavigationItemToApp(ctx, navigationAppID, int(navID), 99); err != nil {
		log.Warnf("Failed to assign navigation item to app %d: %v", navigationAppID, err)
	}
}

// systemTableConfig is a built-in table configuration upserted at startup.
type systemTableConfig struct {
	name             string
	title            string
	table            string
	createDelegate   string
	createButtonText string
	rowName          string
	ordinal          int
}

var systemTableConfigs = []systemTableConfig{
	{name: "table_settings", title: "Settings", table: "table_settings", ordinal: 0, createButtonText: "Add Setting", rowName: "Setting"},
	{name: "table_configurations", title: "Table Configurations", table: "table_configurations", ordinal: 1, createDelegate: "/admin/table/create", createButtonText: "Create Table", rowName: "Table configuration"},
	{name: "table_workflows", title: "Workflows", table: "table_workflows", ordinal: 2, createButtonText: "Add Workflow", rowName: "Workflow"},
	{name: "table_navigation", title: "Navigation", table: "table_navigation", ordinal: 3, createButtonText: "Add Navigation Item", rowName: "Navigation item"},
	{name: "table_apps", title: "Sidebar apps", table: "apps", ordinal: 9, createButtonText: "Add app", rowName: "App"},
	{name: "table_app_navigation_links", title: "App navigation links", table: "app_navigation_links", ordinal: 10, createButtonText: "Add link", rowName: "App navigation link"},
	{name: "table_dashboards", title: "Dashboards", table: "table_dashboards", ordinal: 4, createDelegate: "/admin/dashboard/create", createButtonText: "Create Dashboard", rowName: "Dashboard"},
	{name: "table_dashboard_components", title: "Dashboard components", table: "table_dashboard_components", ordinal: 8, createButtonText: "Add Component", rowName: "Dashboard component"},
	{name: "table_read_only_exports", title: "Read-only calendar exports", table: "read_only_calendar_exports", ordinal: 5, createButtonText: "Add Export", rowName: "Calendar export"},
	{name: "table_rss_calendar_feeds", title: "Calendar feeds", table: "rss_calendar_feeds", ordinal: 7, createDelegate: "/admin/rss-calendar-feeds/new", createButtonText: "Add feed", rowName: "Calendar feed"},
	{name: "table_logs", title: "Audit logs", table: "audit_logs", ordinal: 6, createButtonText: "Insert Row", rowName: "Audit log entry"},
}

const upsertSystemTableConfigMySQL = `INSERT INTO table_configurations (name, title, ` + "`db`" + `, ` + "`table`" + `, ordinal, create_delegate, create_button_text, row_name)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE title = VALUES(title), ` + "`db`" + ` = VALUES(` + "`db`" + `), ` + "`table`" + ` = VALUES(` + "`table`" + `), ordinal = VALUES(ordinal), create_delegate = COALESCE(NULLIF(VALUES(create_delegate), ''), create_delegate), create_button_text = COALESCE(NULLIF(VALUES(create_button_text), ''), create_button_text), row_name = COALESCE(NULLIF(VALUES(row_name), ''), row_name)`

const upsertSystemTableConfigSQLite = `INSERT INTO table_configurations (name, title, db, "table", ordinal, create_delegate, create_button_text, row_name)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(name) DO UPDATE SET title = excluded.title, db = excluded.db, "table" = excluded."table", ordinal = excluded.ordinal, create_delegate = COALESCE(NULLIF(excluded.create_delegate, ''), table_configurations.create_delegate), create_button_text = COALESCE(NULLIF(excluded.create_button_text, ''), table_configurations.create_button_text), row_name = COALESCE(NULLIF(excluded.row_name, ''), table_configurations.row_name)`

// UpdateSystemTableConfigurations upserts system table configurations (systemTableConfigs) using the
// database name from DB_NAME env (or "main" if unset). Call after migrations so system configs always
// use the correct database.
func (r *Repository) UpdateSystemTableConfigurations(ctx context.Context) error {
	dbName := cmp.Or(strings.TrimSpace(os.Getenv("DB_NAME")), "main")
	query := upsertSystemTableConfigSQLite
	if r.db.DriverName() == "mysql" {
		query = upsertSystemTableConfigMySQL
	}
	for _, c := range systemTableConfigs {
		_, err := r.db.ExecContext(ctx, query, c.name, c.title, dbName, c.table, c.ordinal, nilIfEmpty(c.createDelegate), nilIfEmpty(c.createButtonText), nilIfEmpty(c.rowName))
		if err != nil {
			return fmt.Errorf("upsert system table config %s: %w", c.name, err)
		}
		log.Infof("Updated system table config: %s (db: %s)", c.name, dbName)
	}
	return nil
}

// nilIfEmpty returns nil (SQL NULL) for an empty string, otherwise the string.
func nilIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// GetTableConfiguration returns the structure information for a table
func (r *Repository) GetTableConfiguration(ctx context.Context, tcName string) (*TableConfig, error) {
	log.WithFields(log.Fields{
		"tcName": tcName,
	}).Debugf("Getting TableConfiguration")

	// Query table_configurations for this table's metadata
	var config TableConfig
	query := "SELECT " + tableConfigurationSelectColumns + " FROM table_configurations WHERE name = ?"
	err := r.db.GetContext(ctx, &config, query, tcName)

	if err != nil {
		return nil, tableConfigurationLookupError(tcName, err)
	}

	log.Debugf("TableConfiguration: %+v", config)

	if !config.Table.Valid || !config.Db.Valid {
		return nil, fmt.Errorf("table structure is invalid, missing table or db: %+v", config)
	}

	return &config, nil
}

func tableConfigurationLookupError(tcName string, err error) error {
	log.Errorf("Failed to get table configuration for table %s: %v", tcName, err)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("table not found in configurations")
	}
	return err
}

// TableViewColumn represents a column configuration in a table view
type TableViewColumn struct {
	ColumnName  string `db:"column_name"`
	SortOrder   string `db:"sort_order"`
	ColumnOrder int    `db:"column_order"`
	IsVisible   bool   `db:"is_visible"`
}

// TableView represents a saved table view
type TableView struct {
	TableName  string            `db:"table_name"`
	ViewName   string            `db:"view_name"`
	ViewType   string            `db:"view_type"`
	LayoutJSON string            `db:"layout_json"`
	Columns    []TableViewColumn `db:"-"`
	ID         int               `db:"id"`
	IsDefault  bool              `db:"is_default"`
}

// CreateTableView creates a new table view with its column configurations and returns its ID.
func (r *Repository) CreateTableView(ctx context.Context, tableName, viewName, viewType, layoutJSON string, columns []TableViewColumn) (int, error) {
	t := sanitizeDatabaseIdentifier(tableName)

	// Default to "table" if viewType is empty
	if viewType == "" {
		viewType = "table"
	}

	// Start a transaction
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()

	// Insert the table view
	result, err := tx.ExecContext(ctx,
		"INSERT INTO table_views (table_name, view_name, is_default, view_type, layout_json) VALUES (?, ?, ?, ?, ?)",
		t, viewName, false, viewType, nullIfEmpty(layoutJSON))
	if err != nil {
		return 0, err
	}
	viewID, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}

	if err := insertTableViewColumns(ctx, tx, viewID, columns); err != nil {
		return 0, err
	}

	return int(viewID), tx.Commit()
}

// insertTableViewColumns stores a view's column configurations (only visible columns are saved).
func insertTableViewColumns(ctx context.Context, tx *sqlx.Tx, viewID int64, columns []TableViewColumn) error {
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
	return nil
}

func nullIfEmpty(value string) sql.NullString {
	return sql.NullString{String: value, Valid: value != ""}
}

// UpdateTableView updates an existing table view with its column configurations
func (r *Repository) UpdateTableView(ctx context.Context, viewID int, tableName, viewName, viewType, layoutJSON string, columns []TableViewColumn) error {
	t := sanitizeDatabaseIdentifier(tableName)

	// Default to "table" if viewType is empty
	viewType = cmp.Or(viewType, "table")

	return r.inTx(ctx, func(tx *sqlx.Tx) error {
		result, err := tx.ExecContext(ctx,
			"UPDATE table_views SET view_name = ?, view_type = ?, layout_json = ? WHERE id = ? AND table_name = ?",
			viewName, viewType, nullIfEmpty(layoutJSON), viewID, t)
		if err != nil {
			return err
		}
		if affected, rowsErr := result.RowsAffected(); rowsErr == nil && affected == 0 {
			return fmt.Errorf("view with ID %d not found for table %s", viewID, t)
		}

		// Replace the column configurations
		if _, err = tx.ExecContext(ctx, "DELETE FROM table_view_columns WHERE view_id = ?", viewID); err != nil {
			return err
		}
		return insertTableViewColumns(ctx, tx, int64(viewID), columns)
	})
}

// GetTableViews retrieves all views for a given table
func (r *Repository) GetTableViews(ctx context.Context, tableName string) ([]TableView, error) {
	t := sanitizeDatabaseIdentifier(tableName)

	// Get all views for the table
	rows, err := r.db.QueryxContext(ctx,
		"SELECT id, table_name, view_name, is_default, COALESCE(view_type, 'table') as view_type, COALESCE(layout_json, '') as layout_json FROM table_views WHERE table_name = ? ORDER BY view_name",
		t)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var views []TableView
	for rows.Next() {
		var view TableView
		err := rows.Scan(&view.ID, &view.TableName, &view.ViewName, &view.IsDefault, &view.ViewType, &view.LayoutJSON)
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
				_ = columnRows.Close()
				return nil, err
			}
			columns = append(columns, col)
		}
		_ = columnRows.Close()
		view.Columns = columns
		views = append(views, view)
	}

	return views, rows.Err()
}

// GetTableViewTableName returns the table a view belongs to.
func (r *Repository) GetTableViewTableName(ctx context.Context, viewID int) (string, error) {
	var tableName string
	err := r.db.GetContext(ctx, &tableName, "SELECT table_name FROM table_views WHERE id = ?", viewID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("view with ID %d not found", viewID)
	}
	return tableName, err
}

// GetUserDefaultTableView returns the user's own default view for a table, or 0 when unset.
func (r *Repository) GetUserDefaultTableView(ctx context.Context, userID int, tableName string) (int, error) {
	var viewID int
	err := r.db.GetContext(ctx, &viewID,
		"SELECT view_id FROM table_view_user_defaults WHERE user_id = ? AND table_name = ?",
		userID, sanitizeDatabaseIdentifier(tableName))
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return viewID, err
}

// SetUserDefaultTableView sets the user's own default view for a table; viewID 0 clears it.
func (r *Repository) SetUserDefaultTableView(ctx context.Context, userID int, tableName string, viewID int) error {
	t := sanitizeDatabaseIdentifier(tableName)
	if viewID <= 0 {
		_, err := r.db.ExecContext(ctx,
			"DELETE FROM table_view_user_defaults WHERE user_id = ? AND table_name = ?", userID, t)
		return err
	}

	if err := r.validateUserDefaultTableView(ctx, t, viewID); err != nil {
		return err
	}
	return r.upsertUserDefaultTableView(ctx, userID, t, viewID)
}

func (r *Repository) validateUserDefaultTableView(ctx context.Context, tableName string, viewID int) error {
	viewTable, err := r.GetTableViewTableName(ctx, viewID)
	if err != nil {
		return err
	}
	if viewTable != tableName {
		return fmt.Errorf("view %d does not belong to table %s", viewID, tableName)
	}
	return nil
}

func (r *Repository) upsertUserDefaultTableView(ctx context.Context, userID int, tableName string, viewID int) error {
	if r.db.DriverName() == "mysql" {
		_, err := r.db.ExecContext(ctx, `
			INSERT INTO table_view_user_defaults (user_id, table_name, view_id) VALUES (?, ?, ?)
			ON DUPLICATE KEY UPDATE view_id = VALUES(view_id)`, userID, tableName, viewID)
		return err
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO table_view_user_defaults (user_id, table_name, view_id) VALUES (?, ?, ?)
		ON CONFLICT(user_id, table_name) DO UPDATE SET view_id = excluded.view_id`, userID, tableName, viewID)
	return err
}

// DeleteTableView deletes a table view and its associated columns
func (r *Repository) DeleteTableView(ctx context.Context, viewID int) error {
	// Start a transaction to ensure atomicity
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	// Delete the view columns first (due to foreign key constraint)
	_, err = tx.ExecContext(ctx, "DELETE FROM table_view_columns WHERE view_id = ?", viewID)
	if err != nil {
		return err
	}

	// Users who chose this view as their default fall back to the table default.
	_, err = tx.ExecContext(ctx, "DELETE FROM table_view_user_defaults WHERE view_id = ?", viewID)
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
	ReferencedTable   string                     `db:"referenced_table"`
	DisplayName       string                     `db:"display_name"`
	TableSchema       string                     `db:"table_schema"`
	TableName         string                     `db:"table_name"`
	ColumnName        string                     `db:"column_name"`
	ReferencedSchema  string                     `db:"referenced_schema"`
	OnDeleteAction    string                     `db:"on_delete_action"`
	AbsorbType        string                     `db:"-"`
	ConstraintName    string                     `db:"constraint_name"`
	OnUpdateAction    string                     `db:"on_update_action"`
	ReferencedColumn  string                     `db:"referenced_column"`
	PullThroughFields []RelationPullThroughField `db:"-"`
	RelationID        int                        `db:"-"`
	AbsorbIntoItem    bool                       `db:"-"`
}

// CreateForeignKey records a relation in table_relations and applies a database FK when supported.
func (r *Repository) CreateForeignKey(ctx context.Context, tableName, columnName, referencedTable, referencedColumn, onDeleteAction, onUpdateAction string) error {
	col := sanitizeDatabaseIdentifier(columnName)
	refCol := sanitizeDatabaseIdentifier(referencedColumn)

	tc, tcRef, err := r.foreignKeyTableConfigs(ctx, tableName, referencedTable)
	if err != nil {
		return err
	}

	constraintName := makeForeignKeyConstraintName(tableName, col, referencedTable, refCol)
	rel := TableRelation{
		ConstraintName:     constraintName,
		SourceTableKey:     tableName,
		SourceColumn:       col,
		ReferencedTableKey: referencedTable,
		ReferencedColumn:   refCol,
		OnDeleteAction:     onDeleteAction,
		OnUpdateAction:     onUpdateAction,
	}
	if err := r.insertTableRelation(ctx, rel); err != nil {
		return fmt.Errorf("save table relation: %w", err)
	}

	if err := r.applyPhysicalForeignKey(ctx, tc, tcRef, col, refCol, onDeleteAction, onUpdateAction, constraintName); err != nil {
		log.WithError(err).Warn("database foreign key not applied; relation saved in table_relations")
	}
	return nil
}

func (r *Repository) foreignKeyTableConfigs(ctx context.Context, tableName, referencedTable string) (*TableConfig, *TableConfig, error) {
	tc, err := r.GetTableConfiguration(ctx, tableName)
	if err != nil {
		return nil, nil, err
	}
	tcRef, err := r.GetTableConfiguration(ctx, referencedTable)
	if err != nil {
		return nil, nil, err
	}
	return tc, tcRef, nil
}

// GetForeignKeys lists relations involving a table configuration (source or referenced).
func (r *Repository) GetForeignKeys(ctx context.Context, tableName string) ([]ForeignKey, error) {
	if _, err := r.GetTableConfiguration(ctx, tableName); err != nil {
		return nil, err
	}
	relations, err := r.listTableRelationsForConfiguration(ctx, tableName)
	if err != nil {
		return nil, err
	}
	return r.tableRelationsToForeignKeys(ctx, relations)
}

func (r *Repository) getForeignKeysForTableConfig(ctx context.Context, tableName string, tc *TableConfig) ([]ForeignKey, error) {
	if tc == nil {
		return nil, fmt.Errorf("table configuration is required")
	}
	relations, err := r.listTableRelationsForConfiguration(ctx, tableName)
	if err != nil {
		return nil, err
	}
	return r.tableRelationsToForeignKeys(ctx, relations)
}

// GetForeignKeysForTableConfig is like GetForeignKeys but skips reloading table_configurations.
func (r *Repository) GetForeignKeysForTableConfig(ctx context.Context, tc *TableConfig) ([]ForeignKey, error) {
	if tc == nil {
		return nil, fmt.Errorf("table configuration is required")
	}
	return r.getForeignKeysForTableConfig(ctx, tc.Name, tc)
}

// DeleteForeignKey removes a relation from table_relations and drops the database FK when possible.
func (r *Repository) DeleteForeignKey(ctx context.Context, constraintName string) error {
	rel, err := r.getTableRelationByConstraintName(ctx, constraintName)
	if err != nil {
		return fmt.Errorf("table relation not found: %w", err)
	}
	if err := r.deleteTableRelationByConstraintName(ctx, constraintName); err != nil {
		return err
	}
	if err := r.dropPhysicalForeignKey(ctx, rel); err != nil {
		log.WithError(err).Warn("database foreign key not removed; relation deleted from table_relations")
	}
	return nil
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

	// SQLite cannot rewrite a column's declared type. Datetime is stored as text;
	// record the logical type so structure APIs report datetime instead of TEXT.
	if r.db.DriverName() == "sqlite" {
		if IsDatetimeTypeName(newType) {
			return r.SetColumnSemantic(ctx, tc.Name, columnName, SemanticTypeDatetime)
		}
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
	if err != nil {
		return err
	}
	return r.DeleteColumnDisplay(ctx, tableName, columnName)
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
	if err != nil {
		return err
	}
	return r.RenameColumnDisplay(ctx, tableName, oldColumnName, newColumnName)
}

// InsertRecentlyViewed adds a table and item ID to the recently viewed tracking table for one user.
func (r *Repository) InsertRecentlyViewed(ctx context.Context, userID int, tableName, itemID string) error {
	if userID <= 0 || tableName == "" || itemID == "" {
		return nil
	}
	isMySQL := r.db.DriverName() == "mysql"

	// First, try to update existing record if it exists
	result, err := r.db.ExecContext(ctx, recentlyViewedUpdateSQL(isMySQL), userID, tableName, itemID)
	if err != nil {
		return err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil || rowsAffected > 0 {
		return err
	}

	// No existing record, so insert one
	_, err = r.db.ExecContext(ctx, recentlyViewedInsertSQL(isMySQL), userID, tableName, itemID)
	return err
}

func recentlyViewedUpdateSQL(isMySQL bool) string {
	if isMySQL {
		return `UPDATE table_recently_viewed
			SET updated_at_unix = UNIX_TIMESTAMP()
			WHERE user_account_id = ? AND name = ? AND table_id = ?`
	}
	return `UPDATE table_recently_viewed
			SET updated_at_unix = strftime('%s', 'now')
			WHERE user_account_id = ? AND name = ? AND table_id = ?`
}

func recentlyViewedInsertSQL(isMySQL bool) string {
	if isMySQL {
		return `INSERT INTO table_recently_viewed (user_account_id, name, table_id, sr_created, updated_at_unix)
				VALUES (?, ?, ?, NOW(), UNIX_TIMESTAMP())`
	}
	return `INSERT INTO table_recently_viewed (user_account_id, name, table_id, sr_created, updated_at_unix)
				VALUES (?, ?, ?, datetime('now'), strftime('%s', 'now'))`
}

// RecentlyViewedItem represents a recently viewed item with table configuration
type RecentlyViewedItem struct {
	Name          string `db:"name"`
	TableID       string `db:"table_id"`
	Icon          string `db:"icon"`
	ItemName      string
	TableTitle    string `db:"title"`
	UpdatedAtUnix int64  `db:"updated_at_unix"`
}

// GetMostRecentlyViewed returns the most recently viewed items for a user with table configuration details.
func (r *Repository) GetMostRecentlyViewed(ctx context.Context, userID, limit int) ([]RecentlyViewedItem, error) {
	if userID <= 0 {
		return nil, nil
	}
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
		WHERE rv.user_account_id = ?
		ORDER BY rv.updated_at_unix DESC
		LIMIT ?
	`

	var items []RecentlyViewedItem
	err := r.db.SelectContext(ctx, &items, query, userID, limit)
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
func (r *Repository) getItemName(ctx context.Context, tcName, itemID string) (string, error) {
	tc, err := r.GetTableConfiguration(ctx, tcName)
	if err != nil {
		return itemID, err
	}
	pkColumn := tc.PrimaryKeyColumnName()

	var name string
	err = r.db.GetContext(ctx, &name,
		fmt.Sprintf("SELECT `name` FROM `%s`.`%s` WHERE `%s` = ?", tc.Db.String, tc.Table.String, pkColumn),
		itemID)
	if err != nil {
		err = r.db.GetContext(ctx, &name,
			fmt.Sprintf("SELECT `title` FROM `%s`.`%s` WHERE `%s` = ?", tc.Db.String, tc.Table.String, pkColumn),
			itemID)
		if err != nil {
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
	NavigationItem   *NavigationItem
	Title            sql.NullString
	ID               int
	UserID           int
	NavigationItemID int
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
			COALESCE(tc.title, tc.name) as table_title,
			tc.icon as icon,
			tn.dashboard_id as dashboard_id,
			td.name as dashboard_name,
			tn.name as navigation,
			tn.title as link_title,
			tn.workflow_id as workflow_id,
			tw.name as workflow_name,
			tn.name as title
		FROM table_user_bookmarks ub
		INNER JOIN table_navigation tn ON ub.navigation_item = tn.id
		LEFT JOIN table_configurations tc ON tn.table_configuration = tc.id
		LEFT JOIN table_dashboards td ON tn.dashboard_id = td.id
		LEFT JOIN table_workflows tw ON tn.workflow_id = tw.id
		WHERE ub.user = ?
		ORDER BY ub.id DESC
	`

	rows, err := r.db.QueryxContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

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
			&navItem.TableTitle,
			&navItem.Icon,
			&navItem.DashboardID,
			&navItem.DashboardName,
			&navItem.Navigation,
			&navItem.TitleOverride,
			&navItem.WorkflowID,
			&navItem.WorkflowName,
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

	return r.userBookmarkByID(ctx, userID, int(bookmarkID))
}

// userBookmarkByID returns one of the user's bookmarks with its navigation item details.
func (r *Repository) userBookmarkByID(ctx context.Context, userID, bookmarkID int) (*UserBookmark, error) {
	bookmarks, err := r.GetUserBookmarks(ctx, userID)
	if err != nil {
		return nil, err
	}
	if i := slices.IndexFunc(bookmarks, func(b UserBookmark) bool { return b.ID == bookmarkID }); i >= 0 {
		return &bookmarks[i], nil
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

// Tick list state (shared across clients)

// GetTickListState returns the completion state for a table's tick list (item_id -> completed).
func (r *Repository) GetTickListState(ctx context.Context, tcName string) (map[string]bool, error) {
	query := `SELECT item_id, completed FROM tick_list_state WHERE tc_name = ?`
	rows, err := r.db.QueryxContext(ctx, query, tcName)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := make(map[string]bool)
	for rows.Next() {
		var itemID string
		var completed int
		if err := rows.Scan(&itemID, &completed); err != nil {
			return nil, err
		}
		out[itemID] = completed != 0
	}
	return out, rows.Err()
}

// SetTickListCompletion sets or updates one item's completed state for a table's tick list.
func (r *Repository) SetTickListCompletion(ctx context.Context, tcName, itemID string, completed bool) error {
	completedInt := 0
	if completed {
		completedInt = 1
	}
	// SQLite and MySQL support INSERT ... ON DUPLICATE KEY UPDATE / INSERT OR REPLACE
	if r.db.DriverName() == "mysql" {
		_, err := r.db.ExecContext(ctx,
			`INSERT INTO tick_list_state (tc_name, item_id, completed) VALUES (?, ?, ?)
			 ON DUPLICATE KEY UPDATE completed = VALUES(completed)`,
			tcName, itemID, completedInt)
		return err
	}
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO tick_list_state (tc_name, item_id, completed) VALUES (?, ?, ?)
		 ON CONFLICT(tc_name, item_id) DO UPDATE SET completed = excluded.completed`,
		tcName, itemID, completedInt)
	return err
}

// ClearTickListState removes all tick list completion state for a table.
func (r *Repository) ClearTickListState(ctx context.Context, tcName string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM tick_list_state WHERE tc_name = ?`, tcName)
	return err
}

// API Key related methods

type APIKey struct {
	CreatedAt  time.Time
	LastUsedAt *time.Time
	ExpiresAt  *time.Time
	Name       string
	KeyHash    string
	ID         int
	UserID     int
	IsActive   bool
	ReadOnly   bool
}

type ConditionalFormattingRule struct {
	SrCreated      time.Time
	TableName      string
	ColumnName     string
	ConditionType  string
	ConditionValue string
	FormatType     string
	FormatValue    string
	ID             int
	Priority       int
	UpdatedAtUnix  int64
	IsActive       bool
}

// CreateAPIKey creates a new API key for a user
func (r *Repository) CreateAPIKey(ctx context.Context, userID int, name, keyHash string, expiresAt *time.Time, readOnly bool) (*APIKey, error) {
	query := `
		INSERT INTO table_api_keys (user_id, name, key_hash, expires_at, read_only)
		VALUES (?, ?, ?, ?, ?)
	`
	readOnlyInt := 0
	if readOnly {
		readOnlyInt = 1
	}
	result, err := r.db.ExecContext(ctx, query, userID, name, keyHash, expiresAt, readOnlyInt)
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
		SELECT id, user_id, name, key_hash, created_at, last_used_at, expires_at, is_active, read_only
		FROM table_api_keys
		WHERE id = ?
	`

	var apiKey APIKey
	var readOnlyInt int
	err := r.db.QueryRowxContext(ctx, query, id).Scan(
		&apiKey.ID,
		&apiKey.UserID,
		&apiKey.Name,
		&apiKey.KeyHash,
		&apiKey.CreatedAt,
		&apiKey.LastUsedAt,
		&apiKey.ExpiresAt,
		&apiKey.IsActive,
		&readOnlyInt,
	)
	apiKey.ReadOnly = readOnlyInt != 0
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	return &apiKey, nil
}

// GetAPIKeyByHash retrieves an API key by its hash
func (r *Repository) GetAPIKeyByHash(ctx context.Context, keyHash string) (*APIKey, error) {
	query := `
		SELECT id, user_id, name, key_hash, created_at, last_used_at, expires_at, is_active, read_only
		FROM table_api_keys
		WHERE key_hash = ? AND is_active = 1
	`

	var apiKey APIKey
	var readOnlyInt int
	err := r.db.QueryRowxContext(ctx, query, keyHash).Scan(
		&apiKey.ID,
		&apiKey.UserID,
		&apiKey.Name,
		&apiKey.KeyHash,
		&apiKey.CreatedAt,
		&apiKey.LastUsedAt,
		&apiKey.ExpiresAt,
		&apiKey.IsActive,
		&readOnlyInt,
	)
	apiKey.ReadOnly = readOnlyInt != 0
	if err != nil {
		return nil, apiKeyLookupError(err)
	}
	if apiKeyExpired(apiKey) {
		return nil, nil
	}

	return &apiKey, nil
}

func apiKeyLookupError(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	return err
}

func apiKeyExpired(apiKey APIKey) bool {
	return apiKey.ExpiresAt != nil && apiKey.ExpiresAt.Before(time.Now())
}

// GetUserAPIKeys retrieves all API keys for a user
func (r *Repository) GetUserAPIKeys(ctx context.Context, userID int) ([]APIKey, error) {
	query := `
		SELECT id, user_id, name, key_hash, created_at, last_used_at, expires_at, is_active, read_only
		FROM table_api_keys
		WHERE user_id = ?
		ORDER BY created_at DESC
	`

	rows, err := r.db.QueryxContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var apiKeys []APIKey
	for rows.Next() {
		var apiKey APIKey
		var readOnlyInt int
		err := rows.Scan(
			&apiKey.ID,
			&apiKey.UserID,
			&apiKey.Name,
			&apiKey.KeyHash,
			&apiKey.CreatedAt,
			&apiKey.LastUsedAt,
			&apiKey.ExpiresAt,
			&apiKey.IsActive,
			&readOnlyInt,
		)
		apiKey.ReadOnly = readOnlyInt != 0
		if err != nil {
			return nil, err
		}
		apiKeys = append(apiKeys, apiKey)
	}

	return apiKeys, nil
}

// UpdateAPIKey updates an API key's name, expires_at, and/or read_only. Pass nil for fields to leave unchanged.
func (r *Repository) UpdateAPIKey(ctx context.Context, userID, apiKeyID int, name *string, expiresAt *time.Time, readOnly *bool) error {
	updates, args := apiKeyUpdateSet(name, expiresAt, readOnly)
	if len(updates) == 0 {
		return nil
	}
	args = append(args, apiKeyID, userID)
	query := "UPDATE table_api_keys SET " + strings.Join(updates, ", ") + " WHERE id = ? AND user_id = ?"
	result, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("API key not found or not owned by user")
	}
	return nil
}

// apiKeyUpdateSet builds SET clauses for the API key fields that are being changed (non-nil).
func apiKeyUpdateSet(name *string, expiresAt *time.Time, readOnly *bool) (updates []string, args []any) {
	if name != nil {
		updates = append(updates, "name = ?")
		args = append(args, *name)
	}
	if expiresAt != nil {
		updates = append(updates, "expires_at = ?")
		args = append(args, *expiresAt)
	}
	if readOnly != nil {
		updates = append(updates, "read_only = ?")
		args = append(args, boolToInt(*readOnly))
	}
	return updates, args
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
	hasSrCreated := r.conditionalFormattingHasSrCreated(ctx)
	query, args := conditionalFormattingRulesQuery(hasSrCreated, tableName)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var rules []*ConditionalFormattingRule
	for rows.Next() {
		rule, err := scanConditionalFormattingRule(rows, hasSrCreated)
		if err != nil {
			return nil, err
		}
		rules = append(rules, rule)
	}

	return rules, nil
}

// conditionalFormattingHasSrCreated reports whether table_conditional_formatting_rules has the
// sr_created column (absent on older schema versions).
func (r *Repository) conditionalFormattingHasSrCreated(ctx context.Context) bool {
	if r.db.DriverName() == "mysql" {
		var count int
		checkQuery := `SELECT COUNT(*) FROM information_schema.COLUMNS
			WHERE TABLE_SCHEMA = DATABASE()
			AND TABLE_NAME = 'table_conditional_formatting_rules'
			AND COLUMN_NAME = 'sr_created'`
		err := r.db.GetContext(ctx, &count, checkQuery)
		return err == nil && count > 0
	}

	// SQLite - check using PRAGMA
	type colInfo struct {
		Name string `db:"name"`
		Type string `db:"type"`
		Cid  int    `db:"cid"`
	}
	var cols []colInfo
	if err := r.db.SelectContext(ctx, &cols, "PRAGMA table_info(table_conditional_formatting_rules)"); err != nil {
		return false
	}
	return slices.ContainsFunc(cols, func(c colInfo) bool { return c.Name == "sr_created" })
}

// conditionalFormattingRulesQuery returns the rules query for one table, or for all tables when
// tableName is empty, selecting sr_created only when the column exists.
func conditionalFormattingRulesQuery(hasSrCreated bool, tableName string) (string, []any) {
	columns := "id, table_name, column_name, condition_type, condition_value, format_type, format_value, priority, is_active, "
	if hasSrCreated {
		columns += "sr_created, "
	}
	columns += "updated_at_unix"

	if tableName != "" {
		return "SELECT " + columns + " FROM table_conditional_formatting_rules WHERE table_name = ? ORDER BY priority ASC, id ASC", []any{tableName}
	}
	return "SELECT " + columns + " FROM table_conditional_formatting_rules ORDER BY table_name ASC, priority ASC, id ASC", []any{}
}

func scanConditionalFormattingRule(rows *sql.Rows, hasSrCreated bool) (*ConditionalFormattingRule, error) {
	var rule ConditionalFormattingRule
	var srCreatedStr string
	dest := []any{
		&rule.ID,
		&rule.TableName,
		&rule.ColumnName,
		&rule.ConditionType,
		&rule.ConditionValue,
		&rule.FormatType,
		&rule.FormatValue,
		&rule.Priority,
		&rule.IsActive,
	}
	if hasSrCreated {
		dest = append(dest, &srCreatedStr)
	}
	dest = append(dest, &rule.UpdatedAtUnix)
	if err := rows.Scan(dest...); err != nil {
		return nil, err
	}
	// sr_created remains zero time when the column is absent or empty
	if srCreatedStr != "" {
		rule.SrCreated = parseRuleTimestamp(srCreatedStr)
	}
	return &rule, nil
}

// ruleTimestampLayouts are tried in order: RFC3339 (SQLite), MySQL datetime, ISO 8601 without timezone.
var ruleTimestampLayouts = []string{time.RFC3339, "2006-01-02 15:04:05", "2006-01-02T15:04:05"}

// parseRuleTimestamp parses a stored sr_created value, logging and returning zero time when no layout matches.
func parseRuleTimestamp(value string) time.Time {
	var err error
	for _, layout := range ruleTimestampLayouts {
		var t time.Time
		if t, err = time.Parse(layout, value); err == nil {
			return t
		}
	}
	// Don't fail if parsing fails, just log and continue with zero time
	log.Warnf("Failed to parse sr_created timestamp '%s': %v", value, err)
	return time.Time{}
}

// CreateConditionalFormattingRule creates a new conditional formatting rule
func (r *Repository) CreateConditionalFormattingRule(ctx context.Context, userID int, rule *ConditionalFormattingRule) (int, error) {
	query := fmt.Sprintf(`
		INSERT INTO table_conditional_formatting_rules
		(table_name, column_name, condition_type, condition_value, format_type, format_value, priority, is_active, sr_created, updated_at_unix)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, %s, %s)`,
		r.sqlCurrentTimestampExpr(), r.sqlUnixTimestampExpr())

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

// Notification System Types and Methods

type NotificationEvent struct {
	SrCreated   time.Time
	EventCode   string
	EventName   string
	Description string
	ID          int
}

type UserNotificationChannel struct {
	SrCreated    time.Time
	SrUpdated    time.Time
	ChannelName  *string
	ChannelType  string
	ChannelValue string
	ID           int
	User         int
	IsActive     bool
}

type UserNotificationSubscription struct {
	SrCreated time.Time
	ID        int
	User      int
	EventID   int
	ChannelID int
}

// GetNotificationEvents retrieves all available notification events
func (r *Repository) GetNotificationEvents(ctx context.Context) ([]NotificationEvent, error) {
	query := `
		SELECT id, event_code, event_name, description, sr_created
		FROM notification_events
		ORDER BY event_code ASC
	`

	rows, err := r.db.QueryxContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var events []NotificationEvent
	for rows.Next() {
		var event NotificationEvent
		err := rows.Scan(
			&event.ID,
			&event.EventCode,
			&event.EventName,
			&event.Description,
			&event.SrCreated,
		)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}

	return events, nil
}

// GetNotificationEventByCode retrieves a notification event by its code
func (r *Repository) GetNotificationEventByCode(ctx context.Context, eventCode string) (*NotificationEvent, error) {
	query := `
		SELECT id, event_code, event_name, description, sr_created
		FROM notification_events
		WHERE event_code = ?
	`

	var event NotificationEvent
	err := r.db.QueryRowxContext(ctx, query, eventCode).Scan(
		&event.ID,
		&event.EventCode,
		&event.EventName,
		&event.Description,
		&event.SrCreated,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	return &event, nil
}

// GetUserNotificationChannels retrieves all notification channels for a user
func (r *Repository) GetUserNotificationChannels(ctx context.Context, userID int) ([]UserNotificationChannel, error) {
	query := `
		SELECT id, user, channel_type, channel_value, channel_name, is_active, sr_created, sr_updated
		FROM user_notification_channels
		WHERE user = ?
		ORDER BY channel_type ASC, sr_created ASC
	`

	rows, err := r.db.QueryxContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return scanUserNotificationChannels(rows)
}

func scanUserNotificationChannels(rows *sqlx.Rows) ([]UserNotificationChannel, error) {
	var channels []UserNotificationChannel
	for rows.Next() {
		channel, err := scanUserNotificationChannel(rows)
		if err != nil {
			return nil, err
		}
		channels = append(channels, channel)
	}
	return channels, nil
}

func scanUserNotificationChannel(rows *sqlx.Rows) (UserNotificationChannel, error) {
	var channel UserNotificationChannel
	var channelName sql.NullString
	err := rows.Scan(
		&channel.ID,
		&channel.User,
		&channel.ChannelType,
		&channel.ChannelValue,
		&channelName,
		&channel.IsActive,
		&channel.SrCreated,
		&channel.SrUpdated,
	)
	if err != nil {
		return UserNotificationChannel{}, err
	}
	if channelName.Valid {
		channel.ChannelName = &channelName.String
	}
	return channel, nil
}

// CreateUserNotificationChannel creates a new notification channel for a user
func (r *Repository) CreateUserNotificationChannel(ctx context.Context, userID int, channelType, channelValue string, channelName *string) (*UserNotificationChannel, error) {
	query := `
		INSERT INTO user_notification_channels (user, channel_type, channel_value, channel_name)
		VALUES (?, ?, ?, ?)
	`
	result, err := r.db.ExecContext(ctx, query, userID, channelType, channelValue, channelName)
	if err != nil {
		return nil, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	return r.GetUserNotificationChannelByID(ctx, int(id))
}

// GetUserNotificationChannelByID retrieves a notification channel by its ID
func (r *Repository) GetUserNotificationChannelByID(ctx context.Context, id int) (*UserNotificationChannel, error) {
	query := `
		SELECT id, user, channel_type, channel_value, channel_name, is_active, sr_created, sr_updated
		FROM user_notification_channels
		WHERE id = ?
	`

	var channel UserNotificationChannel
	var channelName sql.NullString
	err := r.db.QueryRowxContext(ctx, query, id).Scan(
		&channel.ID,
		&channel.User,
		&channel.ChannelType,
		&channel.ChannelValue,
		&channelName,
		&channel.IsActive,
		&channel.SrCreated,
		&channel.SrUpdated,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if channelName.Valid {
		channel.ChannelName = &channelName.String
	}

	return &channel, nil
}

// UpdateUserNotificationChannel updates a notification channel
func (r *Repository) UpdateUserNotificationChannel(ctx context.Context, channelID int, channelValue string, channelName *string, isActive bool) error {
	query := `
		UPDATE user_notification_channels
		SET channel_value = ?, channel_name = ?, is_active = ?, sr_updated = CURRENT_TIMESTAMP
		WHERE id = ?
	`
	result, err := r.db.ExecContext(ctx, query, channelValue, channelName, isActive, channelID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return fmt.Errorf("notification channel not found")
	}

	return nil
}

// DeleteUserNotificationChannel deletes a notification channel
func (r *Repository) DeleteUserNotificationChannel(ctx context.Context, channelID int) error {
	query := `DELETE FROM user_notification_channels WHERE id = ?`
	result, err := r.db.ExecContext(ctx, query, channelID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return fmt.Errorf("notification channel not found")
	}

	return nil
}

// GetUserNotificationSubscriptions retrieves all notification subscriptions for a user
func (r *Repository) GetUserNotificationSubscriptions(ctx context.Context, userID int) ([]UserNotificationSubscription, error) {
	query := `
		SELECT id, user, event_id, channel_id, sr_created
		FROM user_notification_subscriptions
		WHERE user = ?
		ORDER BY sr_created DESC
	`

	rows, err := r.db.QueryxContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var subscriptions []UserNotificationSubscription
	for rows.Next() {
		var sub UserNotificationSubscription
		err := rows.Scan(
			&sub.ID,
			&sub.User,
			&sub.EventID,
			&sub.ChannelID,
			&sub.SrCreated,
		)
		if err != nil {
			return nil, err
		}
		subscriptions = append(subscriptions, sub)
	}

	return subscriptions, nil
}

// CreateUserNotificationSubscription creates a new notification subscription
func (r *Repository) CreateUserNotificationSubscription(ctx context.Context, userID, eventID, channelID int) (*UserNotificationSubscription, error) {
	query := `
		INSERT INTO user_notification_subscriptions (user, event_id, channel_id)
		VALUES (?, ?, ?)
	`
	result, err := r.db.ExecContext(ctx, query, userID, eventID, channelID)
	if err != nil {
		return nil, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	return r.GetUserNotificationSubscriptionByID(ctx, int(id))
}

// GetUserNotificationSubscriptionByID retrieves a subscription by its ID
func (r *Repository) GetUserNotificationSubscriptionByID(ctx context.Context, id int) (*UserNotificationSubscription, error) {
	query := `
		SELECT id, user, event_id, channel_id, sr_created
		FROM user_notification_subscriptions
		WHERE id = ?
	`

	var sub UserNotificationSubscription
	err := r.db.QueryRowxContext(ctx, query, id).Scan(
		&sub.ID,
		&sub.User,
		&sub.EventID,
		&sub.ChannelID,
		&sub.SrCreated,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	return &sub, nil
}

// DeleteUserNotificationSubscription deletes a notification subscription
func (r *Repository) DeleteUserNotificationSubscription(ctx context.Context, subscriptionID int) error {
	query := `DELETE FROM user_notification_subscriptions WHERE id = ?`
	result, err := r.db.ExecContext(ctx, query, subscriptionID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return fmt.Errorf("notification subscription not found")
	}

	return nil
}

// GetSubscriptionsForEvent retrieves all subscriptions for a specific event code
func (r *Repository) GetSubscriptionsForEvent(ctx context.Context, eventCode string) ([]eventSubscription, error) {
	query := `
		SELECT
			uns.id, uns.user, uns.event_id, uns.channel_id, uns.sr_created,
			unc.id, unc.user, unc.channel_type, unc.channel_value, unc.channel_name, unc.is_active, unc.sr_created, unc.sr_updated,
			ne.id, ne.event_code, ne.event_name, ne.description, ne.sr_created
		FROM user_notification_subscriptions uns
		JOIN user_notification_channels unc ON uns.channel_id = unc.id
		JOIN notification_events ne ON uns.event_id = ne.id
		WHERE ne.event_code = ? AND unc.is_active = 1
	`

	rows, err := r.db.QueryxContext(ctx, query, eventCode)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return scanSubscriptionsForEvent(rows)
}

type eventSubscription struct {
	Channel      UserNotificationChannel
	Event        NotificationEvent
	Subscription UserNotificationSubscription
}

func scanSubscriptionsForEvent(rows *sqlx.Rows) ([]eventSubscription, error) {
	var results []eventSubscription
	for rows.Next() {
		result, err := scanEventSubscription(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, result)
	}
	return results, nil
}

func scanEventSubscription(rows *sqlx.Rows) (eventSubscription, error) {
	var result eventSubscription
	var channelName sql.NullString
	err := rows.Scan(
		&result.Subscription.ID,
		&result.Subscription.User,
		&result.Subscription.EventID,
		&result.Subscription.ChannelID,
		&result.Subscription.SrCreated,
		&result.Channel.ID,
		&result.Channel.User,
		&result.Channel.ChannelType,
		&result.Channel.ChannelValue,
		&channelName,
		&result.Channel.IsActive,
		&result.Channel.SrCreated,
		&result.Channel.SrUpdated,
		&result.Event.ID,
		&result.Event.EventCode,
		&result.Event.EventName,
		&result.Event.Description,
		&result.Event.SrCreated,
	)
	if err != nil {
		return eventSubscription{}, err
	}
	if channelName.Valid {
		result.Channel.ChannelName = &channelName.String
	}
	return result, nil
}

// inTx runs fn in a transaction, committing when it returns nil and rolling back otherwise.
func (r *Repository) inTx(ctx context.Context, fn func(tx *sqlx.Tx) error) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}
