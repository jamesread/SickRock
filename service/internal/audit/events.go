package audit

const (
	EventLogin       = "auth.login"
	EventLoginFailed = "auth.login_failed"
	EventLogout      = "auth.logout"
	EventTableView   = "table.view"
	EventTableInsert = "table.insert"
	EventTableEdit   = "table.edit"
	EventTableDelete = "table.delete"
	EventTableDenied = "table.denied"
	EventExportView  = "export.view"
)

const TableConfiguration = "table_logs"
