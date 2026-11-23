# AGENTS.md

This document outlines the AI agents and automated systems that can be used to work with the SickRock project.

## Project Overview

SickRock is a no-code database web application builder that allows users to create web applications on top of real databases. It's built with:

- **Backend**: Go with gRPC/Connect protocol
- **Frontend**: Vue.js 3 with TypeScript
- **Database**: SQLite/MySQL support
- **Architecture**: Microservices with protobuf communication

## Available Agents

### 1. Database Schema Agent
**Purpose**: Automatically analyze and manage database schemas

**Capabilities**:
- Analyze existing database tables and relationships
- Generate table configurations automatically
- Suggest optimal column types and constraints
- Create foreign key relationships
- Generate migration scripts

**Use Cases**:
- Onboard new databases into SickRock
- Optimize existing table structures
- Generate documentation for database schemas
- Suggest performance improvements

**API Endpoints**:
- `GetDatabaseTables` - List all tables in a database
- `CreateTableConfiguration` - Create new table configurations
- `GetTableStructure` - Analyze table structure
- `AddTableColumn` - Add new columns to tables

### 2. UI Generation Agent
**Purpose**: Automatically generate user interfaces for database tables

**Capabilities**:
- Create table views with optimal column visibility
- Generate forms for data entry
- Create dashboard components
- Suggest UI layouts based on data types
- Generate responsive designs

**Use Cases**:
- Auto-generate CRUD interfaces for new tables
- Create custom views for specific use cases
- Generate dashboard layouts
- Optimize user experience based on data patterns

**API Endpoints**:
- `CreateTableView` - Create custom table views
- `UpdateTableView` - Modify existing views
- `GetTableViews` - List available views (includes each view's `layoutJson` and the caller's `userDefaultViewId`)
- `SetUserDefaultTableView` - Set or clear the caller's own default view for a table
- `GetDashboards` - Manage dashboard components

