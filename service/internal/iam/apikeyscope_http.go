package iam

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"connectrpc.com/authn"
	"connectrpc.com/connect"
	sickrockpbconnect "github.com/jamesread/SickRock/gen/sickrockpbconnect"
	armlayer "github.com/jamesread/armature-iam/layer"
	log "github.com/sirupsen/logrus"
)

// API key scopes are enforced here, after armature-iam has authenticated the key's owner and
// checked the owner holds the procedure's permission. This then checks the key's expiry,
// read-only flag and own permissions, narrows the request's RBAC to the scope, and records the
// scope in the context for resource (table, dashboard, workflow) checks.

// allowUnauthenticated procedures need no login, so no key checks apply to them either.
var allowUnauthenticated = []string{
	sickrockpbconnect.SickRockInitProcedure,
	sickrockpbconnect.SickRockLoginProcedure,
	sickrockpbconnect.SickRockLogoutProcedure,
	sickrockpbconnect.SickRockValidateTokenProcedure,
	sickrockpbconnect.SickRockGenerateDeviceCodeProcedure,
	sickrockpbconnect.SickRockCheckDeviceCodeProcedure,
	sickrockpbconnect.SickRockGetDeviceCodeSessionProcedure,
	sickrockpbconnect.SickRockPingProcedure,
}

var allowUnauthenticatedSet = func() map[string]bool {
	out := map[string]bool{}
	for _, p := range allowUnauthenticated {
		out[p] = true
	}
	return out
}()

// sessionOnlyProcedures cannot be called with an API key: a key must not create, widen or remove
// keys, or change passwords.
var sessionOnlyProcedures = map[string]bool{
	sickrockpbconnect.SickRockCreateAPIKeyProcedure:     true,
	sickrockpbconnect.SickRockUpdateAPIKeyProcedure:     true,
	sickrockpbconnect.SickRockDeleteAPIKeyProcedure:     true,
	sickrockpbconnect.SickRockDeactivateAPIKeyProcedure: true,
	sickrockpbconnect.SickRockChangePasswordProcedure:   true,
	sickrockpbconnect.SickRockClaimDeviceCodeProcedure:  true,
}

// readProcedurePrefixes start the method names of procedures that change nothing.
var readProcedurePrefixes = []string{"Get", "List", "Ping", "Init", "Validate", "Check", "Preview"}

// IsReadProcedure reports whether a Connect procedure only reads. Read-only API keys may call
// only these; anything not named like a read is treated as a write.
func IsReadProcedure(procedure string) bool {
	method := procedure[strings.LastIndex(procedure, "/")+1:]
	for _, prefix := range readProcedurePrefixes {
		if strings.HasPrefix(method, prefix) {
			return true
		}
	}
	return false
}

// AuthorizeAPIKeyProcedure checks a procedure against an API key's read-only flag and scope.
// scope is nil for requests not made with an API key.
func AuthorizeAPIKeyProcedure(procedure string, readOnly bool, scope *APIKeyScope) error {
	if allowUnauthenticatedSet[procedure] {
		return nil
	}
	if err := denySessionOnlyAPIKey(procedure, scope); err != nil {
		return err
	}
	if err := denyReadOnlyAPIKeyWrite(procedure, readOnly); err != nil {
		return err
	}
	return denyMissingScopedPermission(procedure, scope)
}

func denySessionOnlyAPIKey(procedure string, scope *APIKeyScope) error {
	if scope == nil || !sessionOnlyProcedures[procedure] {
		return nil
	}
	return connect.NewError(connect.CodePermissionDenied,
		errors.New("API keys cannot manage API keys or passwords; log in to the web UI"))
}

func denyReadOnlyAPIKeyWrite(procedure string, readOnly bool) error {
	if !readOnly || IsReadProcedure(procedure) {
		return nil
	}
	return connect.NewError(connect.CodePermissionDenied,
		fmt.Errorf("read-only API key cannot call %s", procedure))
}

func denyMissingScopedPermission(procedure string, scope *APIKeyScope) error {
	required := RequiredPermission(procedure)
	if required == "" || scope.AllowsPermission(required) {
		return nil
	}
	return connect.NewError(connect.CodePermissionDenied,
		fmt.Errorf("this API key does not have the %s permission", required))
}

// lacksRequiredPermission reports whether procedure needs a permission the user does not have.
func lacksRequiredPermission(au *armlayer.AuthenticatedUser, procedure, required string) bool {
	return required != "" && !allowUnauthenticatedSet[procedure] && !au.HasPermission(required)
}

