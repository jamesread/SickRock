# SickRock MCP Server

An [MCP](https://modelcontextprotocol.io) (Model Context Protocol) server that exposes SickRock API operations as tools. It connects to a SickRock instance and authenticates using **API keys**.

## Requirements

- A SickRock instance running and reachable (e.g. `http://localhost:8080`)
- An API key created in SickRock (Settings → API Keys)

## Configuration

| Variable          | Required | Default                     | Description                          |
|-------------------|----------|-----------------------------|--------------------------------------|
| `SICKROCK_API_KEY` | Yes      | —                           | API key for SickRock authentication  |
| `SICKROCK_API_URL` | No       | `http://localhost:8080/api` | Base URL of the SickRock API         |

## Build and run

```bash
# From the service directory
cd service
go build -o sickrock-mcp ./cmd/sickrock-mcp/

# Run (stdio transport; used by MCP clients like Cursor)
export SICKROCK_API_KEY=your-api-key
export SICKROCK_API_URL=http://localhost:8080/api   # optional
./sickrock-mcp
```

## Cursor configuration

Add the server to your Cursor MCP settings (e.g. `~/.cursor/mcp.json` or project `.cursor/mcp.json`):

```json
{
  "mcpServers": {
    "sickrock": {
      "command": "/absolute/path/to/sickrock-mcp",
      "env": {
        "SICKROCK_API_KEY": "your-api-key",
        "SICKROCK_API_URL": "http://localhost:8080/api"
      }
    }
  }
}
```

Use the full path to the `sickrock-mcp` binary. Restart Cursor (or reload MCP) after changing config.

## Tools

| Tool | Description |
|------|-------------|
| `sickrock_ping` | Health check; optional message to echo |
| `sickrock_get_navigation` | Navigation tree (pages, workflows, bookmarks) |
| `sickrock_get_table_configurations` | List table configurations (pages) |
| `sickrock_get_database_tables` | List tables in a database (`database` name required) |
| `sickrock_get_table_structure` | Fields, types, foreign keys for a table (`page_id` required) |
| `sickrock_list_items` | List items from a table (`tc_name`; optional `where` JSON) |
| `sickrock_get_item` | Get one item by `page_id` and `id` |
| `sickrock_create_item` | Create item; `page_id` and `additional_fields` (JSON) |
| `sickrock_edit_item` | Update item; `page_id`, `id`, `additional_fields` (JSON) |
| `sickrock_delete_item` | Delete item by `page_id` and `id` |

All tools return JSON. Errors from the SickRock API are returned as tool errors.
