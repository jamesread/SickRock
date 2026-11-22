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
	return j.headerPresent(r) || j.cookiePresent(r)
}

func (j jwtSources) headerPresent(r *http.Request) bool {
	return j.header != "" && r.Header.Get(j.header) != ""
}

func (j jwtSources) cookiePresent(r *http.Request) bool {
	if j.cookie == "" {
		return false
	}
	c, err := r.Cookie(j.cookie)
	return err == nil && c.Value != ""
}

func hasSSOOptOut(r *http.Request) bool {
	c, err := r.Cookie(SSOOptOutCookieName)
	return err == nil && c.Value != ""
}

// optedOutWithJWT reports whether the browser has opted out of SSO yet the request carries a JWT.
func (j jwtSources) optedOutWithJWT(r *http.Request) bool {
	return j.configured() && hasSSOOptOut(r) && j.present(r)
}

// stripJWTIfOptedOut removes the JWT header and cookie when the browser has opted out
// of SSO, so the auth layer falls through to the local session (or no user).
func (j jwtSources) stripJWTIfOptedOut(r *http.Request) *http.Request {
	if !j.optedOutWithJWT(r) {
		return r
	}
	clone := r.Clone(r.Context())
	if j.header != "" {
		clone.Header.Del(j.header)
	}
	if j.cookie != "" {
		removeCookie(clone, r.Cookies(), j.cookie)
	}
	return clone
}

// removeCookie rewrites the Cookie header of r to hold every cookie except name.
func removeCookie(r *http.Request, cookies []*http.Cookie, name string) {
	kept := make([]string, 0, len(cookies))
	for _, c := range cookies {
		if c.Name != name {
			kept = append(kept, c.Name+"="+c.Value)
		}
	}
	r.Header.Del("Cookie")
	if len(kept) > 0 {
		r.Header.Set("Cookie", strings.Join(kept, "; "))
	}
}

// SSOOptOutCookie returns the cookie to set on logout when the request was made with
// a configured JWT, so the user is not signed straight back in by the proxy.
func (a *AuthLayer) SSOOptOutCookie(header http.Header) (http.Cookie, bool) {
	r := &http.Request{Header: header}
	if !a.jwt.present(r) {
		return http.Cookie{}, false
	}
	// Not HttpOnly on purpose: the SPA clears it to sign in with SSO again, and it
	// carries no credentials (see SSOOptOutCookieName).
	return http.Cookie{ //nolint:gosec // G124: intentionally readable by the SPA
		Name:     SSOOptOutCookieName,
		Value:    "1",
		Path:     "/",
		MaxAge:   ssoOptOutMaxAge,
		SameSite: http.SameSiteLaxMode,
		Secure:   a.SecureCookiesEnabled(),
	}, true
}