// AuthorizeProcedure checks that the caller may run a Connect procedure: its RBAC permission and,
// for API keys, the key's read-only flag and scope. It is for callers that reach server methods
// without going through the Connect handler, such as the MCP server.
func AuthorizeProcedure(ctx context.Context, procedure string) error {
	au, ok := authenticatedAPIKeyUser(ctx)
	if !ok {
		return connect.NewError(connect.CodeUnauthenticated, errors.New("authentication required"))
	}
	if required := RequiredPermission(procedure); lacksRequiredPermission(au, procedure, required) {
		return connect.NewError(connect.CodePermissionDenied, fmt.Errorf("missing permission %s", required))
	}
	return AuthorizeAPIKeyProcedure(procedure, au.ReadOnly, APIKeyScopeFromContext(ctx))
}

// withAPIKeyScope wraps a handler that runs after armature-iam's auth middleware.
func withAPIKeyScope(db *sql.DB, next http.Handler) http.Handler {
	if db == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, err := applyAPIKeyScope(r, db)
		if err != nil {
			writeScopeError(w, r, err)
			return
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func applyAPIKeyScope(r *http.Request, db *sql.DB) (context.Context, error) {
	ctx := r.Context()
	token, au, ok := apiKeyRequestUser(r)
	if !ok {
		return ctx, nil
	}
	details, err := loadLiveAPIKeyDetails(ctx, db, token)
	if err != nil {
		return ctx, err
	}
	if details == nil {
		return ctx, nil
	}
	scope := details.Scope
	if err := authorizeScopedRequest(r, au, &scope); err != nil {
		return ctx, err
	}
	scope.OwnerIsSuperuser = ownerIsSuperuser(au)
	au.RBAC = NarrowRBAC(au.RBAC, &scope)
	return ContextWithAPIKeyScope(ctx, &scope), nil
}

// apiKeyRequestUser returns the API key token the request carries and its authenticated user. It is false
// without a key, or for a guest on an allow-listed procedure (the key was not accepted).
func apiKeyRequestUser(r *http.Request) (string, *armlayer.AuthenticatedUser, bool) {
	token, ok := bearerAPIKey(r)
	if !ok {
		return "", nil, false
	}
	au, ok := authenticatedAPIKeyUser(r.Context())
	return token, au, ok
}

func ownerIsSuperuser(au *armlayer.AuthenticatedUser) bool {
	return au.RBAC != nil && au.RBAC.IsSuperuser
}

func bearerAPIKey(r *http.Request) (string, bool) {
	token, ok := authn.BearerToken(r)
	token = strings.TrimSpace(token)
	if !ok || !strings.HasPrefix(token, APIKeyPrefix) {
		return "", false
	}
	return token, true
}

func authenticatedAPIKeyUser(ctx context.Context) (*armlayer.AuthenticatedUser, bool) {
	au := armlayer.UserFromContext(ctx)
	if au == nil || au.User == nil {
		return nil, false
	}
	return au, true
}

func loadLiveAPIKeyDetails(ctx context.Context, db *sql.DB, token string) (*APIKeyDetails, error) {
	details, err := LoadAPIKeyDetailsByValue(ctx, db, token)
	if err != nil {
		log.WithError(err).Error("load api key scope")
		return nil, connect.NewError(connect.CodeInternal, errors.New("could not load API key"))
	}
	if details == nil {
		return nil, nil
	}
	if details.Expired(time.Now()) {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("API key has expired"))
	}
	return details, nil
}

func authorizeScopedRequest(r *http.Request, au *armlayer.AuthenticatedUser, scope *APIKeyScope) error {
	procedure, ok := connectProcedure(r)
	if !ok {
		return nil
	}
	if err := AuthorizeAPIKeyProcedure(procedure, au.ReadOnly, scope); err != nil {
		log.WithFields(log.Fields{"user": au.User.Username, "procedure": procedure}).WithError(err).Warn("API key scope denied")
		return err
	}
	return nil
}

// connectProcedure returns the SickRock Connect procedure a request calls, if it calls one.
func connectProcedure(r *http.Request) (string, bool) {
	procedure, ok := authn.InferProcedure(r.URL)
	if !ok || !strings.HasPrefix(procedure, "/"+sickrockpbconnect.SickRockName+"/") {
		return "", false
	}
	return procedure, true
}

func writeScopeError(w http.ResponseWriter, r *http.Request, err error) {
	ew := connect.NewErrorWriter()
	if ew.IsSupported(r) {
		_ = ew.Write(w, r, err)
		return
	}
	status := http.StatusForbidden
	msg := err.Error()
	if cerr, ok := errors.AsType[*connect.Error](err); ok {
		msg = cerr.Message()
		switch cerr.Code() {
		case connect.CodeUnauthenticated:
			status = http.StatusUnauthorized
		case connect.CodeInternal:
			status = http.StatusInternalServerError
		}
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
