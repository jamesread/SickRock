// Package mcp provides an MCP (Model Context Protocol) server exposed as an HTTP endpoint.
// It exposes SickRock API operations as MCP tools, using the same auth as the Connect API.
package mcp

import (
	"context"
	"encoding/json"
	"net/http"

	"connectrpc.com/connect"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	sickrockpb "github.com/jamesread/SickRock/gen/proto"
	"github.com/jamesread/SickRock/internal/auth"
	srvpkg "github.com/jamesread/SickRock/internal/server"
)

// NewHandler returns an http.Handler that serves the MCP Streamable HTTP endpoint.
// It should be mounted at /mcp. The caller is responsible for running auth middleware
// before this handler so that the request context contains the authenticated user.
func NewHandler(srv *srvpkg.SickRockServer) http.Handler {
	mcpServer := server.NewMCPServer(
		"SickRock",
		"1.0.0",
		server.WithToolCapabilities(false),
		server.WithRecovery(),
	)

	// Ping
	mcpServer.AddTool(mcp.NewTool("sickrock_ping",
		mcp.WithDescription("Check connectivity to the SickRock API (health check)."),
		mcp.WithString("message", mcp.Description("Optional message to echo")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handlePing(ctx, srv, req)
	})

	// Navigation
	mcpServer.AddTool(mcp.NewTool("sickrock_get_navigation",
		mcp.WithDescription("Get the navigation tree (pages, workflows, bookmarks) from SickRock."),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleGetNavigation(ctx, srv, req)
	})

	mcpServer.AddTool(mcp.NewTool("sickrock_get_table_configurations",
		mcp.WithDescription("List all table configurations (pages) available in SickRock."),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleGetTableConfigurations(ctx, srv, req)
	})

	mcpServer.AddTool(mcp.NewTool("sickrock_get_database_tables",
		mcp.WithDescription("List tables in a database. Use the database name (e.g. 'main' for SQLite)."),
		mcp.WithString("database", mcp.Required(), mcp.Description("Database name")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleGetDatabaseTables(ctx, srv, req)
	})

	mcpServer.AddTool(mcp.NewTool("sickrock_get_table_structure",
		mcp.WithDescription("Get the structure (fields, types, foreign keys) of a table. page_id is the table config name or table identifier."),
		mcp.WithString("page_id", mcp.Required(), mcp.Description("Table/page ID (table configuration name or table name)")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleGetTableStructure(ctx, srv, req)
	})

	mcpServer.AddTool(mcp.NewTool("sickrock_list_items",
		mcp.WithDescription("List items from a table. tc_name is the table configuration name (e.g. from get_table_configurations). Optionally pass where as JSON object of column->value for exact-match filters."),
		mcp.WithString("tc_name", mcp.Required(), mcp.Description("Table configuration name")),
		mcp.WithString("where", mcp.Description("Optional JSON object of column name to value for filtering, e.g. {\"status\":\"active\"}")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleListItems(ctx, srv, req)
	})

	mcpServer.AddTool(mcp.NewTool("sickrock_get_item",
		mcp.WithDescription("Get a single item by ID from a table."),
		mcp.WithString("page_id", mcp.Required(), mcp.Description("Table/page ID")),
		mcp.WithString("id", mcp.Required(), mcp.Description("Item ID")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleGetItem(ctx, srv, req)
	})

	mcpServer.AddTool(mcp.NewTool("sickrock_create_item",
		mcp.WithDescription("Create a new item in a table. additional_fields is a JSON object of column name to value."),
		mcp.WithString("page_id", mcp.Required(), mcp.Description("Table/page ID")),
		mcp.WithString("additional_fields", mcp.Required(), mcp.Description("JSON object of column names to values, e.g. {\"name\":\"Foo\",\"count\":42}")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleCreateItem(ctx, srv, req)
	})

	mcpServer.AddTool(mcp.NewTool("sickrock_edit_item",
		mcp.WithDescription("Update an existing item. additional_fields is a JSON object of column name to new value."),
		mcp.WithString("page_id", mcp.Required(), mcp.Description("Table/page ID")),
		mcp.WithString("id", mcp.Required(), mcp.Description("Item ID")),
		mcp.WithString("additional_fields", mcp.Required(), mcp.Description("JSON object of column names to new values")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleEditItem(ctx, srv, req)
	})

	mcpServer.AddTool(mcp.NewTool("sickrock_delete_item",
		mcp.WithDescription("Delete an item from a table."),
		mcp.WithString("page_id", mcp.Required(), mcp.Description("Table/page ID")),
		mcp.WithString("id", mcp.Required(), mcp.Description("Item ID")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleDeleteItem(ctx, srv, req)
	})

	return server.NewStreamableHTTPServer(mcpServer, server.WithEndpointPath("/mcp"))
}

func jsonResult(v any) (*mcp.CallToolResult, error) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(string(b)), nil
}

func handlePing(ctx context.Context, srv *srvpkg.SickRockServer, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	msg := req.GetString("message", "")
	res, err := srv.Ping(ctx, connect.NewRequest(&sickrockpb.PingRequest{Message: msg}))
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return jsonResult(map[string]any{
		"message":        res.Msg.GetMessage(),
		"timestamp_unix": res.Msg.GetTimestampUnix(),
	})
}

func handleGetNavigation(ctx context.Context, srv *srvpkg.SickRockServer, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	res, err := srv.GetNavigation(ctx, connect.NewRequest(&sickrockpb.GetNavigationRequest{}))
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	out := struct {
		Items     []*sickrockpb.NavigationItem   `json:"items"`
		Bookmarks []*sickrockpb.UserBookmark    `json:"bookmarks"`
		Workflows []*sickrockpb.Workflow        `json:"workflows"`
	}{res.Msg.GetItems(), res.Msg.GetBookmarks(), res.Msg.GetWorkflows()}
	return jsonResult(out)
}

func handleGetTableConfigurations(ctx context.Context, srv *srvpkg.SickRockServer, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	res, err := srv.GetTableConfigurations(ctx, connect.NewRequest(&sickrockpb.GetTableConfigurationsRequest{}))
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return jsonResult(map[string]any{"pages": res.Msg.GetPages()})
}

func handleGetDatabaseTables(ctx context.Context, srv *srvpkg.SickRockServer, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	db, err := req.RequireString("database")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	res, err := srv.GetDatabaseTables(ctx, connect.NewRequest(&sickrockpb.GetDatabaseTablesRequest{Database: db}))
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return jsonResult(map[string]any{"tables": res.Msg.GetTables()})
}

func handleGetTableStructure(ctx context.Context, srv *srvpkg.SickRockServer, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	pageID, err := req.RequireString("page_id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	res, err := srv.GetTableStructure(ctx, connect.NewRequest(&sickrockpb.GetTableStructureRequest{PageId: pageID}))
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	out := map[string]any{
		"fields":           res.Msg.GetFields(),
		"createButtonText": res.Msg.GetCreateButtonText(),
		"view":             res.Msg.GetView(),
		"foreignKeys":      res.Msg.GetForeignKeys(),
	}
	return jsonResult(out)
}

func handleListItems(ctx context.Context, srv *srvpkg.SickRockServer, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	tcName, err := req.RequireString("tc_name")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	r := &sickrockpb.ListItemsRequest{TcName: tcName}
	if w := req.GetString("where", ""); w != "" {
		var where map[string]string
		if err := json.Unmarshal([]byte(w), &where); err != nil {
			return mcp.NewToolResultError("invalid where JSON: " + err.Error()), nil
		}
		r.Where = where
	}
	res, err := srv.ListItems(ctx, connect.NewRequest(r))
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return jsonResult(map[string]any{"items": res.Msg.GetItems()})
}

func handleGetItem(ctx context.Context, srv *srvpkg.SickRockServer, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	pageID, err := req.RequireString("page_id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	id, err := req.RequireString("id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	res, err := srv.GetItem(ctx, connect.NewRequest(&sickrockpb.GetItemRequest{PageId: pageID, Id: id}))
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return jsonResult(res.Msg.GetItem())
}

func handleCreateItem(ctx context.Context, srv *srvpkg.SickRockServer, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if auth.IsAPIKeyReadOnly(ctx) {
		return mcp.NewToolResultError("permission denied: read-only API key cannot create items"), nil
	}
	pageID, err := req.RequireString("page_id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	fieldsStr, err := req.RequireString("additional_fields")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	var fields map[string]string
	if err := json.Unmarshal([]byte(fieldsStr), &fields); err != nil {
		return mcp.NewToolResultError("invalid additional_fields JSON: " + err.Error()), nil
	}
	res, err := srv.CreateItem(ctx, connect.NewRequest(&sickrockpb.CreateItemRequest{
		PageId:           pageID,
		AdditionalFields: fields,
	}))
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return jsonResult(res.Msg.GetItem())
}

func handleEditItem(ctx context.Context, srv *srvpkg.SickRockServer, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if auth.IsAPIKeyReadOnly(ctx) {
		return mcp.NewToolResultError("permission denied: read-only API key cannot edit items"), nil
	}
	pageID, err := req.RequireString("page_id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	id, err := req.RequireString("id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	fieldsStr, err := req.RequireString("additional_fields")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	var fields map[string]string
	if err := json.Unmarshal([]byte(fieldsStr), &fields); err != nil {
		return mcp.NewToolResultError("invalid additional_fields JSON: " + err.Error()), nil
	}
	res, err := srv.EditItem(ctx, connect.NewRequest(&sickrockpb.EditItemRequest{
		PageId:           pageID,
		Id:               id,
		AdditionalFields: fields,
	}))
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return jsonResult(res.Msg.GetItem())
}

func handleDeleteItem(ctx context.Context, srv *srvpkg.SickRockServer, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if auth.IsAPIKeyReadOnly(ctx) {
		return mcp.NewToolResultError("permission denied: read-only API key cannot delete items"), nil
	}
	pageID, err := req.RequireString("page_id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	id, err := req.RequireString("id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	res, err := srv.DeleteItem(ctx, connect.NewRequest(&sickrockpb.DeleteItemRequest{PageId: pageID, Id: id}))
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return jsonResult(map[string]any{"deleted": res.Msg.GetDeleted()})
}
