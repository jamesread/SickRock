package iam

import (
	"net/http"
	"os"
	"strings"

	armlayer "github.com/jamesread/armature-iam/layer"
	"github.com/jamesread/armature-iam/store"
	sickrockpbconnect "github.com/jamesread/SickRock/gen/sickrockpbconnect"
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
		if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
			token := strings.TrimSpace(strings.TrimPrefix(auth, "Bearer "))
			// API keys use the sk_ prefix; session SIDs are UUIDs / legacy hashes.
			if token != "" && !strings.HasPrefix(token, "sk_") {
				sid = token
			}
		}
	}
	if sid == "" {
		return r
	}
	clone := r.Clone(r.Context())
	clone.AddCookie(&http.Cookie{Name: CookieName, Value: sid})
	return clone
}
