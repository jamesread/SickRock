package iam

import (
	sickrockpbconnect "github.com/jamesread/SickRock/gen/sickrockpbconnect"
	"github.com/jamesread/armature-iam/rbac"
)

var writeProcedures = map[string]bool{
	sickrockpbconnect.SickRockResetUserPasswordProcedure:                  true,
	sickrockpbconnect.SickRockChangePasswordProcedure:                     true,
	sickrockpbconnect.SickRockCreateUserProcedure:                         true,
	sickrockpbconnect.SickRockDeleteUserProcedure:                         true,
	sickrockpbconnect.SickRockCreateRbacRoleProcedure:                     true,
	sickrockpbconnect.SickRockUpdateRbacRoleProcedure:                     true,
	sickrockpbconnect.SickRockDeleteRbacRoleProcedure:                     true,
	sickrockpbconnect.SickRockSetUserGroupRbacRolesProcedure:              true,
	sickrockpbconnect.SickRockSetGroupTableRoleGrantsProcedure:            true,
	sickrockpbconnect.SickRockSetTableShareGrantsProcedure:                true,
	sickrockpbconnect.SickRockCreateUserGroupProcedure:                    true,
	sickrockpbconnect.SickRockDeleteUserGroupProcedure:                    true,
	sickrockpbconnect.SickRockSetUserGroupMembersProcedure:                true,
	sickrockpbconnect.SickRockCreateTableProcedure:                        true,
	sickrockpbconnect.SickRockCreateTableConfigurationProcedure:           true,
	sickrockpbconnect.SickRockCreateDashboardProcedure:                    true,
	sickrockpbconnect.SickRockCreateItemProcedure:                         true,
	sickrockpbconnect.SickRockAddItemQuantityProcedure:                    true,
	sickrockpbconnect.SickRockUploadImageProcedure:                        true,
	sickrockpbconnect.SickRockEditItemProcedure:                           true,
	sickrockpbconnect.SickRockDeleteItemProcedure:                         true,
	sickrockpbconnect.SickRockAddTableColumnProcedure:                     true,
	sickrockpbconnect.SickRockCreateTableViewProcedure:                    true,
	sickrockpbconnect.SickRockUpdateTableViewProcedure:                    true,
	sickrockpbconnect.SickRockDeleteTableViewProcedure:                    true,
	sickrockpbconnect.SickRockCreateForeignKeyProcedure:                   true,
	sickrockpbconnect.SickRockDeleteForeignKeyProcedure:                   true,
	sickrockpbconnect.SickRockUpdateTableRelationProcedure:                true,
	sickrockpbconnect.SickRockSetItemPageLayoutProcedure:                  true,
	sickrockpbconnect.SickRockSetItemFieldGroupsProcedure:                 true,
	sickrockpbconnect.SickRockSetColumnDisplaySettingsProcedure:           true,
	sickrockpbconnect.SickRockSetTableRowLabelsProcedure:                  true,
	sickrockpbconnect.SickRockChangeColumnTypeProcedure:                   true,
	sickrockpbconnect.SickRockDropColumnProcedure:                         true,
	sickrockpbconnect.SickRockChangeColumnNameProcedure:                   true,
	sickrockpbconnect.SickRockCreateDashboardComponentRuleProcedure:       true,
	sickrockpbconnect.SickRockCreateUserBookmarkProcedure:                 true,
	sickrockpbconnect.SickRockDeleteUserBookmarkProcedure:                 true,
	sickrockpbconnect.SickRockCreateConditionalFormattingRuleProcedure:    true,
	sickrockpbconnect.SickRockUpdateConditionalFormattingRuleProcedure:    true,
	sickrockpbconnect.SickRockDeleteConditionalFormattingRuleProcedure:    true,
	sickrockpbconnect.SickRockCreateUserNotificationChannelProcedure:      true,
	sickrockpbconnect.SickRockUpdateUserNotificationChannelProcedure:      true,
	sickrockpbconnect.SickRockDeleteUserNotificationChannelProcedure:      true,
	sickrockpbconnect.SickRockCreateUserNotificationSubscriptionProcedure: true,
	sickrockpbconnect.SickRockDeleteUserNotificationSubscriptionProcedure: true,
	sickrockpbconnect.SickRockSetTickListCompletionProcedure:              true,
	sickrockpbconnect.SickRockClearTickListStateProcedure:                 true,
}

