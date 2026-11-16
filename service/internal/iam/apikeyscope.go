package iam

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/jamesread/armature-iam/rbac"
)

// APIKeyPrefix starts every SickRock API key (session IDs never do).
const APIKeyPrefix = "sk_"

// Resource types an API key can be limited to.
const (
	ResourceTypeTable     = "table"     // key: table configuration name
	ResourceTypeDashboard = "dashboard" // key: dashboard name
	ResourceTypeWorkflow  = "workflow"  // key: workflow id
)

// IsAPIKeyResourceType reports whether t is a resource type a key can be limited to.
func IsAPIKeyResourceType(t string) bool {
	return t == ResourceTypeTable || t == ResourceTypeDashboard || t == ResourceTypeWorkflow
}

// APIKeyResource is one table, dashboard or workflow an API key may use.
type APIKeyResource struct {
	Type string
	Key  string
}

// APIKeyScope narrows what an API key can do. Its owner's current access is always the ceiling:
// a scope can only take access away.
type APIKeyScope struct {
	// When RestrictPermissions is false the key has every permission its owner has.
	RestrictPermissions bool
	Permissions         []string

	// When RestrictResources is false the key can use every table, dashboard and workflow its
	// owner can.
	RestrictResources bool
	Resources         []APIKeyResource

	// OwnerIsSuperuser is set per request, before the owner's RBAC is narrowed, so resource checks
	// can still let a superuser's key reach every resource the scope allows.
	OwnerIsSuperuser bool
}

// AllowsPermission reports whether the scope lets the key use a permission its owner holds.
// A nil scope (not an API key) allows everything.
func (s *APIKeyScope) AllowsPermission(p string) bool {
	return s == nil || !s.RestrictPermissions || slices.Contains(s.Permissions, p)
}

// AllowsResource reports whether the scope lets the key use a table, dashboard or workflow.
func (s *APIKeyScope) AllowsResource(resourceType, key string) bool {
	if s == nil || !s.RestrictResources {
		return true
	}
	return slices.Contains(s.Resources, APIKeyResource{Type: resourceType, Key: key})
}

// APIKeyDetails is what is stored about a key beyond armature-iam's columns.
type APIKeyDetails struct {
	ExpiresAt int64 // Unix seconds; 0 never expires
	Scope     APIKeyScope
}

// Expired reports whether the key has passed its expiry time.
func (d *APIKeyDetails) Expired(now time.Time) bool {
	return d.ExpiresAt > 0 && now.Unix() >= d.ExpiresAt
}

// ErrAPIKeyNotFound is returned when a key does not exist or belongs to another user.
var ErrAPIKeyNotFound = errors.New("API key not found")

// LoadAPIKeyDetailsByValue returns the details of the key with this secret, or nil when the token
// is not a known API key.
func LoadAPIKeyDetailsByValue(ctx context.Context, db *sql.DB, keyValue string) (*APIKeyDetails, error) {
	if !strings.HasPrefix(keyValue, APIKeyPrefix) {
		return nil, nil
	}
	var id int
	err := db.QueryRowContext(ctx, `SELECT id FROM api_keys WHERE key_value = ?`, keyValue).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("load api key: %w", err)
	}
	return LoadAPIKeyDetails(ctx, db, id)
}

// LoadAPIKeyDetails returns the expiry and scope of an API key by ID.
func LoadAPIKeyDetails(ctx context.Context, db *sql.DB, keyID int) (*APIKeyDetails, error) {
	d := &APIKeyDetails{Scope: APIKeyScope{Permissions: []string{}, Resources: []APIKeyResource{}}}
	var expiresAt sql.NullInt64
	var restrictPermissions, restrictResources int
	err := db.QueryRowContext(ctx,
		`SELECT expires_at, restrict_permissions, restrict_resources FROM api_keys WHERE id = ?`, keyID,
	).Scan(&expiresAt, &restrictPermissions, &restrictResources)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrAPIKeyNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("load api key scope: %w", err)
	}
	d.ExpiresAt = expiresAt.Int64
	d.Scope.RestrictPermissions = restrictPermissions != 0
	d.Scope.RestrictResources = restrictResources != 0
	if d.Scope.Permissions, err = loadAPIKeyPermissions(ctx, db, keyID); err != nil {
		return nil, err
	}
	if d.Scope.Resources, err = loadAPIKeyResources(ctx, db, keyID); err != nil {
		return nil, err
	}
	return d, nil
}

