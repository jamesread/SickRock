package iam

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInjectSessionCookieFromHeaders(t *testing.T) {
	sessionCookie := func(r *http.Request) string {
		c, err := r.Cookie(CookieName)
		if err != nil {
			return ""
		}
		return c.Value
	}

	r, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "/", http.NoBody)
	require.NoError(t, err)
	r.Header.Set("Authorization", "Bearer abc")
	assert.Equal(t, "abc", sessionCookie(injectSessionCookieFromHeaders(r)))

	r.Header.Set("Session-Token", "  def ")
	assert.Equal(t, "def", sessionCookie(injectSessionCookieFromHeaders(r)), "Session-Token wins")

	apiKey, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "/", http.NoBody)
	require.NoError(t, err)
	apiKey.Header.Set("Authorization", "Bearer sk_key")
	assert.Same(t, apiKey, injectSessionCookieFromHeaders(apiKey), "API keys are left for the API key auth")

	withCookie, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "/", http.NoBody)
	require.NoError(t, err)
	withCookie.AddCookie(&http.Cookie{Name: CookieName, Value: "existing"})
	withCookie.Header.Set("Session-Token", "other")
	assert.Same(t, withCookie, injectSessionCookieFromHeaders(withCookie), "an existing cookie is kept")
}
