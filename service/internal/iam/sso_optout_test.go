package iam

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	armlayer "github.com/jamesread/armature-iam/layer"
	iamsqlite "github.com/jamesread/armature-iam/store/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sickrockpbconnect "github.com/jamesread/SickRock/gen/sickrockpbconnect"
)

const testJWTCookie = "CF_Authorization"

func testJWTAuthYAMLWithCookie() map[string]any {
	y := testJWTAuthYAML()
	y["jwt"].(map[string]any)["cookieName"] = testJWTCookie
	return y
}

func newJWTRequest(t *testing.T, email string) *http.Request {
	t.Helper()
	req := httptest.NewRequestWithContext(context.Background(), "POST", sickrockpbconnect.SickRockInitProcedure, nil)
	tok := signTestJWT(t, email)
	req.Header.Set(testJWTHeader, tok)
	req.AddCookie(&http.Cookie{Name: testJWTCookie, Value: tok})
	return req
}

func TestStripJWTIfOptedOut(t *testing.T) {
	j := jwtSourcesFromAuthYAML(testJWTAuthYAMLWithCookie())
	require.True(t, j.configured())

	req := newJWTRequest(t, "alice@example.com")
	assert.Same(t, req, j.stripJWTIfOptedOut(req), "no opt-out cookie: request unchanged")

	req.AddCookie(&http.Cookie{Name: SSOOptOutCookieName, Value: "1"})
	req.AddCookie(&http.Cookie{Name: CookieName, Value: "local-sid"})
	stripped := j.stripJWTIfOptedOut(req)

	assert.Empty(t, stripped.Header.Get(testJWTHeader))
	_, err := stripped.Cookie(testJWTCookie)
	require.ErrorIs(t, err, http.ErrNoCookie)
	sid, err := stripped.Cookie(CookieName)
	require.NoError(t, err)
	assert.Equal(t, "local-sid", sid.Value, "local session cookie is kept")
	assert.NotEmpty(t, req.Header.Get(testJWTHeader), "original request is not modified")
}

func TestStripJWTIfOptedOut_NotConfigured(t *testing.T) {
	j := jwtSourcesFromAuthYAML(nil)
	assert.False(t, j.configured())
	req := newJWTRequest(t, "alice@example.com")
	req.AddCookie(&http.Cookie{Name: SSOOptOutCookieName, Value: "1"})
	assert.Same(t, req, j.stripJWTIfOptedOut(req))
}

func TestSSOOptOut_IgnoresJWTEndToEnd(t *testing.T) {
	st, err := iamsqlite.OpenMemory()
	require.NoError(t, err)
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()
	require.NoError(t, st.EnsureRBACBootstrap(ctx))

	layer, err := NewAuthLayer(st, testJWTAuthYAMLWithCookie())
	require.NoError(t, err)

	req := newJWTRequest(t, "alice@example.com")
	req.AddCookie(&http.Cookie{Name: SSOOptOutCookieName, Value: "1"})
	info, err := layer.Handle(ctx, layer.jwt.stripJWTIfOptedOut(req))
	require.NoError(t, err, "Init is allowed unauthenticated")
	assert.Nil(t, info, "opted-out browser is not signed in by the JWT")

	info, err = layer.Handle(ctx, newJWTRequest(t, "alice@example.com"))
	require.NoError(t, err)
	au, ok := info.(*armlayer.AuthenticatedUser)
	require.True(t, ok, "without the opt-out cookie the JWT still signs in")
	assert.Equal(t, "alice@example.com", au.User.Username)
}

func TestSSOOptOutCookie(t *testing.T) {
	st, err := iamsqlite.OpenMemory()
	require.NoError(t, err)
	t.Cleanup(func() { _ = st.Close() })

	layer, err := NewAuthLayer(st, testJWTAuthYAML())
	require.NoError(t, err)

	c, ok := layer.SSOOptOutCookie(newJWTRequest(t, "alice@example.com").Header)
	require.True(t, ok)
	assert.Equal(t, SSOOptOutCookieName, c.Name)
	assert.Equal(t, "/", c.Path)
	assert.Positive(t, c.MaxAge)
	assert.False(t, c.HttpOnly, "the SPA clears it to sign in with SSO again")

	_, ok = layer.SSOOptOutCookie(http.Header{})
	assert.False(t, ok, "no JWT on the request: nothing to opt out of")

	noJWT, err := NewAuthLayer(st, nil)
	require.NoError(t, err)
	_, ok = noJWT.SSOOptOutCookie(newJWTRequest(t, "alice@example.com").Header)
	assert.False(t, ok, "JWT not configured")
}
