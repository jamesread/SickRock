package iam

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	armlayer "github.com/jamesread/armature-iam/layer"
	iamstore "github.com/jamesread/armature-iam/store"
	iamsqlite "github.com/jamesread/armature-iam/store/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sickrockpbconnect "github.com/jamesread/SickRock/gen/sickrockpbconnect"
)

const (
	testJWTSecret = "test-hmac-secret"
	testJWTHeader = "Cf-Access-Jwt-Assertion"
	testJWTIssuer = "https://team.cloudflareaccess.com"
	testJWTAud    = "test-aud"
)

func testJWTAuthYAML() map[string]any {
	return map[string]any{
		"jwt": map[string]any{
			"hmacSecret":    testJWTSecret,
			"issuer":        testJWTIssuer,
			"aud":           testJWTAud,
			"claimUsername": "email",
			"header":        testJWTHeader,
		},
	}
}

func signTestJWT(t *testing.T, email string) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"email": email,
		"iss":   testJWTIssuer,
		"aud":   testJWTAud,
		"exp":   time.Now().Add(time.Hour).Unix(),
	})
	s, err := tok.SignedString([]byte(testJWTSecret))
	require.NoError(t, err)
	return s
}

func TestNewAuthLayer_JWTAutoCreatesUser(t *testing.T) {
	st, err := iamsqlite.OpenMemory()
	require.NoError(t, err)
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()
	require.NoError(t, st.EnsureRBACBootstrap(ctx))

	layer, err := NewAuthLayer(st, testJWTAuthYAML())
	require.NoError(t, err)

	req := httptest.NewRequestWithContext(ctx, "POST", sickrockpbconnect.SickRockInitProcedure, nil)
	req.Header.Set(testJWTHeader, signTestJWT(t, "alice@example.com"))
	info, err := layer.Handle(ctx, req)
	require.NoError(t, err)

	au, ok := info.(*armlayer.AuthenticatedUser)
	require.True(t, ok, "JWT request should be authenticated")
	assert.Equal(t, "alice@example.com", au.User.Username)

	user, err := st.GetUserByUsername(ctx, "alice@example.com")
	require.NoError(t, err)
	require.NotNil(t, user)
	assert.Equal(t, iamstore.UserCreatedBySSO, user.CreatedBy)
}

func TestNewAuthLayer_NoAuthConfigIgnoresJWT(t *testing.T) {
	st, err := iamsqlite.OpenMemory()
	require.NoError(t, err)
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()

	layer, err := NewAuthLayer(st, nil)
	require.NoError(t, err)

	req := httptest.NewRequestWithContext(ctx, "POST", sickrockpbconnect.SickRockInitProcedure, nil)
	req.Header.Set(testJWTHeader, signTestJWT(t, "bob@example.com"))
	info, err := layer.Handle(ctx, req)
	require.NoError(t, err, "Init is allowed unauthenticated")
	assert.Nil(t, info)

	user, err := st.GetUserByUsername(ctx, "bob@example.com")
	require.NoError(t, err)
	assert.Nil(t, user)
}
