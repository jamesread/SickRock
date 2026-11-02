package iam

import (
	"net/http"
	"strings"
)

// SSOOptOutCookieName marks a browser that logged out of a JWT (eg Cloudflare Access)
// session. While it is set, JWTs are ignored so the login form can be used with a
// local account. It is readable by the SPA (not HttpOnly), which clears it to sign
// in with SSO again; it carries no credentials.
const SSOOptOutCookieName = "sickrock-sso-off"

const ssoOptOutMaxAge = 365 * 24 * 60 * 60

// jwtSources is where the configured JWT is read from (auth.jwt.header / cookieName).
type jwtSources struct {
	header string
	cookie string
}

func jwtSourcesFromAuthYAML(authYAML map[string]any) jwtSources {
	jwt, _ := authYAML["jwt"].(map[string]any)
	header, _ := jwt["header"].(string)
	cookie, _ := jwt["cookieName"].(string)
	return jwtSources{header: strings.TrimSpace(header), cookie: strings.TrimSpace(cookie)}
}

func (j jwtSources) configured() bool {
	return j.header != "" || j.cookie != ""
}

// present reports whether the request carries a JWT from a configured source.
func (j jwtSources) present(r *http.Request) bool {
	if j.header != "" && r.Header.Get(j.header) != "" {
		return true
	}
	if j.cookie != "" {
		if c, err := r.Cookie(j.cookie); err == nil && c.Value != "" {
			return true
		}
	}
	return false
}

func hasSSOOptOut(r *http.Request) bool {
	c, err := r.Cookie(SSOOptOutCookieName)
	return err == nil && c.Value != ""
}

// stripJWTIfOptedOut removes the JWT header and cookie when the browser has opted out
// of SSO, so the auth layer falls through to the local session (or no user).
func (j jwtSources) stripJWTIfOptedOut(r *http.Request) *http.Request {
	if !j.configured() || !hasSSOOptOut(r) || !j.present(r) {
		return r
	}
	clone := r.Clone(r.Context())
	if j.header != "" {
		clone.Header.Del(j.header)
	}
	if j.cookie != "" {
		kept := make([]string, 0)
		for _, c := range r.Cookies() {
			if c.Name != j.cookie {
				kept = append(kept, c.Name+"="+c.Value)
			}
		}
		clone.Header.Del("Cookie")
		if len(kept) > 0 {
			clone.Header.Set("Cookie", strings.Join(kept, "; "))
		}
	}
	return clone
}

// SSOOptOutCookie returns the cookie to set on logout when the request was made with
// a configured JWT, so the user is not signed straight back in by the proxy.
func (a *AuthLayer) SSOOptOutCookie(header http.Header) (http.Cookie, bool) {
	r := &http.Request{Header: header}
	if !a.jwt.present(r) {
		return http.Cookie{}, false
	}
	return http.Cookie{
		Name:     SSOOptOutCookieName,
		Value:    "1",
		Path:     "/",
		MaxAge:   ssoOptOutMaxAge,
		SameSite: http.SameSiteLaxMode,
		Secure:   a.SecureCookiesEnabled(),
	}, true
}