**View types**: `table`, `calendar`, `ticklist`, `weeklyplan` and `cards`. A **WeeklyPlan** view (`WeeklyPlanComponent.vue`, logic in `frontend/src/utils/weeklyPlan.ts`) shows a week as a calendar-style grid, with a column per day and a row per user-defined slot (days stack on phones), and is configured in the view's `layout_json` under `weeklyPlan`, e.g. `{"weeklyPlan":{"dateColumn":"datetime","slotColumn":"","itemColumn":"recipe","slots":[{"label":"Breakfast","time":"08:00","keywords":"breakfast"},{"label":"Lunch","time":"12:30"},{"label":"Dinner","time":"18:00"}],"weekStartsOn":1}}`; a slot's optional `keywords` fill in the picker's search when adding to that slot. `notifyUserIds` lists the users the toolbar's **Notify…** dialog ticks by default (see *User-to-user notifications*); its link opens the view on that week with `?view={viewId}&week=YYYY-MM-DD` (TableView reads `view`, the weekly plan reads `week`). A row's day comes from `dateColumn`; its slot from `slotColumn` (matched against slot labels) or, when that is empty, from the time of day in `dateColumn` (the last slot whose time is not after it). `itemColumn` fills the slot; when it is a relation, entries show the referenced row's name and first image column, and new entries are picked from that table in a dialog of image tiles (`RowPickerDialog.vue`); when that table has a Cards view, the tiles use its image column and show its detail columns, and the search covers the name and those details, whose **Create <row name>** button (when the user may insert into that table and it has no `create_delegate`) shows the table's insert form in the dialog, with the name filled in from the search, and puts the new row straight into the slot. Adding writes the day + slot time (and slot label), dragging an entry to another cell edits those columns, and removing deletes the row. Right-clicking an entry opens a menu (`ContextMenu.vue`, a reusable menu styled by picocrank's `nav.menu.context-menu`) with **Duplicate** (a new row copying every column except `id`, `sr_created` and `sr_updated`), **Open** and **Remove**. Rows are loaded with `ListItems` and filtered to the week client-side. A **Cards** view (`CardsComponent.vue`, logic in `frontend/src/utils/cardsView.ts`) shows rows as a grid of cards linking to each row, with a search over the title and the details shown on each card, configured under `cards` in `layout_json`, e.g. `{"cards":{"imageColumn":"main_picture","titleColumn":"name","detailColumns":["sr_created"],"size":"medium"}}`: `imageColumn` (an image column, shown across the top; empty for none), `titleColumn` (empty for the row's name/title/id), up to four `detailColumns`, and `size` (`small`, `medium`, `large`). The view editor keeps the existing `layout_json` when saving. A **Tick List** view (`TickListComponent.vue`, logic in `frontend/src/utils/tickList.ts`, settings `TickListSettings.vue`) shows rows as tiles ticked off with state shared on the server (`tick_list_state`), configured under `tickList` in `layout_json`, e.g. `{"tickList":{"labelColumn":"ingredient","detailColumns":["quantity","unit"],"sortColumns":["ingredient"],"groupColumn":"ingredient.aisle"}}`. Each setting is a column path: a column, or `relation.column` for a column of the referenced row. A relation column on its own shows the referenced row's name. Details (up to four) are joined with spaces under the label, tiles are sorted by the group column and then up to three sort columns (naturally, with empty values last), and `groupColumn` puts them under headings (rows without a value go under "Other"). With no settings, the old behaviour is kept: the name or title, in table order.

Table views are stored on the server: column visibility/order/sort in `table_view_columns`, plus filters, group-by, page size and column priorities as a JSON object in `table_views.layout_json`. The frontend passes PicoCrank Table `viewControlMode="callbackControlled"` with server-backed `viewSettings` hooks (`frontend/src/composables/useServerTableViewSettings.ts`), so views saved from PicoCrank's view menu never go to browser localStorage. Each user may pick their own default view (`table_view_user_defaults`); otherwise the table-wide `is_default` view is used.

### 3. Data Analysis Agent
**Purpose**: Analyze data patterns and provide insights

**Capabilities**:
- Analyze data distribution and patterns
- Detect anomalies in data
- Generate reports and summaries
- Suggest data quality improvements
- Create data visualizations
- Process synthetic fields for enhanced data analysis

**Use Cases**:
- Data quality assessment
- Performance analysis
- Business intelligence reporting
- Data migration planning
- Time-based analysis using relative timestamps

**API Endpoints**:
- `GetSystemInfo` - Get system statistics
- `ListItems` - Query data with filters (includes synthetic `srCreatedRelative` and `srUpdatedRelative` fields)
- `GetMostRecentlyViewed` - Analyze usage patterns

**Synthetic Fields**:
The `ListItems` API automatically includes synthetic fields to enhance data analysis:
- `srCreatedRelative` - Relative time in seconds from the current timestamp when `srCreated` is set
- `srUpdatedRelative` - Relative time in seconds from the current timestamp when `srUpdated` is set
- These fields are included as top-level fields alongside `srCreated`, `srUpdated`, and `additionalFields`
- These fields enable easy time-based filtering and analysis without requiring client-side calculations
- When tables have an `sr_updated` column, it is automatically set to the current timestamp on updates

### 4. Authentication & Security Agent
**Purpose**: Manage user authentication and security

**Capabilities**:
- Generate secure API keys
- Manage user sessions
- Implement device code authentication
- Monitor security events
- Suggest security improvements

**Use Cases**:
- Automated user management
- Security audit and compliance
- API key rotation
- Device authentication flows

**API Endpoints**:
- `CreateAPIKey` - Generate API keys
- `UpdateAPIKey` - Change a key's name, expiry, read-only flag or scope (the secret stays the same)
- `GenerateDeviceCode` - Device authentication
- `ValidateToken` - Session management
- `ResetUserPassword` - Password management

**API key scopes** (like StencilBox's): a key may be read-only, may expire (`api_keys.expires_at`), and may be limited to some of its owner's permissions (`restrict_permissions`, `api_key_permissions`) and to some tables, dashboards and workflows (`restrict_resources`, `api_key_resources`, keyed by table configuration name, dashboard name or workflow id). The owner's current access is always the ceiling. `service/internal/iam/apikeyscope*.go` holds the scope and a middleware that runs after armature-iam authenticates a request (`AuthLayer.WrapHandler`/`WrapMCPHandler`): it refuses expired keys, writes from read-only keys (anything not named `Get…`/`List…`/…), procedures whose permission the scope leaves out, and key management from any key; narrows the request's RBAC to the scope; and puts the scope in the context (`iam.APIKeyScopeFromContext`). Resource checks (`requireTableAccess`, `requireDashboardAccess`, `requireWorkflowAccess`, navigation and table discovery in `service/internal/server/server_resource_access.go`) apply it, so any new resource check should go through them. MCP tools check their Connect procedure with `iam.AuthorizeProcedure`. `GetAPIKeys` also returns the permissions and resources a scope can be chosen from; the UI is `UserAPIKeys.vue` with `ApiKeyScopeFields.vue`.

### 5. Migration Agent
**Purpose**: Automatically handle database migrations and schema changes

**Capabilities**:
- Generate migration scripts
- Handle schema changes safely
- Backup data before migrations
- Rollback failed migrations
- Validate migration integrity

**Use Cases**:
- Automated schema updates
- Data migration between databases
- Version control for database changes
- Production deployment safety

**API Endpoints**:
- `ChangeColumnType` - Modify column types
- `DropColumn` - Remove columns safely
- `ChangeColumnName` - Rename columns
- `CreateForeignKey` - Add relationships

## Home page layout

Each user can customise their own home page: in design mode, **Edit home** at the bottom of the home page (`HomeView.vue`, editor `HomeLayoutEditor.vue`, logic in `frontend/src/utils/homeLayout.ts`) reorders the sections (picocrank `ReorderList`), shows or hides each one, and chooses bookmarks to show as **Quick access** buttons. Sections: `quick-access`, `welcome`, `shared-calendars`, `workflows`, `recently-viewed`, and the extra `bookmarks` (all bookmarks) and `pages` (every sidebar page), which start hidden. The layout is stored as JSON in `user_accounts.home_layout_json` (migration 0065; NULL is the default) via **`GetHomeLayout`** / **`SetHomeLayout`** (self-service), e.g. `{"sections":[{"id":"quick-access","visible":true},{"id":"welcome","visible":false}],"quickAccess":[3,1]}`, where `quickAccess` holds bookmarked navigation item ids. Unknown sections are dropped and sections a stored layout does not mention appear in their default place, so add new sections to `HOME_SECTIONS`.

## Health checks

SickRock runs **health checks** in the background (shortly after startup, then every `health_check_interval`, default `15m`, set via `SICKROCK_HEALTH_CHECK_INTERVAL` or `config.yaml`) and shows the issues they find to admins (`system.settings`) on **Control Panel → Health** (`/admin/health`), with an issue count on the sidebar's System → Health link. Issues are stored in `health_issues` (check runs in `health_check_runs`). A new warning or error sends the opt-in `health.issue_raised` notification event once.

- **Engine:** the runner, issue store (`store/sqlstore`), logged-error hook (`errorlog`) and the generic checks (default admin password, risky settings) come from [`github.com/jamesread/armature-health`](https://github.com/jamesread/armature-health). `service/internal/health/` only holds SickRock's own checks and `DefaultChecks`; `server_health.go` maps the library's `Service` results to protobuf.
- **Adding a check:** implement `armhealth.Check` (`ID()`, `Title()`, `Run(ctx) ([]armhealth.Issue, error)`) in a new file in `service/internal/health/`, and add it to `DefaultChecks` in `checks.go`. Put database queries in `service/internal/repo/health_checks.go`. Checks every app needs belong in armature-health instead.
- **Issues:** `Key` must be stable for the same problem across runs (e.g. `nav:42`); an issue no longer returned is resolved, and one that recurs later is a new issue. Use `error` for things that break the app or security, `warning` for broken configuration, and `info` for advice; only warnings and errors are notified and counted. Set `Link` to the frontend path where it can be fixed.
- **Failures:** a check that returns an error (or panics) is recorded as failed and keeps its previous issues.
- **Logged errors:** a logrus hook (`errorlog.ErrorLog` from armature-health) groups error-level log lines by fingerprint for the `logs.recent_errors` check.
- **API:** `ListHealthIssues`, `GetHealthSummary`, `RunHealthChecks`, `AcknowledgeHealthIssue` (all require `system.settings`).

## UI usage tracking

Users can opt in to logging their own navigation, for offline analysis of navigation loops, hot spots and places to optimise: **user control panel → UI usage** (`/user-ui-usage`, `UserUiUsage.vue`), option **Track and log UI usage** (`user_accounts.ui_usage_tracking`, migration 0064). While it is on, the client (`frontend/src/composables/useUiUsageTracking.ts`, logic in `frontend/src/utils/uiUsage.ts`, installed in `App.vue`) records route changes (`router.afterEach`) and clicks on links and buttons in picocrank's sidebar (`#picocrank-sidebar`) and header (`<header>`), never page content or anything typed. Events wait in `localStorage` (per user, at most 500) and are sent with **`RecordUIUsage`** every 30 seconds and when the page is hidden. The server (`service/internal/uiusage`) appends them to `<ui_usage_dir>/user-<id>.jsonl` (`SICKROCK_UI_USAGE_DIR`; default `<configdir>/ui-usage`, `/config/ui-usage` in the container, else `../tmp/ui-usage`), keeping the newest 1000 lines. Each line is a JSON object: `ts`, `version` (the app version the client loaded, else the server's), `session` (random per page load), `kind` (`route`, `sidebar` or `header`), `from`, `to` and `label`. Events from a user who has not opted in are dropped and the client is told to stop. **`GET /api/ui-usage.jsonl`** downloads the signed-in user's own log (same auth as the API); `GetUIUsageSettings`, `SetUIUsageTracking` and `ClearUIUsageLog` manage it. All four RPCs are self-service (no permission beyond being signed in).

## Installable app (PWA)

SickRock installs as a Progressive Web App.

- **Manifest:** `GET /manifest.json` is built by the server (`service/internal/pwa`) from `frontend/public/manifest.template.json`, named after the instance's `appTitle` setting (`table_settings`), and sent as `application/manifest+json` with `Cache-Control: no-cache`. `frontend/index.html` links to it; the template itself is not served. Its `id` is fixed, so renaming an instance does not create a second app. iOS takes the name from the `apple-mobile-web-app-title` meta tag, kept in step by `syncAppTitleMeta` (`frontend/src/appTitle.ts`).
- **Service worker** (`frontend/public/sw.js`, registered with `updateViaCache: 'none'`, served with `Cache-Control: no-cache`): hashed `/assets/*` files are cache-first; page loads are network-first and refresh the cached app shell; offline, home and `/table/*` pages open from the shell and other pages show `offline.html`. Only 200 responses of the expected kind are cached, never HTML in place of a script, image or manifest. `/api/`, `/mcp`, `/openapi`, `/llms.txt` and `/metrics` bypass it. Bump `CACHE_VERSION` when the caching rules change.
- **Server:** hashed assets are sent `immutable`; pages are `no-cache`; a missing file with a static extension (`.js`, `.png`, `.json`, …) is a 404, never the SPA page.
- **Checks:** the `pwa.installable` health check reports problems with the deployed frontend (template, icon files and sizes, manifest link, `sw.js`, `offline.html`). Tests: `service/internal/pwa` (manifest building and validation against the shipped files), `service/frontend_routes_test.go` (routes and cache headers), `frontend/src/sw.test.ts` (the service worker run against a fake browser) and `integration-tests/tests/pwa.spec.js` (Chrome's installability verdict). When changing icons or the template, keep the declared sizes equal to the real image sizes; the tests check them.

## User-to-user notifications

One user can notify others, e.g. that a week's plan is ready (**Notify…** on a WeeklyPlan view, `NotifyUsersDialog.vue`).

- **Who:** senders need `notifications.send` (given to the `member` and `superuser` roles) and may notify only users who share an IAM group with them (`ListNotifiableUsers`, `repo.ListNotifiableUsers`). Everyone is in **Everyone** by default, so remove users from it to limit this.
- **Delivery:** each recipient gets the `user.notified` event (*Notified by another user*) on the channels they subscribed to it on their Notifications page, never anyone else's: `NotificationService.SendToUser`. Events in `targetedEvents` (`service/internal/notifications`) are refused by the broadcast `SendNotification`. The message is `sender: title`, then the message and an absolute link.
- **API:** `NotifyUsers` (`recipient_user_ids`, `title`, optional `message` and `link_path`) returns per recipient `sent`, `not_subscribed` or `failed`. Limits: 1–20 recipients, a 200-character title, a 2000-character message, a link that is a path on this site, and 30 recipients per sender per hour (in memory). The link's origin comes from the request (`WithRequestOrigin`, honouring `X-Forwarded-Proto`/`X-Forwarded-Host`). MCP: `sickrock_list_notifiable_users`, `sickrock_notify_users` (recipients by username).
- Notification channels and subscriptions reference `user_accounts` (migration 0063; they referenced the legacy `table_users` before, so users created since armature-iam could not add channels).

## Rollups

A **rollup** gathers the detail rows owned by a set of source rows, groups and sums them, and **replaces every row** of a target table with the result. It is generic. For example, from a meal plan (source) through each planned recipe's `recipe_ingredients` (detail) to a `shopping_list` (target); or packing lists from planned trips, or parts from planned builds.

- **Definitions:** the `rollups` table (migration 0066), managed as the system table **`table_rollups`** (Control Panel → Rollups; `system.settings`). Its columns are `name`, `source_table`, `source_link_column` (the source column holding the owner id the details reference; empty when the details reference the source rows themselves), `detail_table`, `detail_link_column`, `group_columns_json` (e.g. `["ingredient","unit"]`), `sum_columns_json` (e.g. `["quantity"]`), `target_table`, `column_map_json` (optional, detail column → target column; same name otherwise) and `count_column` (optional, the target column for how many source rows fed each group).
- **Running** (`service/internal/repo/rollups.go`, in one transaction): read the matching source rows; count each owner once per source row, so a recipe planned twice doubles its ingredients; read the detail rows of those owners; group them, summing as decimals; `DELETE` every target row; insert one row per group, sorted by the group values; and clear the target's tick-list state. Detail rows whose group columns are all empty are left out, and so are values that are not numbers (these are reported in `skipped`). Every column name is checked against the real tables before any SQL runs. A system table, or the rollup's own source or detail table, is refused as a target.
- **API:** `ListRollups` (`source_table` optional; lists only rollups whose source the caller may view) and `RunRollup` (`rollup_id`, plus `date_column` with wall-clock `from`/`to` bounds, end exclusive, exact-match `where` filters, and optional `source_ids` primary keys with `limit_to_source_ids`; all of these only narrow the rollup's own source rows). `RunRollup` needs the **`rollup.run`** permission (given to `member` and `superuser`), view access to the source and detail tables, and insert **and** delete access to the target. Read-only API keys cannot call it. MCP: `sickrock_list_rollups`, `sickrock_run_rollup`.
- **UI:** `RollupButtons.vue` shows a button for each rollup that starts from the table, on the table toolbar and on WeeklyPlan views. From the table toolbar a run covers every row, or only the rows the table's current filters show: those are matched in the browser the way picocrank's Table matches them (`filteredRowIds` in `frontend/src/utils/rollups.ts`) and sent as `source_ids` with `limit_to_source_ids`, at most 20,000. From a WeeklyPlan view a run covers the shown week, using the view's `dateColumn`. The button opens a confirmation dialog, and the result links to the target table.
- **Shopping-list example** (shop from a Tick List view of `shopping_list` labelled by `ingredient`, with details `quantity`, `unit` and grouped by `ingredient.aisle`): `{"name":"Shopping list","source_table":"meal_plan","source_link_column":"recipe","detail_table":"recipe_ingredients","detail_link_column":"recipe","group_columns_json":"[\"ingredient\",\"unit\"]","sum_columns_json":"[\"quantity\"]","target_table":"shopping_list"}`. Here `shopping_list` has `ingredient` (a relation to `ingredients`), `unit` and a decimal `quantity`.

## MCP Server

An **MCP (Model Context Protocol)** server is exposed as an HTTP endpoint on the same SickRock server so AI assistants (e.g. Cursor, Claude Desktop) can call the SickRock API via tools.

- **Endpoint**: `GET/POST /mcp` on the same base URL as the app (e.g. `https://your-sickrock-instance/mcp` when running in Docker).
- **Auth**: Same as the Connect API — **API key** (Bearer token) or **session** (Session-Token header). No separate process or config.
- **Transport**: MCP Streamable HTTP; point your MCP client at `https://your-host/mcp`.
- **Tools**: `sickrock_ping`, `sickrock_get_navigation`, `sickrock_get_table_configurations`, `sickrock_get_database_tables`, `sickrock_get_table_structure`, `sickrock_list_items`, `sickrock_get_item`, `sickrock_create_item`, `sickrock_edit_item`, `sickrock_delete_item`, `sickrock_list_notifiable_users`, `sickrock_notify_users`, `sickrock_list_rollups`, `sickrock_run_rollup`

## Read-only calendar exports

Table and workflow access uses global permissions (`table.view`, `table.insert`, `table.edit`, `table.delete`, `workflow.view`, `workflow.start`, `dashboard.view`) plus per-resource rows in **`rbac_resource_grants`** (scoped by IAM group). **`GetNavigation`** omits links the user cannot view. The system **`viewer`** role (`app.read`, `exports.view`, `table.view`, `workflow.view`, `dashboard.view`) is read-only; **`member`** adds write caps. Assign **`viewer`** or **`member`** to groups and grant resources via **`rbac_resource_grants`** (seeded for **Everyone** on migrate).

**Sidebar apps**: Navigation links from `table_navigation` can be grouped into collapsible sidebar sections. Create rows in **`apps`** (via system table **`table_apps`**) and assign links in **`app_navigation_links`** (`table_app_navigation_links`). Unassigned links appear under **Ungrouped** in the sidebar; the **System** section remains hardcoded for admin links. An app's item page has a **Create app table** link (`/admin/table/create?navigationAppId={id}`); its **Navigation links** panel lists, adds, edits and removes the app's links. That panel comes from built-in relations (`app_navigation_links.app_id` → `table_apps`, with a pull-through of the navigation item's `name`, and `navigation_item_id` → `table_navigation`) that `EnsureSystemTableRelations` (`service/internal/repo/system_table_relations.go`) adds at startup when missing; add other built-in relations between system tables there, not in SQL migrations, because the physical-FK backfill only runs while `table_relations` is empty.

**Quick add**: a relation can show a quick add row under its related rows panel on the parent's item page, for typing child rows in one after another without leaving the page (for example, a recipe's ingredients). It is set in **Manage relation → Quick add** and stored as JSON in `table_relations.quick_add_json` (migration 0067), e.g. `{"enabled":true,"columns":["ingredient","quantity","unit"],"createMissing":true}`. It is returned as `ForeignKey.quick_add_json` and set with `UpdateTableRelation` (`quick_add_json` is optional: an empty string clears it, and leaving it out keeps it). `columns` lists the related table's columns in order; when it is empty, every writable column except the link to the parent is shown. A relation column is typed by name and matched against the referenced table's rows: by `name` or `title`, ignoring case, or by `#id`. With `createMissing`, a name that matches no row creates that row (in its `name`, else `title` column) before the child row is added. Enter adds the row, clears the inputs and puts focus back on the first one. The row appears only for users who may insert into the related table. Code: `QuickAddRow.vue` (in `TableComponent`'s `footer` slot) and `frontend/src/utils/quickAdd.ts`; on the server, `service/internal/repo/relation_quick_add.go` validates and normalises the setting.

**Row actions**: `table_configurations.row_actions` is an optional JSON array of links shown on a row's item page, e.g. `[{"label":"Create app table","path":"/admin/table/create","query":{"navigationAppId":"{id}"},"icon":"Add01Icon"}]`. `{id}` is the row's primary key and `{column}` any row field; `fromTable`/`fromRowId` are always appended so the target can return to the row. They are returned as `rowActions` by `GetTableStructure`. System table defaults are seeded only while the column is NULL, so user edits are kept; when a default changes, list the old one in `previousRowActions` (`service/internal/repo/repo.go`) so unedited rows are updated at startup. Related-panel inserts go to the related table's `create_delegate` (with the parent context query) when it has one.

**Bulk entry**: setting `table_configurations.bulk_entry_json` adds a **Bulk entry** button to the table toolbar (`/table/{tc}/bulk-entry`, `BulkEntryView.vue`) for entering many rows in quick succession, by keyboard-wedge barcode scanner or phone camera (native `BarcodeDetector`, else the `barcode-detector` zxing-wasm ponyfill, loaded on demand and served locally). Example: `{"scan_column":"barcode","quantity_column":"quantity","match_columns":["expirydate"],"fields":["title"],"save_on_scan":false}`. With `quantity_column` set, each entry calls **`AddItemQuantity`**, which in one transaction adds to the newest row whose scan and match columns are equal (a blank value matches NULL) or creates that row; a negative quantity takes stock away (never below zero). Without it, every entry is a new row. `fields` are filled in from the newest row with the same code. Unknown column names in the JSON are dropped, and an unknown `scan_column` turns bulk entry off.

**Images**: a column of type `image` (Add column → Image; stored as `BIGINT` with the `image` column semantic) holds an id from the **`images`** system table (`table_images`, admin only), which records each file's `mime_type`, `size_bytes`, `width`, `height`, `sha256`, `original_filename`, uploader and the `table_configuration`/`column_name` it was uploaded for. Files live on disk in `images_dir` (`SICKROCK_IMAGES_DIR`; default `<configdir>/images`, `/config/images` in the container, else `../tmp/images`) under random names (`service/internal/images`). **`UploadImage`** (page id, column name, bytes, filename) needs insert or edit access to the table and an image column, accepts JPEG/PNG/GIF/WebP up to 15 MiB (never SVG), and returns the image; the row's column is then set with `CreateItem`/`EditItem`. **`GET /api/images/{id}`** serves the file to callers with view access to the owning table (otherwise 404), using the same session/API-key auth as the API; the SPA fetches it with the session token into an object URL (`frontend/src/utils/images.ts`). In the UI, `ImageField.vue` uploads a file, takes a photo with the browser camera (`getUserMedia`, `useCameraCapture.ts`), or pastes an image from the clipboard (the **Paste** button reads `navigator.clipboard`; Ctrl+V while focus is in the field uses the paste event). Camera and the Paste button need HTTPS or localhost. Files replaced or cleared on a row are not deleted yet.

Authenticated users can open **`/exports/{slug}`** (for example `/exports/free_weekends`). Export definitions live in **`read_only_calendar_exports`** and are managed via the system table **`table_read_only_exports`** (Control Panel → Read-only calendar exports). Each row defines the source table configuration, optional calendar `table_view_id`, server-side `where_json` filters (supports `%` LIKE patterns), `weekends_only` post-filter, `display_mode` (`full` or `free_busy`), and `allowed_group_ids` (JSON array of IAM group ids). Data is loaded only through **`GetReadOnlyCalendarExport`** (slug only — clients never send table names or arbitrary `where` clauses). OAuth login providers can be advertised via Init (`SICKROCK_OAUTH_PROVIDERS` JSON env); group ACL applies to OAuth users the same as local users.

**Cursor**: In MCP settings use the HTTP transport with URL `https://your-sickrock-host/mcp` and set the API key in the request (per your client’s docs). No separate binary is required when SickRock is reachable over HTTP.

## Agent Integration Patterns

### 1. Webhook Integration
Agents can be integrated via webhooks to respond to events:
- Table creation events
- Data modification events
- User authentication events
- System health events

### 2. Scheduled Tasks
Agents can run on schedules for:
- Data analysis reports
- Security audits
- Performance monitoring
- Cleanup tasks

### 3. Real-time Processing
Agents can process data in real-time for:
- Live data validation
- Instant UI updates
- Real-time notifications
- Dynamic dashboard updates

## Development Guidelines

### Agent Development
When developing new agents:

1. **Use the existing gRPC API** - All agents should interact through the defined protobuf interface
2. **Follow the authentication pattern** - Use JWT tokens or API keys for authentication
3. **Implement proper error handling** - Use the standard error response format
4. **Log all activities** - Use the structured logging system
5. **Test with both SQLite and MySQL** - Ensure compatibility with both database engines
6. **Prefer existing UI libraries** - Use femtocrank (NPM library) and picocrank (Vue component library) for styling instead of creating new CSS rules

### Agent Deployment
Agents can be deployed as:
- **Standalone services** - Independent microservices
- **Embedded modules** - Integrated into the main application
- **External tools** - Command-line utilities or scripts

### Configuration
Agents should be configurable via:
- Environment variables
- Configuration files
- Database settings
- User preferences

### UI Styling Guidelines
When developing UI components for agents:

1. **Use femtocrank** - The project's NPM library provides consistent styling utilities and CSS classes
2. **Leverage picocrank** - The Vue component library offers pre-built components with proper styling
3. **Avoid custom CSS** - New CSS rules should generally be avoided in favor of existing library styles
4. **Maintain consistency** - Use the established design system to ensure UI consistency across the application
5. **Component composition** - Build complex UIs by composing existing picocrank components rather than creating new styled elements
6. **Dialogs** - Use picocrank's `Dialog` (`v-model:open`, or a template ref typed `DialogApi` from `frontend/src/utils/dialog.ts` with `show()`/`close()`), not hand-made overlays or bare `<dialog>`s. Put the main action in its `actions` slot (a submit button reaches its form with `form="<form id>"`); the close button, Escape and a backdrop click dismiss it, so there is no Cancel button. It is a bottom sheet on narrow screens (`narrow-layout="fullscreen"` for pickers).
7. **Responsive layout** - Narrow screens are 768px and below (`useIsNarrow()`, `.hide-narrow` / `.show-narrow`). Section and Header toolbars fold into a ⋮ menu when they do not fit, so keep toolbar controls as direct children (give a wrapper `display: contents`), give icon-only buttons an `aria-label` (shown as the menu label), and do not hide toolbar buttons on phones. Use `.action-row` for groups of buttons and `dvh` (after a `vh` fallback) for viewport heights.

**Example of preferred approach:**
```vue
<template>
  <!-- Use picocrank components -->
  <Section>
    <Button variant="primary" @click="handleAction">
      <!-- Use femtocrank styling classes -->
      <span class="text-sm font-medium">Action Button</span>
    </Button>
  </Section>
</template>
```

**Avoid:**
```vue
<template>
  <!-- Don't create custom styled elements -->
  <div class="custom-button custom-styling">
    <span style="color: blue; font-weight: bold;">Action Button</span>
  </div>
</template>

<style>
/* Avoid adding new CSS rules */
.custom-button {
  background: #007bff;
  /* ... */
}
</style>
```

## Example Agent Implementations

### Database Schema Analyzer
```go
// Example agent that analyzes database schemas
type SchemaAnalyzer struct {
    client *sickrockpbconnect.SickRockClient
}

func (sa *SchemaAnalyzer) AnalyzeDatabase(database string) (*SchemaAnalysis, error) {
    // Get all tables
    tables, err := sa.client.GetDatabaseTables(context.Background(),
        &sickrockpb.GetDatabaseTablesRequest{Database: database})

    // Analyze each table structure
    for _, table := range tables.Tables {
        structure, err := sa.client.GetTableStructure(context.Background(),
            &sickrockpb.GetTableStructureRequest{PageId: table.TableName})
        // ... analysis logic
    }

    return analysis, nil
}
```

### UI Generator
```typescript
// Example agent that generates UI components
class UIGenerator {
    private client: SickRockClient;

    async generateTableView(tableName: string): Promise<TableView> {
        const structure = await this.client.getTableStructure({
            pageId: tableName
        });

        const columns = structure.fields.map(field => ({
            columnName: field.name,
            isVisible: true,
            columnOrder: 0,
            sortOrder: ""
        }));

        return await this.client.createTableView({
            tableName,
            viewName: `${tableName}_default`,
            columns
        });
    }
}
```

## Monitoring and Observability

### Metrics
Agents should expose metrics for:
- Processing time
- Success/failure rates
- Resource usage
- Data processed

### Logging
Use structured logging with:
- Agent identification
- Operation context
- Performance metrics
- Error details

### Health Checks
Implement health checks for:
- Agent availability
- Database connectivity
- API responsiveness
- Resource utilization

## Security Considerations

### Authentication
- All agents must authenticate with valid tokens
- Use least privilege principle for permissions
- Implement proper session management

### Data Protection
- Encrypt sensitive data in transit and at rest
- Implement proper access controls
- Audit all data access

### API Security
- Validate all inputs
- Implement rate limiting
- Use HTTPS for all communications
- Regular security updates

## Contributing

When contributing agent implementations:

1. Follow the existing code patterns
2. Add comprehensive tests
3. Update this documentation
4. Include usage examples
5. Consider both SQLite and MySQL compatibility

## Future Enhancements

Potential future agent capabilities:
- **ML Data Insights Agent** - Machine learning-based data analysis
- **Performance Optimization Agent** - Automatic query optimization
- **Compliance Agent** - Automated compliance checking
- **Backup Agent** - Automated backup and recovery
- **Integration Agent** - Third-party system integration

## Support

For questions about agent development or integration:
- Discord: [SickRock Discord Server](https://discord.gg/jhYWWpNJ3v)
- GitHub Issues: [SickRock Issues](https://github.com/jamesread/SickRock/issues)
- Documentation: See the main README.md for project details

---

*This document is part of the SickRock project - a no-code database web application builder.*
