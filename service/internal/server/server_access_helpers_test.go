package server

import (
	"testing"

	armlayer "github.com/jamesread/armature-iam/layer"
	"github.com/jamesread/armature-iam/rbac"
	armstore "github.com/jamesread/armature-iam/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jamesread/SickRock/internal/iam/systemroles"
)

func authUserWith(superuser bool, permissions ...string) *armlayer.AuthenticatedUser {
	perms := map[string]bool{}
	for _, p := range permissions {
		perms[p] = true
	}
	return &armlayer.AuthenticatedUser{
		User: &armstore.UserAccountRow{ID: 1},
		RBAC: &rbac.EffectiveRBAC{Permissions: perms, IsSuperuser: superuser},
	}
}

func TestIsAuthenticated(t *testing.T) {
	assert.False(t, isAuthenticated(nil))
	assert.False(t, isAuthenticated(&armlayer.AuthenticatedUser{}))
	assert.True(t, isAuthenticated(authUserWith(false)))
}

func TestIsSuperuser(t *testing.T) {
	assert.False(t, isSuperuser(nil))
	assert.False(t, isSuperuser(&armlayer.AuthenticatedUser{User: &armstore.UserAccountRow{ID: 1}}))
	assert.False(t, isSuperuser(authUserWith(false)))
	assert.True(t, isSuperuser(authUserWith(true)))
}

func TestMayViewDashboards(t *testing.T) {
	assert.True(t, mayViewDashboards(&armlayer.AuthenticatedUser{User: &armstore.UserAccountRow{ID: 1}}), "no RBAC loaded defers to grants")
	assert.True(t, mayViewDashboards(authUserWith(false, systemroles.PermissionDashboardView)))
	assert.True(t, mayViewDashboards(authUserWith(false, rbac.PermissionAppAccess)))
	assert.False(t, mayViewDashboards(authUserWith(false, "table.view")))
}

func TestValidateChannelValue(t *testing.T) {
	require.Error(t, validateChannelValue("telegram", ""))
	require.NoError(t, validateChannelValue("telegram", "12345"))
	require.Error(t, validateChannelValue("email", "not-an-address"))
	require.NoError(t, validateChannelValue("email", "a@example.com"))
	require.Error(t, validateChannelValue("webhook", "ftp://example.com"))
	require.NoError(t, validateChannelValue("webhook", "https://example.com/hook"))
}

func TestOptionalString(t *testing.T) {
	assert.Nil(t, optionalString("   "))
	if got := optionalString(" name "); assert.NotNil(t, got) {
		assert.Equal(t, "name", *got)
	}
}