// procedurePermissions maps Connect procedures to the RBAC permission they require. Procedures
// not listed require app access when they write (writeProcedures) and app read otherwise.
var procedurePermissions = map[string]string{
	sickrockpbconnect.SickRockResetUserPasswordProcedure:          rbac.PermissionUsersResetPassword,
	sickrockpbconnect.SickRockListUsersProcedure:                  rbac.PermissionUsersView,
	sickrockpbconnect.SickRockGetUserProcedure:                    rbac.PermissionUsersView,
	sickrockpbconnect.SickRockGetUserEffectiveRoleGrantsProcedure: rbac.PermissionUsersView,
	sickrockpbconnect.SickRockCreateUserProcedure:                 rbac.PermissionUsersCreate,
	sickrockpbconnect.SickRockDeleteUserProcedure:                 rbac.PermissionUsersDelete,
	// Self-service: any authenticated user may inspect their own effective permissions.
	sickrockpbconnect.SickRockGetMyPermissionsAuditProcedure:         "",
	sickrockpbconnect.SickRockListRbacPermissionsProcedure:           rbac.PermissionRbacView,
	sickrockpbconnect.SickRockListRbacRolesProcedure:                 rbac.PermissionRbacView,
	sickrockpbconnect.SickRockGetUserRbacRolesProcedure:              rbac.PermissionRbacView,
	sickrockpbconnect.SickRockGetUserGroupRbacRolesProcedure:         rbac.PermissionRbacView,
	sickrockpbconnect.SickRockListGroupTableRoleGrantsProcedure:      rbac.PermissionRbacView,
	sickrockpbconnect.SickRockListTableShareGrantsProcedure:          rbac.PermissionRbacView,
	sickrockpbconnect.SickRockGetRbacRoleUsersProcedure:              rbac.PermissionRbacView,
	sickrockpbconnect.SickRockGetRbacRoleGroupsProcedure:             rbac.PermissionRbacView,
	sickrockpbconnect.SickRockCreateRbacRoleProcedure:                rbac.PermissionRbacManage,
	sickrockpbconnect.SickRockUpdateRbacRoleProcedure:                rbac.PermissionRbacManage,
	sickrockpbconnect.SickRockDeleteRbacRoleProcedure:                rbac.PermissionRbacManage,
	sickrockpbconnect.SickRockSetUserGroupRbacRolesProcedure:         rbac.PermissionRbacManage,
	sickrockpbconnect.SickRockSetGroupTableRoleGrantsProcedure:       rbac.PermissionRbacManage,
	sickrockpbconnect.SickRockSetTableShareGrantsProcedure:           rbac.PermissionRbacManage,
	sickrockpbconnect.SickRockGetTableAccessProcedure:                PermissionAppRead,
	sickrockpbconnect.SickRockListUserGroupsProcedure:                rbac.PermissionUserGroupsView,
	sickrockpbconnect.SickRockGetUserGroupMembersProcedure:           rbac.PermissionUserGroupsView,
	sickrockpbconnect.SickRockCreateUserGroupProcedure:               rbac.PermissionUserGroupsManage,
	sickrockpbconnect.SickRockDeleteUserGroupProcedure:               rbac.PermissionUserGroupsManage,
	sickrockpbconnect.SickRockSetUserGroupMembersProcedure:           rbac.PermissionUserGroupsManage,
	sickrockpbconnect.SickRockCreateTableProcedure:                   rbac.PermissionSystemSettings,
	sickrockpbconnect.SickRockCreateTableConfigurationProcedure:      rbac.PermissionSystemSettings,
	sickrockpbconnect.SickRockCreateDashboardProcedure:               rbac.PermissionSystemSettings,
	sickrockpbconnect.SickRockAddTableColumnProcedure:                rbac.PermissionSystemSettings,
	sickrockpbconnect.SickRockChangeColumnTypeProcedure:              rbac.PermissionSystemSettings,
	sickrockpbconnect.SickRockChangeColumnNameProcedure:              rbac.PermissionSystemSettings,
	sickrockpbconnect.SickRockDropColumnProcedure:                    rbac.PermissionSystemSettings,
	sickrockpbconnect.SickRockCreateForeignKeyProcedure:              rbac.PermissionSystemSettings,
	sickrockpbconnect.SickRockGetDatabaseTablesProcedure:             rbac.PermissionSystemSettings,
	sickrockpbconnect.SickRockGetSystemInfoProcedure:                 rbac.PermissionSystemSettings,
	sickrockpbconnect.SickRockListHealthIssuesProcedure:              rbac.PermissionSystemSettings,
	sickrockpbconnect.SickRockGetHealthSummaryProcedure:              rbac.PermissionSystemSettings,
	sickrockpbconnect.SickRockRunHealthChecksProcedure:               rbac.PermissionSystemSettings,
	sickrockpbconnect.SickRockAcknowledgeHealthIssueProcedure:        rbac.PermissionSystemSettings,
	sickrockpbconnect.SickRockGetReadOnlyCalendarExportProcedure:     PermissionExportsView,
	sickrockpbconnect.SickRockListAccessibleReadOnlyExportsProcedure: PermissionExportsView,
	sickrockpbconnect.SickRockSetWorkflowNavigationMembersProcedure:  PermissionWorkflowStart,
	sickrockpbconnect.SickRockGetAPIKeysProcedure:                    PermissionAPIKeysUse,
	sickrockpbconnect.SickRockCreateAPIKeyProcedure:                  PermissionAPIKeysUse,
	sickrockpbconnect.SickRockUpdateAPIKeyProcedure:                  PermissionAPIKeysUse,
	sickrockpbconnect.SickRockDeleteAPIKeyProcedure:                  PermissionAPIKeysUse,
	sickrockpbconnect.SickRockDeactivateAPIKeyProcedure:              PermissionAPIKeysUse,
	sickrockpbconnect.SickRockClaimDeviceCodeProcedure:               PermissionDeviceCodeClaim,
}

// RequiredPermission maps Connect procedures to RBAC permission names.
func RequiredPermission(procedureName string) string {
	if permission, ok := procedurePermissions[procedureName]; ok {
		return permission
	}
	if writeProcedures[procedureName] {
		return rbac.PermissionAppAccess
	}
	return PermissionAppRead
}
