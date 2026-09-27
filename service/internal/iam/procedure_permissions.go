package iam

import (
	"github.com/jamesread/armature-iam/rbac"
	sickrockpbconnect "github.com/jamesread/SickRock/gen/sickrockpbconnect"
)

var writeProcedures = map[string]bool{
	sickrockpbconnect.SickRockResetUserPasswordProcedure:                      true,
	sickrockpbconnect.SickRockChangePasswordProcedure:                         true,
	sickrockpbconnect.SickRockCreateUserProcedure:                             true,
	sickrockpbconnect.SickRockDeleteUserProcedure:                             true,
	sickrockpbconnect.SickRockCreateRbacRoleProcedure:                         true,
	sickrockpbconnect.SickRockUpdateRbacRoleProcedure:                         true,
	sickrockpbconnect.SickRockDeleteRbacRoleProcedure:                         true,
	sickrockpbconnect.SickRockSetUserGroupRbacRolesProcedure:                  true,
	sickrockpbconnect.SickRockSetGroupTableRoleGrantsProcedure:                true,
	sickrockpbconnect.SickRockSetTableShareGrantsProcedure:                    true,
	sickrockpbconnect.SickRockCreateUserGroupProcedure:                        true,
	sickrockpbconnect.SickRockDeleteUserGroupProcedure:                        true,
	sickrockpbconnect.SickRockSetUserGroupMembersProcedure:                    true,
	sickrockpbconnect.SickRockCreateTableProcedure:                            true,
	sickrockpbconnect.SickRockCreateTableConfigurationProcedure:               true,
	sickrockpbconnect.SickRockCreateItemProcedure:                             true,
	sickrockpbconnect.SickRockEditItemProcedure:                               true,
	sickrockpbconnect.SickRockDeleteItemProcedure:                             true,
	sickrockpbconnect.SickRockAddTableColumnProcedure:                         true,
	sickrockpbconnect.SickRockCreateTableViewProcedure:                        true,
	sickrockpbconnect.SickRockUpdateTableViewProcedure:                        true,
	sickrockpbconnect.SickRockDeleteTableViewProcedure:                        true,
	sickrockpbconnect.SickRockCreateForeignKeyProcedure:                       true,
	sickrockpbconnect.SickRockDeleteForeignKeyProcedure:                       true,
	sickrockpbconnect.SickRockChangeColumnTypeProcedure:                       true,
	sickrockpbconnect.SickRockDropColumnProcedure:                             true,
	sickrockpbconnect.SickRockChangeColumnNameProcedure:                       true,
	sickrockpbconnect.SickRockCreateDashboardComponentRuleProcedure:           true,
	sickrockpbconnect.SickRockCreateUserBookmarkProcedure:                     true,
	sickrockpbconnect.SickRockDeleteUserBookmarkProcedure:                     true,
	sickrockpbconnect.SickRockCreateConditionalFormattingRuleProcedure:        true,
	sickrockpbconnect.SickRockUpdateConditionalFormattingRuleProcedure:        true,
	sickrockpbconnect.SickRockDeleteConditionalFormattingRuleProcedure:        true,
	sickrockpbconnect.SickRockCreateUserNotificationChannelProcedure:          true,
	sickrockpbconnect.SickRockUpdateUserNotificationChannelProcedure:          true,
	sickrockpbconnect.SickRockDeleteUserNotificationChannelProcedure:          true,
	sickrockpbconnect.SickRockCreateUserNotificationSubscriptionProcedure:     true,
	sickrockpbconnect.SickRockDeleteUserNotificationSubscriptionProcedure:     true,
	sickrockpbconnect.SickRockSetTickListCompletionProcedure:                  true,
	sickrockpbconnect.SickRockClearTickListStateProcedure:                     true,
}

