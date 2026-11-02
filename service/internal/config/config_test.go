package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/knadh/koanf/v2"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_Defaults(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()
	os.Args = []string{"sickrock"}

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, "8080", cfg.Port)
	assert.Equal(t, "info", cfg.LogLevel)
	assert.Equal(t, "text", cfg.LogFormat)
	assert.Empty(t, cfg.ConfigDir)
}

func TestLoadAuthConfig(t *testing.T) {
	dir := t.TempDir()
	yamlText := `auth:
  jwt:
    certsUrl: "https://team.cloudflareaccess.com/cdn-cgi/access/certs"
    issuer: "https://team.cloudflareaccess.com"
    aud: "abc123"
    claimUsername: "email"
    header: "Cf-Access-Jwt-Assertion"
    cookieName: "CF_Authorization"
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(yamlText), 0o600))

	k := koanf.New(".")
	require.NoError(t, loadConfigFile(k, dir))
	auth, err := loadAuthConfig(k)
	require.NoError(t, err)

	jwt, ok := auth["jwt"].(map[string]any)
	require.True(t, ok, "auth.jwt should be a map")
	assert.Equal(t, "https://team.cloudflareaccess.com/cdn-cgi/access/certs", jwt["certsUrl"])
	assert.Equal(t, "Cf-Access-Jwt-Assertion", jwt["header"])
	assert.Equal(t, "CF_Authorization", jwt["cookieName"])
	assert.Equal(t, "email", jwt["claimUsername"])
}

func TestLoadAuthConfig_Absent(t *testing.T) {
	auth, err := loadAuthConfig(koanf.New("."))
	require.NoError(t, err)
	assert.Nil(t, auth)
}
