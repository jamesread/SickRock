package iam

import (
	"net/http"
	"os"
	"strings"

	sickrockpbconnect "github.com/jamesread/SickRock/gen/sickrockpbconnect"
	armlayer "github.com/jamesread/armature-iam/layer"
	"github.com/jamesread/armature-iam/store"
	log "github.com/sirupsen/logrus"
)

const CookieName = "sickrock-sid"

// AuthLayer wraps armature-iam Connect auth for SickRock.
type AuthLayer struct {
	*armlayer.Layer
	Store store.Store
}

// NewAuthLayer wires armature-iam Connect auth.
func NewAuthLayer(st store.Store) (*AuthLayer, error) {
	cfg := armlayer.Config{
		CookieName: CookieName,
		Logger:     log.StandardLogger(),
		AllowUnauthenticated: []string{
			sickrockpbconnect.SickRockInitProcedure,
			sickrockpbconnect.SickRockLoginProcedure,
			sickrockpbconnect.SickRockLogoutProcedure,
			sickrockpbconnect.SickRockValidateTokenProcedure,
			sickrockpbconnect.SickRockGenerateDeviceCodeProcedure,
			sickrockpbconnect.SickRockCheckDeviceCodeProcedure,
			sickrockpbconnect.SickRockGetDeviceCodeSessionProcedure,
			sickrockpbconnect.SickRockPingProcedure,
		},
		RequiredPermission: RequiredPermission,
		DevDisableAuth:     os.Getenv("SICKROCK_DEV_DISABLE_AUTH") == "true",
		// Secure cookies require HTTPS. Local HTTP (and Vite proxy) must set
		// SICKROCK_SECURE_COOKIES=false or the browser will drop the session cookie.
		SecureCookies: os.Getenv("SICKROCK_SECURE_COOKIES") != "false",
	}
	inner, err := armlayer.New(st, cfg)
	if err != nil {
		return nil, err
	}
	return &AuthLayer{Layer: inner, Store: st}, nil
}

// WrapHandler injects Session-Token / non-API-key Bearer into the session cookie
// so armature-iam's CookieSID provider accepts the SPA's localStorage token on
// local HTTP (where Secure cookies are dropped) and when cookies are absent.
func (a *AuthLayer) WrapHandler(in http.Handler) http.Handler {
	authed := a.Layer.WrapHandler(in)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authed.ServeHTTP(w, injectSessionCookieFromHeaders(r))
	})
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
	if !ok || strings.HasPrefix(token, "sk_") {
		return ""
	}
	return token
}
