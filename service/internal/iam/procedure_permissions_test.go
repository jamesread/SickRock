package iam

import (
	"testing"

	"github.com/jamesread/armature-iam/rbac"
	"github.com/stretchr/testify/assert"

	sickrockpbconnect "github.com/jamesread/SickRock/gen/sickrockpbconnect"
)

func TestRequiredPermission(t *testing.T) {
	assert.Equal(t, rbac.PermissionUsersView, RequiredPermission(sickrockpbconnect.SickRockListUsersProcedure))
	assert.Equal(t, rbac.PermissionSystemSettings, RequiredPermission(sickrockpbconnect.SickRockRunHealthChecksProcedure))
	assert.Empty(t, RequiredPermission(sickrockpbconnect.SickRockGetMyPermissionsAuditProcedure), "self-service")
	assert.Equal(t, rbac.PermissionAppAccess, RequiredPermission(sickrockpbconnect.SickRockEditItemProcedure), "unlisted write")
	assert.Equal(t, PermissionAppRead, RequiredPermission(sickrockpbconnect.SickRockListItemsProcedure), "unlisted read")
}