// RequiredPermission maps Connect procedures to RBAC permission names.
func RequiredPermission(procedureName string) string {
	switch procedureName {
	case sickrockpbconnect.SickRockResetUserPasswordProcedure:
		return rbac.PermissionUsersResetPassword
	case sickrockpbconnect.SickRockListUsersProcedure,
		sickrockpbconnect.SickRockGetUserProcedure,
		sickrockpbconnect.SickRockGetUserEffectiveRoleGrantsProcedure:
		return rbac.PermissionUsersView
	case sickrockpbconnect.SickRockCreateUserProcedure:
		return rbac.PermissionUsersCreate
	case sickrockpbconnect.SickRockDeleteUserProcedure:
		return rbac.PermissionUsersDelete
	case sickrockpbconnect.SickRockGetMyPermissionsAuditProcedure:
		// Self-service: any authenticated user may inspect their own effective permissions.
		return ""
	case sickrockpbconnect.SickRockListRbacPermissionsProcedure,
		sickrockpbconnect.SickRockListRbacRolesProcedure,
		sickrockpbconnect.SickRockGetUserRbacRolesProcedure,
		sickrockpbconnect.SickRockGetUserGroupRbacRolesProcedure,
		sickrockpbconnect.SickRockListGroupTableRoleGrantsProcedure,
		sickrockpbconnect.SickRockListTableShareGrantsProcedure,
		sickrockpbconnect.SickRockGetRbacRoleUsersProcedure,
		sickrockpbconnect.SickRockGetRbacRoleGroupsProcedure:
		return rbac.PermissionRbacView
	case sickrockpbconnect.SickRockCreateRbacRoleProcedure,
		sickrockpbconnect.SickRockUpdateRbacRoleProcedure,
		sickrockpbconnect.SickRockDeleteRbacRoleProcedure,
		sickrockpbconnect.SickRockSetUserGroupRbacRolesProcedure,
		sickrockpbconnect.SickRockSetGroupTableRoleGrantsProcedure,
		sickrockpbconnect.SickRockSetTableShareGrantsProcedure:
		return rbac.PermissionRbacManage
	case sickrockpbconnect.SickRockGetTableAccessProcedure:
		return PermissionAppRead
	case sickrockpbconnect.SickRockListUserGroupsProcedure,
		sickrockpbconnect.SickRockGetUserGroupMembersProcedure:
		return rbac.PermissionUserGroupsView
	case sickrockpbconnect.SickRockCreateUserGroupProcedure,
		sickrockpbconnect.SickRockDeleteUserGroupProcedure,
		sickrockpbconnect.SickRockSetUserGroupMembersProcedure:
		return rbac.PermissionUserGroupsManage
	case sickrockpbconnect.SickRockCreateTableProcedure,
		sickrockpbconnect.SickRockCreateTableConfigurationProcedure,
		sickrockpbconnect.SickRockAddTableColumnProcedure,
		sickrockpbconnect.SickRockChangeColumnTypeProcedure,
		sickrockpbconnect.SickRockChangeColumnNameProcedure,
		sickrockpbconnect.SickRockDropColumnProcedure,
		sickrockpbconnect.SickRockCreateForeignKeyProcedure,
		sickrockpbconnect.SickRockGetDatabaseTablesProcedure,
		sickrockpbconnect.SickRockGetSystemInfoProcedure:
		return rbac.PermissionSystemSettings
	case sickrockpbconnect.SickRockGetReadOnlyCalendarExportProcedure,
		sickrockpbconnect.SickRockListAccessibleReadOnlyExportsProcedure:
		return PermissionExportsView
	case sickrockpbconnect.SickRockSetWorkflowNavigationMembersProcedure:
		return PermissionWorkflowStart
	case sickrockpbconnect.SickRockGetAPIKeysProcedure,
		sickrockpbconnect.SickRockCreateAPIKeyProcedure,
		sickrockpbconnect.SickRockUpdateAPIKeyProcedure,
		sickrockpbconnect.SickRockDeleteAPIKeyProcedure,
		sickrockpbconnect.SickRockDeactivateAPIKeyProcedure:
		return PermissionAPIKeysUse
	case sickrockpbconnect.SickRockClaimDeviceCodeProcedure:
		return PermissionDeviceCodeClaim
	}
	if writeProcedures[procedureName] {
		return rbac.PermissionAppAccess
	}
	return PermissionAppRead
}