func loadAPIKeyPermissions(ctx context.Context, db *sql.DB, keyID int) ([]string, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT permission_name FROM api_key_permissions WHERE api_key_id = ? ORDER BY permission_name`, keyID)
	if err != nil {
		return nil, fmt.Errorf("load api key permissions: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := []string{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		out = append(out, name)
	}
	return out, rows.Err()
}

func loadAPIKeyResources(ctx context.Context, db *sql.DB, keyID int) ([]APIKeyResource, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT resource_type, resource_key FROM api_key_resources WHERE api_key_id = ?
		 ORDER BY resource_type, resource_key`, keyID)
	if err != nil {
		return nil, fmt.Errorf("load api key resources: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := []APIKeyResource{}
	for rows.Next() {
		var r APIKeyResource
		if err := rows.Scan(&r.Type, &r.Key); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// APIKeyUpdate changes one of a user's keys. Nil fields are left as they are.
type APIKeyUpdate struct {
	Name      *string
	ReadOnly  *bool
	ExpiresAt *int64 // 0 clears the expiry
	Scope     *APIKeyScope
}

// UpdateAPIKey applies an update to one of userID's API keys in one transaction. Unknown
// permission names and resource types are refused, so a typo cannot silently produce a key with
// different rights than intended.
func UpdateAPIKey(ctx context.Context, db *sql.DB, keyID, userID int, u APIKeyUpdate) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("update api key begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	sets, args := apiKeyUpdateSet(u)
	args = append(args, keyID, userID)
	// sets is built from fixed column names only.
	res, err := tx.ExecContext(ctx, `UPDATE api_keys SET `+strings.Join(sets, ", ")+` WHERE id = ? AND user_account_id = ?`, args...) //nolint:gosec // G202: fixed column names
	if err != nil {
		return fmt.Errorf("update api key: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrAPIKeyNotFound
	}
	if u.Scope != nil {
		if err := replaceAPIKeyPermissions(ctx, tx, keyID, u.Scope); err != nil {
			return err
		}
		if err := replaceAPIKeyResources(ctx, tx, keyID, u.Scope); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// apiKeyUpdateSet returns the SET clauses for an update. updated_at is always set, so the
// statement also finds keys whose other values do not change.
func apiKeyUpdateSet(u APIKeyUpdate) (sets []string, args []any) {
	sets = []string{"updated_at = ?"}
	args = []any{time.Now().UTC().Format("2006-01-02 15:04:05")}
	if u.Name != nil {
		sets = append(sets, "name = ?")
		args = append(args, *u.Name)
	}
	if u.ReadOnly != nil {
		sets = append(sets, "read_only = ?")
		args = append(args, boolToInt(*u.ReadOnly))
	}
	if u.ExpiresAt != nil {
		sets = append(sets, "expires_at = ?")
		args = append(args, nullUnix(*u.ExpiresAt))
	}
	if u.Scope != nil {
		sets = append(sets, "restrict_permissions = ?", "restrict_resources = ?")
		args = append(args, boolToInt(u.Scope.RestrictPermissions), boolToInt(u.Scope.RestrictResources))
	}
	return sets, args
}

func replaceAPIKeyPermissions(ctx context.Context, tx *sql.Tx, keyID int, scope *APIKeyScope) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM api_key_permissions WHERE api_key_id = ?`, keyID); err != nil {
		return fmt.Errorf("clear api key permissions: %w", err)
	}
	if !scope.RestrictPermissions {
		return nil
	}
	for _, name := range uniqueSorted(scope.Permissions) {
		var known int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM rbac_permissions WHERE name = ?`, name).Scan(&known); err != nil {
			return fmt.Errorf("check permission %s: %w", name, err)
		}
		if known == 0 {
			return &InvalidScopeError{fmt.Sprintf("unknown permission %q", name)}
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO api_key_permissions (api_key_id, permission_name) VALUES (?, ?)`, keyID, name); err != nil {
			return fmt.Errorf("add api key permission %s: %w", name, err)
		}
	}
	return nil
}

func replaceAPIKeyResources(ctx context.Context, tx *sql.Tx, keyID int, scope *APIKeyScope) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM api_key_resources WHERE api_key_id = ?`, keyID); err != nil {
		return fmt.Errorf("clear api key resources: %w", err)
	}
	if !scope.RestrictResources {
		return nil
	}
	seen := map[APIKeyResource]bool{}
	for _, r := range scope.Resources {
		r.Key = strings.TrimSpace(r.Key)
		if !IsAPIKeyResourceType(r.Type) {
			return &InvalidScopeError{fmt.Sprintf("unknown resource type %q", r.Type)}
		}
		if r.Key == "" || seen[r] {
			continue
		}
		seen[r] = true
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO api_key_resources (api_key_id, resource_type, resource_key) VALUES (?, ?, ?)`,
			keyID, r.Type, r.Key); err != nil {
			return fmt.Errorf("add api key resource %s %s: %w", r.Type, r.Key, err)
		}
	}
	return nil
}

// DeleteAPIKeyDetails removes a key's scope rows (for databases without foreign key enforcement).
func DeleteAPIKeyDetails(ctx context.Context, db *sql.DB, keyID int) error {
	for _, q := range []string{
		`DELETE FROM api_key_permissions WHERE api_key_id = ?`,
		`DELETE FROM api_key_resources WHERE api_key_id = ?`,
	} {
		if _, err := db.ExecContext(ctx, q, keyID); err != nil {
			return fmt.Errorf("delete api key scope: %w", err)
		}
	}
	return nil
}

// InvalidScopeError reports a scope that names something that does not exist.
type InvalidScopeError struct{ msg string }

func (e *InvalidScopeError) Error() string { return e.msg }

// NarrowRBAC returns the permissions an API key request really has: the owner's effective
// permissions limited by the key's scope. A permission-restricted key never counts as superuser.
func NarrowRBAC(owner *rbac.EffectiveRBAC, scope *APIKeyScope) *rbac.EffectiveRBAC {
	if scope == nil || !scope.RestrictPermissions || owner == nil {
		return owner
	}
	out := &rbac.EffectiveRBAC{Permissions: map[string]bool{}, RoleNames: owner.RoleNames}
	for p, ok := range owner.Permissions {
		// A superuser's map holds every permission (armature-iam loads them all).
		if (ok || owner.IsSuperuser) && scope.AllowsPermission(p) {
			out.Permissions[p] = true
		}
	}
	return out
}

type apiKeyScopeCtxKey struct{}

// ContextWithAPIKeyScope records the scope of the API key that authenticated the request.
func ContextWithAPIKeyScope(ctx context.Context, scope *APIKeyScope) context.Context {
	return context.WithValue(ctx, apiKeyScopeCtxKey{}, scope)
}

// APIKeyScopeFromContext returns the scope of the API key that authenticated this request, or
// nil for browser sessions and SSO (which are limited only by their RBAC).
func APIKeyScopeFromContext(ctx context.Context) *APIKeyScope {
	s, _ := ctx.Value(apiKeyScopeCtxKey{}).(*APIKeyScope)
	return s
}

func nullUnix(v int64) any {
	if v <= 0 {
		return nil
	}
	return v
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func uniqueSorted(in []string) []string {
	out := []string{}
	for _, s := range in {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	slices.Sort(out)
	return slices.Compact(out)
}
