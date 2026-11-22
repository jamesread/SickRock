package iam

import (
	"database/sql"
	"net/http"
	"os"
	"strings"

	armlayer "github.com/jamesread/armature-iam/layer"
	"github.com/jamesread/armature-iam/store"
	log "github.com/sirupsen/logrus"
)

const CookieName = "sickrock-sid"

// AuthLayer wraps armature-iam Connect auth for SickRock.
type AuthLayer struct {
	Store store.Store
	*armlayer.Layer
	db  *sql.DB
	jwt jwtSources
}

// NewAuthLayer wires armature-iam Connect auth. authYAML is the config.yaml "auth"
// block (eg auth.jwt for Cloudflare Access); JWT users are auto-created on first login.
// db is the database that holds the API key tables, for API key expiry and scopes.
func NewAuthLayer(st store.Store, db *sql.DB, authYAML map[string]any) (*AuthLayer, error) {
	cfg := armlayer.Config{
		CookieName:           CookieName,
		AuthYAML:             authYAML,
		Logger:               log.StandardLogger(),
		AllowUnauthenticated: allowUnauthenticated,
		RequiredPermission:   RequiredPermission,
		DevDisableAuth:       os.Getenv("SICKROCK_DEV_DISABLE_AUTH") == "true",
		// Secure cookies require HTTPS. Local HTTP (and Vite proxy) must set
		// SICKROCK_SECURE_COOKIES=false or the browser will drop the session cookie.
		SecureCookies: os.Getenv("SICKROCK_SECURE_COOKIES") != "false",
	}
	inner, err := armlayer.New(st, cfg)
	if err != nil {
		return nil, err
	}
	return &AuthLayer{Layer: inner, Store: st, jwt: jwtSourcesFromAuthYAML(authYAML), db: db}, nil
}

// WrapHandler injects Session-Token / non-API-key Bearer into the session cookie
// so armature-iam's CookieSID provider accepts the SPA's localStorage token on
// local HTTP (where Secure cookies are dropped) and when cookies are absent.
// It also drops the JWT when the browser has opted out of SSO (see SSOOptOutCookieName),
// since armature-iam checks JWTs before the session cookie.
// API key expiry and scopes are applied after authentication (see withAPIKeyScope).
func (a *AuthLayer) WrapHandler(in http.Handler) http.Handler {
	authed := a.Layer.WrapHandler(withAPIKeyScope(a.db, in))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authed.ServeHTTP(w, a.jwt.stripJWTIfOptedOut(injectSessionCookieFromHeaders(r)))
	})
}

// WrapMCPHandler authenticates MCP requests (API keys only) and applies API key expiry and scopes.
func (a *AuthLayer) WrapMCPHandler(in http.Handler) http.Handler {
	return a.Layer.WrapMCPHandler(withAPIKeyScope(a.db, in))
}

func injectSessionCookieFromHeaders(r *http.Request) *http.Request {
	if cookie, err := r.Cookie(CookieName); err == nil && cookie.Value != "" {
		return r
	}
	sid := strings.TrimSpace(r.Header.Get("Session-Token"))
	if sid == "" {
		sid = bearerSessionToken(r.Header.Get("Authorization"))
	}
	if sid == "" {
		return r
	}
	clone := r.Clone(r.Context())
	// Request-side cookie for the auth middleware; Secure/HttpOnly/SameSite are never sent.
	clone.AddCookie(&http.Cookie{Name: CookieName, Value: sid}) //nolint:gosec // G124: request cookie, not Set-Cookie
	return clone
}

// bearerSessionToken returns the bearer token from an Authorization header when it is a session ID.
// API keys use the sk_ prefix; session SIDs are UUIDs / legacy hashes.
func bearerSessionToken(authorization string) string {
	token, ok := strings.CutPrefix(authorization, "Bearer ")
	token = strings.TrimSpace(token)
	if !ok || strings.HasPrefix(token, APIKeyPrefix) {
		return ""
	}
	return token
}
