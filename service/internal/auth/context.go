package auth

import "context"

// contextKey type for API key read-only flag to avoid key collisions
type contextKey string

const (
	// ContextKeyAPIKeyReadOnly is set to true when the request is authenticated via a read-only API key
	ContextKeyAPIKeyReadOnly contextKey = "api_key_read_only"
)

// IsAPIKeyReadOnly returns true if the context indicates a read-only API key was used for auth
func IsAPIKeyReadOnly(ctx context.Context) bool {
	v, _ := ctx.Value(ContextKeyAPIKeyReadOnly).(bool)
	return v
}

// writeProcedures is the set of RPC procedure paths that require read-write access (not read-only)
var writeProcedures = map[string]bool{
	"/sickrock.SickRock/Logout": true,
	"/sickrock.SickRock/ResetUserPassword": true,
	"/sickrock.SickRock/ClaimDeviceCode": true,
	"/sickrock.SickRock/CreateTable": true,
	"/sickrock.SickRock/CreateTableConfiguration": true,
	"/sickrock.SickRock/CreateItem": true,
	"/sickrock.SickRock/EditItem": true,
	"/sickrock.SickRock/DeleteItem": true,
	"/sickrock.SickRock/AddTableColumn": true,
	"/sickrock.SickRock/CreateTableView": true,
	"/sickrock.SickRock/UpdateTableView": true,
	"/sickrock.SickRock/DeleteTableView": true,
	"/sickrock.SickRock/CreateForeignKey": true,
	"/sickrock.SickRock/DeleteForeignKey": true,
	"/sickrock.SickRock/ChangeColumnType": true,
	"/sickrock.SickRock/DropColumn": true,
	"/sickrock.SickRock/ChangeColumnName": true,
	"/sickrock.SickRock/CreateDashboardComponentRule": true,
	"/sickrock.SickRock/CreateUserBookmark": true,
	"/sickrock.SickRock/DeleteUserBookmark": true,
	"/sickrock.SickRock/CreateAPIKey": true,
	"/sickrock.SickRock/UpdateAPIKey": true,
	"/sickrock.SickRock/DeleteAPIKey": true,
	"/sickrock.SickRock/DeactivateAPIKey": true,
	"/sickrock.SickRock/CreateConditionalFormattingRule": true,
	"/sickrock.SickRock/UpdateConditionalFormattingRule": true,
	"/sickrock.SickRock/DeleteConditionalFormattingRule": true,
	"/sickrock.SickRock/CreateUserNotificationChannel": true,
	"/sickrock.SickRock/UpdateUserNotificationChannel": true,
	"/sickrock.SickRock/DeleteUserNotificationChannel": true,
	"/sickrock.SickRock/CreateUserNotificationSubscription": true,
	"/sickrock.SickRock/DeleteUserNotificationSubscription": true,
}

// IsWriteProcedure returns true if the procedure is a mutating (write) RPC
func IsWriteProcedure(procedure string) bool {
	return writeProcedures[procedure]
}
