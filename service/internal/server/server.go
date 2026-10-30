package server

import (
	"cmp"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"slices"
	"strings"
	"time"
	"unicode"

	"connectrpc.com/connect"
	"github.com/expr-lang/expr"
	armhealth "github.com/jamesread/armature-health"
	armpassword "github.com/jamesread/armature-iam/password"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"

	sickrockpb "github.com/jamesread/SickRock/gen/proto"
	"github.com/jamesread/SickRock/internal/buildinfo"
	"github.com/jamesread/SickRock/internal/iam"
	"github.com/jamesread/SickRock/internal/images"
	"github.com/jamesread/SickRock/internal/notifications"
	repo "github.com/jamesread/SickRock/internal/repo"
	log "github.com/sirupsen/logrus"
)

type SickRockServer struct {
	repo                *repo.Repository
	notificationService *notifications.NotificationService
	auth                *iam.AuthLayer
	health              *armhealth.Service
	images              *images.Store
}

// markdownRenderer is a configured goldmark instance for rendering markdown
var markdownRenderer = goldmark.New(
	goldmark.WithExtensions(extension.GFM),
	goldmark.WithParserOptions(
		parser.WithAutoHeadingID(),
	),
	goldmark.WithRendererOptions(
		html.WithHardWraps(),
		html.WithXHTML(),
	),
)

// renderMarkdown converts markdown content to HTML
func renderMarkdown(content string) string {
	if content == "" {
		return ""
	}

	var buf strings.Builder
	if err := markdownRenderer.Convert([]byte(content), &buf); err != nil {
		log.WithError(err).Error("Failed to render markdown")
		return content // Return original content if rendering fails
	}
	return buf.String()
}

func NewSickRockServer(r *repo.Repository, authLayer *iam.AuthLayer) *SickRockServer {
	return &SickRockServer{
		repo:                r,
		notificationService: notifications.NewNotificationService(r),
		auth:                authLayer,
	}
}

// safeInt64ToInt32 converts an int64 to int32, clamping to int32 max/min values if overflow occurs
func safeInt64ToInt32(value int64) int32 {
	if value > math.MaxInt32 {
		return math.MaxInt32
	}
	if value < math.MinInt32 {
		return math.MinInt32
	}
	return int32(value)
}

func (s *SickRockServer) Init(ctx context.Context, req *connect.Request[sickrockpb.InitRequest]) (*connect.Response[sickrockpb.InitResponse], error) {
	dbName := strings.TrimSpace(os.Getenv("DB_NAME"))

	currentUsername, rbacPerms, rbacSuperuser := s.initIdentity(ctx, req)

	res := connect.NewResponse(&sickrockpb.InitResponse{
		Version:         buildinfo.Version,
		Commit:          buildinfo.Commit,
		Date:            buildinfo.Date,
		DbName:          dbName,
		CurrentUsername: currentUsername,
		RbacPermissions: rbacPerms,
		RbacIsSuperuser: rbacSuperuser,
		OauthProviders:  loadOAuthProvidersFromEnv(),
	})
	return res, nil
}

func (s *SickRockServer) initIdentity(ctx context.Context, req *connect.Request[sickrockpb.InitRequest]) (string, []string, bool) {
	if au := s.authUser(ctx); au != nil && au.User != nil {
		return iamUserToProto(au)
	}
	return s.initIdentityFromSession(ctx, req)
}

func (s *SickRockServer) initIdentityFromSession(ctx context.Context, req *connect.Request[sickrockpb.InitRequest]) (string, []string, bool) {
	sessionUser, err := s.resolveSessionUser(ctx, req)
	if err != nil || sessionUser == nil {
		return "", nil, false
	}
	return iamUserToProto(sessionUser)
}

func (s *SickRockServer) Ping(ctx context.Context, req *connect.Request[sickrockpb.PingRequest]) (*connect.Response[sickrockpb.PingResponse], error) {
	message := req.Msg.GetMessage()
	if message == "" {
		message = "pong"
	}
	res := connect.NewResponse(&sickrockpb.PingResponse{
		Message:       message,
		TimestampUnix: time.Now().Unix(),
	})
	return res, nil
}

func (s *SickRockServer) GetNavigationLinks(ctx context.Context, req *connect.Request[sickrockpb.GetNavigationLinksRequest]) (*connect.Response[sickrockpb.GetNavigationLinksResponse], error) {
	res := connect.NewResponse(&sickrockpb.GetNavigationLinksResponse{Links: []*sickrockpb.NavigationLink{
		{Label: "Home", Path: "/"},
		{Label: "About", Path: "/about"},
	}})
	return res, nil
}

func (s *SickRockServer) GetTableConfigurations(ctx context.Context, req *connect.Request[sickrockpb.GetTableConfigurationsRequest]) (*connect.Response[sickrockpb.GetTableConfigurationsResponse], error) {
	configs, err := s.repo.ListTableConfigurationsWithDetails(ctx)
	if err != nil {
		return nil, err
	}
	pages := make([]*sickrockpb.Page, 0, len(configs))
	for _, config := range configs {
		if !s.canDiscoverTableConfiguration(ctx, config.Name) {
			continue
		}
		// Default to "main" if database is NULL or empty
		dbName := "main"
		if config.Db.Valid && config.Db.String != "" {
			dbName = config.Db.String
		}
		pages = append(pages, &sickrockpb.Page{
			Id:       config.Name,
			Title:    config.Title,
			Slug:     config.Name,
			Ordinal:  toInt32(config.Ordinal),
			Icon:     config.Icon.String,
			View:     "table", // Default view type is always "table" for table configurations
			Database: dbName,
		})
	}
	res := connect.NewResponse(&sickrockpb.GetTableConfigurationsResponse{Pages: pages})
	return res, nil
}

func (s *SickRockServer) CreateTable(ctx context.Context, req *connect.Request[sickrockpb.CreateTableRequest]) (*connect.Response[sickrockpb.CreateTableResponse], error) {
	database := req.Msg.GetDatabase()
	table := req.Msg.GetTable()

	if table == "" {
		return connect.NewResponse(&sickrockpb.CreateTableResponse{
			Success: false,
			Message: "Table name is required",
		}), nil
	}

	if database == "" {
		database = "main" // Default database
	}

	// Create the physical table
	err := s.repo.CreateTable(ctx, database, table)
	if err != nil {
		log.Errorf("Failed to create table: %v", err)
		return connect.NewResponse(&sickrockpb.CreateTableResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to create table: %v", err),
		}), nil
	}

	return connect.NewResponse(&sickrockpb.CreateTableResponse{
		Success: true,
		Message: "Table created successfully",
	}), nil
}

func (s *SickRockServer) CreateTableConfiguration(ctx context.Context, req *connect.Request[sickrockpb.CreateTableConfigurationRequest]) (*connect.Response[sickrockpb.CreateTableConfigurationResponse], error) {
	name := req.Msg.GetName()
	database := req.Msg.GetDatabase()
	table := req.Msg.GetTable()

	// Validate required fields
	if name == "" {
		return connect.NewResponse(&sickrockpb.CreateTableConfigurationResponse{
			Success: false,
			Message: "Configuration name is required",
		}), nil
	}

	if database == "" {
		database = "main" // Default database
	}

	createNavigation := true
	if req.Msg.CreateNavigationEntry != nil {
		createNavigation = req.Msg.GetCreateNavigationEntry()
	}

	navigationAppID := int(req.Msg.GetNavigationAppId())
	err := s.repo.CreateTableConfiguration(ctx, name, database, table, createNavigation, navigationAppID)
	if err != nil {
		log.Errorf("Failed to create table configuration: %v", err)
		return connect.NewResponse(&sickrockpb.CreateTableConfigurationResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to create table configuration: %v", err),
		}), nil
	}
	if grantErr := s.repo.EnsureEveryoneTableRole(ctx, name, "member"); grantErr != nil {
		log.Warnf("Failed to seed Everyone grant for table %s: %v", name, grantErr)
	}

	return connect.NewResponse(&sickrockpb.CreateTableConfigurationResponse{
		Success: true,
		Message: "Table configuration created successfully",
	}), nil
}

func (s *SickRockServer) GetDatabaseTables(ctx context.Context, req *connect.Request[sickrockpb.GetDatabaseTablesRequest]) (*connect.Response[sickrockpb.GetDatabaseTablesResponse], error) {
	if err := s.requireSystemSettings(ctx); err != nil {
		return nil, err
	}
	database := req.Msg.GetDatabase()
	if database == "" {
		database = "main"
	}

	tables, err := s.repo.GetDatabaseTables(ctx, database)
	if err != nil {
		return nil, err
	}

	res := connect.NewResponse(&sickrockpb.GetDatabaseTablesResponse{Tables: databaseTablesToProto(tables)})
	return res, nil
}

func databaseTablesToProto(tables []repo.DatabaseTableInfo) []*sickrockpb.DatabaseTable {
	dbTables := make([]*sickrockpb.DatabaseTable, 0, len(tables))
	for _, table := range tables {
		dbTables = append(dbTables, &sickrockpb.DatabaseTable{
			TableName:         table.TableName,
			HasConfiguration:  table.HasConfiguration,
			ConfigurationName: table.ConfigurationName.String,
			View:              "table",
		})
	}
	return dbTables
}

// displayNavigationTitle formats a stored navigation name for display.
// Underscores become spaces, and each word is title-cased. The stored value is unchanged.
func displayNavigationTitle(raw string) string {
	words := strings.Fields(strings.ReplaceAll(raw, "_", " "))
	for i, word := range words {
		runes := []rune(strings.ToLower(word))
		runes[0] = unicode.ToUpper(runes[0])
		words[i] = string(runes)
	}
	return strings.Join(words, " ")
}

// navigationLinkTitle returns the sidebar label for a navigation item.
// A non-empty title overrides the name-based calculation.
func navigationLinkTitle(item repo.NavigationItem) string {
	if override := strings.TrimSpace(item.TitleOverride.String); override != "" {
		return override
	}
	return displayNavigationTitle(item.Navigation.String)
}

func bookmarkLinkTitle(bookmark repo.UserBookmark) string {
	if bookmark.NavigationItem != nil {
		return navigationLinkTitle(*bookmark.NavigationItem)
	}
	return displayNavigationTitle(bookmark.Title.String)
}

func navigationItemRepoToProto(item repo.NavigationItem, tableView string) *sickrockpb.NavigationItem {
	return &sickrockpb.NavigationItem{
		Id:      toInt32(item.ID),
		Ordinal: toInt32(item.Ordinal),
		TableConfiguration: func() int32 {
			if item.TableConfiguration.Valid {
				return toInt32(item.TableConfiguration.Int64)
			}
			return 0
		}(),
		TableName:  item.TableName.String,
		TableTitle: displayNavigationTitle(item.TableTitle.String),
		Icon:       item.Icon.String,
		TableView:  tableView,
		DashboardId: func() int32 {
			if item.DashboardID.Valid {
				return toInt32(item.DashboardID.Int64)
			}
			return 0
		}(),
		DashboardName: item.DashboardName.String,
		Title:         navigationLinkTitle(item),
		WorkflowId: func() int32 {
			if item.WorkflowID.Valid {
				return toInt32(item.WorkflowID.Int64)
			}
			return 0
		}(),
		WorkflowName: item.WorkflowName.String,
	}
}

func (s *SickRockServer) GetNavigation(ctx context.Context, req *connect.Request[sickrockpb.GetNavigationRequest]) (*connect.Response[sickrockpb.GetNavigationResponse], error) {
	items, err := s.repo.GetNavigation(ctx)
	if err != nil {
		return nil, err
	}

	visibleByID, navigationItems := s.visibleNavigationItems(ctx, items)

	res := connect.NewResponse(&sickrockpb.GetNavigationResponse{
		Items:          navigationItems,
		Bookmarks:      s.navigationBookmarks(ctx),
		Workflows:      s.navigationWorkflows(ctx, items),
		Apps:           s.navigationApps(ctx, visibleByID),
		UngroupedItems: s.ungroupedNavigationItems(ctx, items, visibleByID),
	})
	return res, nil
}

// visibleNavigationItems filters items to those the caller may view, returning them by ID and as protos.
func (s *SickRockServer) visibleNavigationItems(ctx context.Context, items []repo.NavigationItem) (map[int]repo.NavigationItem, []*sickrockpb.NavigationItem) {
	visibleByID := make(map[int]repo.NavigationItem)
	navigationItems := make([]*sickrockpb.NavigationItem, 0, len(items))
	for _, item := range items {
		if !s.canViewNavigationItem(ctx, item) {
			continue
		}
		visibleByID[item.ID] = item
		navigationItems = append(navigationItems, navigationItemRepoToProto(item, ""))
	}
	return visibleByID, navigationItems
}

// navigationApps returns sidebar apps that contain at least one visible navigation item.
func (s *SickRockServer) navigationApps(ctx context.Context, visibleByID map[int]repo.NavigationItem) []*sickrockpb.NavigationApp {
	navApps, err := s.repo.GetNavigationApps(ctx)
	if err != nil {
		log.Warnf("Failed to load navigation apps: %v", err)
		return nil
	}
	var navigationApps []*sickrockpb.NavigationApp
	for _, app := range navApps {
		members := visibleAppMembers(app.MemberIDs, visibleByID)
		if len(members) == 0 {
			continue
		}
		navigationApps = append(navigationApps, &sickrockpb.NavigationApp{
			Id:      toInt32(app.ID),
			Title:   app.Title,
			Ordinal: toInt32(app.Ordinal),
			Items:   members,
		})
	}
	return navigationApps
}

func visibleAppMembers(memberIDs []int, visibleByID map[int]repo.NavigationItem) []*sickrockpb.NavigationItem {
	members := make([]*sickrockpb.NavigationItem, 0, len(memberIDs))
	for _, navID := range memberIDs {
		if item, ok := visibleByID[navID]; ok {
			members = append(members, navigationItemRepoToProto(item, ""))
		}
	}
	return members
}

// ungroupedNavigationItems returns visible items that are not assigned to any sidebar app.
func (s *SickRockServer) ungroupedNavigationItems(ctx context.Context, items []repo.NavigationItem, visibleByID map[int]repo.NavigationItem) []*sickrockpb.NavigationItem {
	assignedNavIDs, err := s.repo.NavigationItemIDsInApps(ctx)
	if err != nil {
		log.Warnf("Failed to load app navigation assignments: %v", err)
		assignedNavIDs = map[int]struct{}{}
	}
	ungroupedItems := make([]*sickrockpb.NavigationItem, 0, len(visibleByID))
	for _, item := range items {
		_, visible := visibleByID[item.ID]
		_, assigned := assignedNavIDs[item.ID]
		if visible && !assigned {
			ungroupedItems = append(ungroupedItems, navigationItemRepoToProto(item, ""))
		}
	}
	return ungroupedItems
}

// navigationBookmarks returns the authenticated caller's bookmarks they may still view.
func (s *SickRockServer) navigationBookmarks(ctx context.Context) []*sickrockpb.UserBookmark {
	userID, err := s.getUserIDFromContext(ctx)
	if err != nil || userID <= 0 {
		return nil
	}
	userBookmarks, err := s.repo.GetUserBookmarks(ctx, userID)
	if err != nil {
		log.Warnf("Failed to load user bookmarks: %v", err)
		return nil
	}
	var bookmarks []*sickrockpb.UserBookmark
	for _, bookmark := range userBookmarks {
		if bookmark.NavigationItem != nil && !s.canViewNavigationItem(ctx, *bookmark.NavigationItem) {
			continue
		}
		bookmarks = append(bookmarks, bookmarkRepoToProto(bookmark))
	}
	return bookmarks
}

func bookmarkRepoToProto(bookmark repo.UserBookmark) *sickrockpb.UserBookmark {
	var navItem *sickrockpb.NavigationItem
	if bookmark.NavigationItem != nil {
		navItem = navigationItemRepoToProto(*bookmark.NavigationItem, bookmark.NavigationItem.TableView.String)
	}
	return &sickrockpb.UserBookmark{
		Id:               toInt32(bookmark.ID),
		UserId:           toInt32(bookmark.UserID),
		NavigationItemId: toInt32(bookmark.NavigationItemID),
		NavigationItem:   navItem,
		Title:            bookmarkLinkTitle(bookmark),
	}
}

// navigationWorkflows returns workflows the caller may view that have hub members or a visible hub link.
func (s *SickRockServer) navigationWorkflows(ctx context.Context, items []repo.NavigationItem) []*sickrockpb.Workflow {
	workflows, err := s.repo.GetWorkflows(ctx)
	if err != nil {
		log.Warnf("Failed to load workflows: %v", err)
		workflows = []repo.Workflow{}
	}

	workflowItemsMap := s.workflowMemberItems(ctx)

	workflowProtos := make([]*sickrockpb.Workflow, 0, len(workflows))
	for _, workflow := range workflows {
		if s.requireWorkflowAccess(ctx, workflow.ID, repo.ResourceActionView) != nil {
			continue
		}
		members := workflowItemsMap[toInt32(workflow.ID)]
		if len(members) == 0 && !s.hasVisibleWorkflowHubLink(ctx, items, workflow.ID) {
			continue
		}
		workflowProtos = append(workflowProtos, &sickrockpb.Workflow{
			Id:      toInt32(workflow.ID),
			Name:    workflow.Name,
			Ordinal: toInt32(workflow.Ordinal),
			Icon:    workflow.Icon.String,
			Items:   members,
		})
	}
	return workflowProtos
}

// workflowMemberItems groups workflow hub members (workflow_navigation_members, not
// table_navigation.workflow_id) by workflow, keeping only those the caller may view.
func (s *SickRockServer) workflowMemberItems(ctx context.Context) map[int32][]*sickrockpb.NavigationItem {
	workflowItemsMap := make(map[int32][]*sickrockpb.NavigationItem)
	memberGroups, err := s.repo.GetNavigationItemsForWorkflowMembership(ctx)
	if err != nil {
		log.Warnf("Failed to load workflow navigation members: %v", err)
		return workflowItemsMap
	}
	for workflowID, memberItems := range memberGroups {
		if s.requireWorkflowAccess(ctx, workflowID, repo.ResourceActionView) != nil {
			continue
		}
		for _, memberItem := range memberItems {
			if s.canViewNavigationItem(ctx, memberItem) {
				workflowItemsMap[toInt32(workflowID)] = append(workflowItemsMap[toInt32(workflowID)], navigationItemRepoToProto(memberItem, ""))
			}
		}
	}
	return workflowItemsMap
}

// hasVisibleWorkflowHubLink reports whether a visible navigation item links to the workflow hub
// itself (a workflow link with no table or dashboard).
func (s *SickRockServer) hasVisibleWorkflowHubLink(ctx context.Context, items []repo.NavigationItem, workflowID int) bool {
	for _, item := range items {
		if isWorkflowHubLink(item, workflowID) && s.canViewNavigationItem(ctx, item) {
			return true
		}
	}
	return false
}

func isWorkflowHubLink(item repo.NavigationItem, workflowID int) bool {
	if !item.WorkflowID.Valid || int(item.WorkflowID.Int64) != workflowID {
		return false
	}
	hasTable := item.TableName.Valid && item.TableName.String != ""
	hasDashboard := item.DashboardName.Valid && item.DashboardName.String != ""
	return !hasTable && !hasDashboard
}

func (s *SickRockServer) SetWorkflowNavigationMembers(ctx context.Context, req *connect.Request[sickrockpb.SetWorkflowNavigationMembersRequest]) (*connect.Response[sickrockpb.SetWorkflowNavigationMembersResponse], error) {
	workflowID := int(req.Msg.GetWorkflowId())
	if workflowID <= 0 {
		return connect.NewResponse(&sickrockpb.SetWorkflowNavigationMembersResponse{
			Success: false,
			Message: "workflow_id is required",
		}), nil
	}
	if err := s.requireWorkflowAccess(ctx, workflowID, repo.ResourceActionStart); err != nil {
		return nil, err
	}

	if err := s.repo.SetWorkflowNavigationMembers(ctx, workflowID, workflowNavigationMembers(req.Msg.GetMembers())); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&sickrockpb.SetWorkflowNavigationMembersResponse{
		Success: true,
		Message: "Workflow navigation members updated",
	}), nil
}

func workflowNavigationMembers(members []*sickrockpb.WorkflowNavigationMember) []repo.WorkflowNavigationMemberInput {
	out := make([]repo.WorkflowNavigationMemberInput, 0, len(members))
	for _, m := range members {
		out = append(out, repo.WorkflowNavigationMemberInput{
			NavigationItemID: int(m.GetNavigationItemId()),
			Ordinal:          int(m.GetOrdinal()),
		})
	}
	return out
}

func (s *SickRockServer) ListItems(ctx context.Context, req *connect.Request[sickrockpb.ListItemsRequest]) (*connect.Response[sickrockpb.ListItemsResponse], error) {
	// Use page_id as table name for this simple mapping
	table := req.Msg.GetTcName()
	if err := s.requireExportsManageForTable(ctx, table); err != nil {
		return nil, err
	}
	if err := s.requireDataTableAccess(ctx, table, repo.ResourceActionView); err != nil {
		return nil, err
	}

	items, err := s.repo.ListItemsInTable(ctx, table, nonEmptyKeys(req.Msg.GetWhere()))
	if err != nil {
		return nil, err
	}

	items, err = s.applyListRelationCalculatedColumns(ctx, table, strings.TrimSpace(req.Msg.GetRelationConstraintName()), items)
	if err != nil {
		return nil, err
	}

	// Get conditional formatting rules once for all items (not per item)
	rules := s.conditionalFormattingRules(ctx, table, "ListItems: ")

	out := make([]*sickrockpb.Item, 0, len(items))
	for _, it := range items {
		additionalFields := itemFieldsToStrings(it.Fields, false)
		applyMarkdownRules(rules, it.Fields, additionalFields)
		out = append(out, itemRepoToProto(it, additionalFields))
	}
	return connect.NewResponse(&sickrockpb.ListItemsResponse{Items: out}), nil
}

// nonEmptyKeys copies m, dropping entries with an empty key.
func nonEmptyKeys(m map[string]string) map[string]string {
	out := make(map[string]string, len(m))
	for k, v := range m {
		if k != "" {
			out[k] = v
		}
	}
	return out
}

// applyListRelationCalculatedColumns adds relation calculated columns to items: those of the named
// relation when constraintName is set, otherwise those of every relation sourced from table.
// Enrichment failures are logged and the items returned unchanged; an invalid relation is an error.
func (s *SickRockServer) applyListRelationCalculatedColumns(ctx context.Context, table, constraintName string, items []repo.Item) ([]repo.Item, error) {
	if constraintName == "" {
		enriched, err := s.repo.ApplyAllSourceRelationCalculatedColumns(ctx, table, items)
		if err != nil {
			log.WithError(err).WithField("table", table).Warn("ListItems: source relation calculated columns skipped")
			return items, nil
		}
		return enriched, nil
	}

	rel, columns, err := s.listRelationPullThrough(ctx, table, constraintName)
	if err != nil {
		return nil, err
	}
	enriched, err := s.repo.ApplyRelationCalculatedColumns(ctx, rel.SourceTableKey, columns, items)
	if err != nil {
		log.WithError(err).WithFields(log.Fields{
			"constraint": constraintName,
			"table":      table,
		}).Warn("ListItems: relation calculated columns skipped")
		return items, nil
	}
	return enriched, nil
}

// listRelationPullThrough loads the named relation, checks it belongs to table, and parses its pull-through columns.
func (s *SickRockServer) listRelationPullThrough(ctx context.Context, table, constraintName string) (*repo.TableRelation, []repo.RelationPullThroughField, error) {
	rel, err := s.repo.GetTableRelationByConstraintName(ctx, constraintName)
	if err != nil {
		return nil, nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("relation not found: %w", err))
	}
	if !strings.EqualFold(strings.TrimSpace(rel.SourceTableKey), strings.TrimSpace(table)) {
		return nil, nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("relation %q is not for table %q", constraintName, table))
	}
	columns, err := repo.ParseRelationPullThroughJSON(rel.PullThroughJSON)
	if err != nil {
		return nil, nil, connect.NewError(connect.CodeInternal, err)
	}
	return rel, columns, nil
}

func (s *SickRockServer) CreateItem(ctx context.Context, req *connect.Request[sickrockpb.CreateItemRequest]) (*connect.Response[sickrockpb.CreateItemResponse], error) {
	table := cmp.Or(req.Msg.GetPageId(), "items")
	if err := s.requireItemWriteAccess(ctx, table, repo.ResourceActionInsert); err != nil {
		return nil, err
	}

	it, err := s.repo.CreateItemInTableWithTimestamp(ctx, table, req.Msg.GetAdditionalFields())
	if err != nil {
		return nil, err
	}
	// Include null values as empty strings so they appear in JSON output
	return connect.NewResponse(&sickrockpb.CreateItemResponse{Item: itemRepoToProto(it, itemFieldsToStrings(it.Fields, true))}), nil
}

// requireItemWriteAccess checks the caller may insert or edit rows in table: exports management for the
// exports table, never for the append-only audit log, and the matching table grant.
func (s *SickRockServer) requireItemWriteAccess(ctx context.Context, table string, action repo.ResourceGrantAction) error {
	if err := s.requireExportsManageForTable(ctx, table); err != nil {
		return err
	}
	if err := s.denyAuditLogMutation(ctx, table); err != nil {
		return err
	}
	return s.requireDataTableAccess(ctx, table, action)
}

func (s *SickRockServer) GetItem(ctx context.Context, req *connect.Request[sickrockpb.GetItemRequest]) (*connect.Response[sickrockpb.GetItemResponse], error) {
	// Get table name from the request, default to "items" for backward compatibility
	table := req.Msg.GetPageId()
	if table == "" {
		table = "items"
	}
	if err := s.requireDataTableAccess(ctx, table, repo.ResourceActionView); err != nil {
		return nil, err
	}

	tc, err := s.repo.GetTableConfiguration(ctx, table)
	if err != nil {
		return nil, err
	}

	it, err := s.repo.GetItemInTable(ctx, tc, req.Msg.GetId())
	if err != nil {
		return nil, err
	}

	s.trackRecentlyViewed(ctx, table, req.Msg.GetId())

	// Include null values as empty strings so they appear in JSON output
	additionalFields := itemFieldsToStrings(it.Fields, true)

	log.WithFields(log.Fields{
		"table":  table,
		"itemID": it.ID,
		"fields": it.Fields,
	}).Info("GetItem: Item fields before markdown processing")

	applyMarkdownRules(s.conditionalFormattingRules(ctx, table, ""), it.Fields, additionalFields)

	log.WithFields(log.Fields{
		"table":            table,
		"itemID":           it.ID,
		"additionalFields": additionalFields,
	}).Info("GetItem: Additional fields after markdown processing")

	return connect.NewResponse(&sickrockpb.GetItemResponse{Item: itemRepoToProto(it, additionalFields)}), nil
}

// trackRecentlyViewed records the item as recently viewed for the authenticated user; failures are only logged.
func (s *SickRockServer) trackRecentlyViewed(ctx context.Context, table, id string) {
	userID, err := s.getUserIDFromContext(ctx)
	if err != nil || userID <= 0 {
		return
	}
	if err := s.repo.InsertRecentlyViewed(ctx, userID, table, id); err != nil {
		log.WithError(err).WithFields(log.Fields{
			"table": table,
			"id":    id,
		}).Warn("Failed to track recently viewed item")
	}
}

func (s *SickRockServer) EditItem(ctx context.Context, req *connect.Request[sickrockpb.EditItemRequest]) (*connect.Response[sickrockpb.EditItemResponse], error) {
	// Get table name from the request, default to "items" for backward compatibility
	table := cmp.Or(req.Msg.GetPageId(), "items")
	if err := s.requireItemWriteAccess(ctx, table, repo.ResourceActionEdit); err != nil {
		return nil, err
	}

	// Get additional fields from the request
	additionalFields := req.Msg.GetAdditionalFields()
	if additionalFields == nil {
		additionalFields = make(map[string]string)
	}

	// Use the new method that supports additional fields
	it, err := s.repo.EditItemInTableWithFields(ctx, table, req.Msg.GetId(), "", additionalFields)
	if err != nil {
		return nil, err
	}

	// Include null values as empty strings so they appear in JSON output
	return connect.NewResponse(&sickrockpb.EditItemResponse{Item: itemRepoToProto(it, itemFieldsToStrings(it.Fields, true))}), nil
}

func (s *SickRockServer) DeleteItem(ctx context.Context, req *connect.Request[sickrockpb.DeleteItemRequest]) (*connect.Response[sickrockpb.DeleteItemResponse], error) {
	table := req.Msg.GetPageId()
	if err := s.requireDeleteItemAccess(ctx, table); err != nil {
		return nil, err
	}

	ok, err := s.repo.DeleteItemInTable(ctx, table, req.Msg.GetId())
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&sickrockpb.DeleteItemResponse{Deleted: ok}), nil
}

func (s *SickRockServer) requireDeleteItemAccess(ctx context.Context, table string) error {
	if err := s.requireExportsManageForTable(ctx, table); err != nil {
		return err
	}
	if err := s.denyAuditLogMutation(ctx, table); err != nil {
		return err
	}
	return s.requireDataTableAccess(ctx, table, repo.ResourceActionDelete)
}

func (s *SickRockServer) GetTableStructure(ctx context.Context, req *connect.Request[sickrockpb.GetTableStructureRequest]) (*connect.Response[sickrockpb.GetTableStructureResponse], error) {
	tcName := req.Msg.GetPageId()
	if err := s.requireExportsManageForTable(ctx, tcName); err != nil {
		return nil, err
	}
	if err := s.requireDataTableAccess(ctx, tcName, repo.ResourceActionView); err != nil {
		return nil, err
	}

	tc, err := s.repo.GetTableConfiguration(ctx, tcName)
	if err != nil {
		return nil, err
	}

	cols := s.tableStructureColumns(ctx, tc, tcName)
	displayByColumn, displayErr := s.repo.GetColumnDisplaySettings(ctx, tcName)
	if displayErr != nil {
		log.WithError(displayErr).Warn("Failed to load column display settings")
		displayByColumn = map[string]repo.ColumnDisplaySetting{}
	}

	colNames := make([]string, len(cols))
	for i, c := range cols {
		colNames[i] = c.Name
	}

	return connect.NewResponse(&sickrockpb.GetTableStructureResponse{
		Fields:            fieldSpecsToProto(cols, displayByColumn),
		CreateButtonText:  nullStringOr(tc.CreateButtonText, "Insert Row"),
		View:              s.defaultViewType(ctx, tcName),
		ForeignKeys:       s.tableForeignKeysToProto(ctx, tc, tcName),
		PrimaryKeyColumn:  tc.PrimaryKeyColumnName(),
		DefaultSortColumn: tc.SortColumnName(colNames),
		CreateDelegate:    strings.TrimSpace(tc.CreateDelegate.String),
		Title:             cmp.Or(tc.Title, tc.Name),
		RowName:           tc.DisplayRowName(),
		RowNamePlural:     tc.DisplayRowNamePlural(),
		ItemPageLayout: &sickrockpb.ItemPageLayout{
			Blocks: s.itemPageLayoutToProto(ctx, tcName),
		},
		ItemFieldGroups: &sickrockpb.ItemFieldGroups{
			Groups: s.itemFieldGroupsToProto(ctx, tcName),
		},
		ColumnDisplaySettings: columnDisplaySettingsToProto(displayByColumn),
		RowActions:            rowActionsToProto(tcName, tc.RowActions.String),
		BulkEntry:             bulkEntryToProto(tcName, tc.BulkEntry.String, colNames),
	}), nil
}

func nullStringOr(ns sql.NullString, fallback string) string {
	if ns.Valid {
		return ns.String
	}
	return fallback
}

// tableStructureColumns lists the table's columns with semantics applied; a listing failure is logged
// and yields no columns.
func (s *SickRockServer) tableStructureColumns(ctx context.Context, tc *repo.TableConfig, tcName string) []repo.FieldSpec {
	cols, err := s.repo.ListColumns(ctx, tc)
	if err != nil {
		log.Errorf("list columns: %v, table: %s", err, tcName)
	} else {
		log.WithField("columnCount", len(cols)).Debugf("list columns for table %s", tcName)
	}
	return s.repo.ApplyColumnSemantics(ctx, tcName, cols)
}

func fieldSpecsToProto(cols []repo.FieldSpec, displayByColumn map[string]repo.ColumnDisplaySetting) []*sickrockpb.Field {
	fields := make([]*sickrockpb.Field, 0, len(cols))
	for _, c := range cols {
		pbField := &sickrockpb.Field{
			Name:                      c.Name,
			Type:                      c.Type,
			Required:                  c.Required,
			DefaultToCurrentTimestamp: false, // This information is not stored in database metadata
		}
		if d, ok := repo.LookupColumnDisplay(displayByColumn, c.Name); ok {
			pbField.DisplayLabel, pbField.ValuePrefix, pbField.ValueSuffix = columnDisplayStrings(d)
		}
		fields = append(fields, pbField)
	}
	return fields
}

// columnDisplayStrings returns a column's display label (trimmed), value prefix and value suffix.
func columnDisplayStrings(d repo.ColumnDisplaySetting) (label, prefix, suffix string) {
	return strings.TrimSpace(d.DisplayLabel.String), d.ValuePrefix.String, d.ValueSuffix.String
}

func columnDisplaySettingsToProto(displayByColumn map[string]repo.ColumnDisplaySetting) []*sickrockpb.ColumnDisplaySettingInput {
	out := make([]*sickrockpb.ColumnDisplaySettingInput, 0, len(displayByColumn))
	for columnName, d := range displayByColumn {
		row := &sickrockpb.ColumnDisplaySettingInput{ColumnName: columnName}
		row.DisplayLabel, row.ValuePrefix, row.ValueSuffix = columnDisplayStrings(d)
		out = append(out, row)
	}
	return out
}

// tableForeignKeysToProto returns the table's foreign keys, resolving the table configuration
// name of each side. A failure to load them is logged and yields an empty list.
func (s *SickRockServer) tableForeignKeysToProto(ctx context.Context, tc *repo.TableConfig, tcName string) []*sickrockpb.ForeignKey {
	foreignKeys, err := s.repo.GetForeignKeysForTableConfig(ctx, tc)
	if err != nil {
		log.Errorf("Failed to get foreign keys: %v", err)
		return []*sickrockpb.ForeignKey{}
	}
	log.WithFields(log.Fields{
		"tcName": tcName,
		"numFks": len(foreignKeys),
	}).Debug("Retrieved foreign keys")

	tcNameByDbTable := s.foreignKeyTableConfigNames(ctx, foreignKeys, tc.Db.String)

	pbForeignKeys := make([]*sickrockpb.ForeignKey, 0, len(foreignKeys))
	for _, fk := range foreignKeys {
		tableDb, referencedDb := foreignKeySchemas(fk, tc.Db.String)
		pbForeignKeys = append(pbForeignKeys, &sickrockpb.ForeignKey{
			RelationId:            toInt32(fk.RelationID),
			ConstraintName:        fk.ConstraintName,
			TableName:             fk.TableName,
			ColumnName:            fk.ColumnName,
			ReferencedTable:       fk.ReferencedTable,
			ReferencedColumn:      fk.ReferencedColumn,
			OnDeleteAction:        fk.OnDeleteAction,
			OnUpdateAction:        fk.OnUpdateAction,
			TableTcName:           tcNameByDbTable[repo.DbTableLookupKey(tableDb, fk.TableName)],
			ReferencedTableTcName: tcNameByDbTable[repo.DbTableLookupKey(referencedDb, fk.ReferencedTable)],
			DisplayName:           fk.DisplayName,
			PullThroughFields:     pullThroughFieldsToProto(fk.PullThroughFields),
			AbsorbIntoItem:        fk.AbsorbIntoItem,
			AbsorbType:            fk.AbsorbType,
		})
	}
	return pbForeignKeys
}

// foreignKeySchemas returns the databases of a foreign key's table and referenced table,
// falling back to defaultDb when the key does not name one.
func foreignKeySchemas(fk repo.ForeignKey, defaultDb string) (tableDb, referencedDb string) {
	return cmp.Or(fk.TableSchema, defaultDb), cmp.Or(fk.ReferencedSchema, defaultDb)
}

// foreignKeyTableConfigNames batch-looks up the table configuration names of both sides of each
// foreign key, keyed by repo.DbTableLookupKey.
func (s *SickRockServer) foreignKeyTableConfigNames(ctx context.Context, foreignKeys []repo.ForeignKey, defaultDb string) map[string]string {
	lookupPairs := make([]repo.DbTablePair, 0, len(foreignKeys)*2)
	for _, fk := range foreignKeys {
		tableDb, referencedDb := foreignKeySchemas(fk, defaultDb)
		lookupPairs = append(lookupPairs,
			repo.DbTablePair{Db: tableDb, Table: fk.TableName},
			repo.DbTablePair{Db: referencedDb, Table: fk.ReferencedTable},
		)
	}
	tcNameByDbTable, err := s.repo.LookupTableConfigNames(ctx, lookupPairs)
	if err != nil {
		log.WithError(err).Warn("Failed to batch lookup table configuration names for foreign keys")
		return map[string]string{}
	}
	return tcNameByDbTable
}

// defaultViewType returns the view type of the caller's default view for the table, or "table".
func (s *SickRockServer) defaultViewType(ctx context.Context, tcName string) string {
	views, err := s.repo.GetTableViews(ctx, tcName)
	if err != nil || len(views) == 0 {
		return "table"
	}
	defaultView := pickDefaultTableView(views, s.userDefaultTableView(ctx, tcName))
	return cmp.Or(defaultView.ViewType, "table")
}

// pickDefaultTableView returns the user's default view when set and present, otherwise the
// table-wide default, otherwise the first view. views must not be empty.
func pickDefaultTableView(views []repo.TableView, userDefaultID int) repo.TableView {
	if userDefaultID > 0 {
		if i := slices.IndexFunc(views, func(v repo.TableView) bool { return v.ID == userDefaultID }); i >= 0 {
			return views[i]
		}
	}
	if i := slices.IndexFunc(views, func(v repo.TableView) bool { return v.IsDefault }); i >= 0 {
		return views[i]
	}
	return views[0] // GetTableViews returns views ordered, first is usually default
}

func (s *SickRockServer) itemPageLayoutToProto(ctx context.Context, tcName string) []*sickrockpb.PageLayoutBlock {
	layoutBlocks, err := s.repo.GetItemPageLayoutBlocks(ctx, tcName)
	if err != nil {
		log.WithError(err).Warn("Failed to load item page layout")
		layoutBlocks = nil
	}
	pbLayoutBlocks := make([]*sickrockpb.PageLayoutBlock, 0, len(layoutBlocks))
	for _, b := range layoutBlocks {
		pbLayoutBlocks = append(pbLayoutBlocks, &sickrockpb.PageLayoutBlock{
			Id:         toInt32(b.ID),
			Ordinal:    toInt32(b.Ordinal),
			BlockType:  b.BlockType,
			ConfigJson: b.ConfigJSON,
		})
	}
	return pbLayoutBlocks
}

func (s *SickRockServer) itemFieldGroupsToProto(ctx context.Context, tcName string) []*sickrockpb.ItemFieldGroup {
	fieldGroups, err := s.repo.GetItemFieldGroups(ctx, tcName)
	if err != nil {
		log.WithError(err).Warn("Failed to load item field groups")
		fieldGroups = nil
	}
	pbFieldGroups := make([]*sickrockpb.ItemFieldGroup, 0, len(fieldGroups))
	for i, g := range fieldGroups {
		pbFieldGroups = append(pbFieldGroups, &sickrockpb.ItemFieldGroup{
			Id:          toInt32(i + 1),
			Ordinal:     toInt32(g.Ordinal),
			Title:       g.Title,
			ColumnNames: append([]string(nil), g.ColumnNames...),
		})
	}
	return pbFieldGroups
}

func (s *SickRockServer) SetItemFieldGroups(ctx context.Context, req *connect.Request[sickrockpb.SetItemFieldGroupsRequest]) (*connect.Response[sickrockpb.SetItemFieldGroupsResponse], error) {
	tcName := strings.TrimSpace(req.Msg.GetTableConfiguration())
	if tcName == "" {
		return connect.NewResponse(&sickrockpb.SetItemFieldGroupsResponse{
			Success: false,
			Message: "Table configuration is required",
		}), nil
	}
	if err := s.requireDataTableAccess(ctx, tcName, repo.ResourceActionEdit); err != nil {
		return nil, err
	}
	inputs := make([]repo.FieldGroupInput, 0, len(req.Msg.GetGroups()))
	for i, g := range req.Msg.GetGroups() {
		ord := int(g.GetOrdinal())
		if ord <= 0 {
			ord = i + 1
		}
		inputs = append(inputs, repo.FieldGroupInput{
			Ordinal:     ord,
			Title:       g.GetTitle(),
			ColumnNames: append([]string(nil), g.GetColumnNames()...),
		})
	}
	if err := s.repo.SetItemFieldGroups(ctx, tcName, inputs); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&sickrockpb.SetItemFieldGroupsResponse{
		Success: true,
		Message: "Field groups saved",
	}), nil
}

func (s *SickRockServer) SetColumnDisplaySettings(ctx context.Context, req *connect.Request[sickrockpb.SetColumnDisplaySettingsRequest]) (*connect.Response[sickrockpb.SetColumnDisplaySettingsResponse], error) {
	tcName := strings.TrimSpace(req.Msg.GetTableConfiguration())
	if tcName == "" {
		return connect.NewResponse(&sickrockpb.SetColumnDisplaySettingsResponse{
			Success: false,
			Message: "Table configuration is required",
		}), nil
	}
	if err := s.requireDataTableAccess(ctx, tcName, repo.ResourceActionEdit); err != nil {
		return nil, err
	}
	allowed, err := s.displaySettingsAllowedColumns(ctx, tcName)
	if err != nil {
		return nil, err
	}
	if err := s.repo.SetColumnDisplaySettings(ctx, tcName, columnDisplayInputs(req.Msg.GetSettings()), allowed); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&sickrockpb.SetColumnDisplaySettingsResponse{
		Success: true,
		Message: "Column display settings saved",
	}), nil
}

// displaySettingsAllowedColumns returns the columns display settings may target: the table's physical
// columns plus its relation synthetic columns (skipped, with a warning, when they cannot be listed).
func (s *SickRockServer) displaySettingsAllowedColumns(ctx context.Context, tcName string) (map[string]struct{}, error) {
	tc, err := s.repo.GetTableConfiguration(ctx, tcName)
	if err != nil {
		return nil, err
	}
	cols, err := s.repo.ListColumns(ctx, tc)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	allowed := make(map[string]struct{}, len(cols))
	for _, c := range cols {
		allowed[c.Name] = struct{}{}
	}
	syntheticKeys, err := s.repo.ListRelationSyntheticColumnKeys(ctx, tcName)
	if err != nil {
		log.WithError(err).Warn("Failed to list relation synthetic columns for display settings")
	}
	for _, key := range syntheticKeys {
		allowed[key] = struct{}{}
	}
	return allowed, nil
}

func columnDisplayInputs(settings []*sickrockpb.ColumnDisplaySettingInput) []repo.ColumnDisplayInput {
	inputs := make([]repo.ColumnDisplayInput, 0, len(settings))
	for _, s := range settings {
		inputs = append(inputs, repo.ColumnDisplayInput{
			ColumnName:   s.GetColumnName(),
			DisplayLabel: s.GetDisplayLabel(),
			ValuePrefix:  s.GetValuePrefix(),
			ValueSuffix:  s.GetValueSuffix(),
		})
	}
	return inputs
}

func (s *SickRockServer) SetTableRowLabels(ctx context.Context, req *connect.Request[sickrockpb.SetTableRowLabelsRequest]) (*connect.Response[sickrockpb.SetTableRowLabelsResponse], error) {
	tcName := strings.TrimSpace(req.Msg.GetTableConfiguration())
	if tcName == "" {
		return connect.NewResponse(&sickrockpb.SetTableRowLabelsResponse{
			Success: false,
			Message: "Table configuration is required",
		}), nil
	}
	if err := s.requireDataTableAccess(ctx, tcName, repo.ResourceActionEdit); err != nil {
		return nil, err
	}
	if err := s.repo.SetTableRowLabels(ctx, tcName, req.Msg.GetRowName(), req.Msg.GetRowNamePlural()); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&sickrockpb.SetTableRowLabelsResponse{
		Success: true,
		Message: "Row labels saved",
	}), nil
}

func (s *SickRockServer) AddTableColumn(ctx context.Context, req *connect.Request[sickrockpb.AddTableColumnRequest]) (*connect.Response[sickrockpb.GetTableStructureResponse], error) {
	tc, err := s.repo.GetTableConfiguration(ctx, req.Msg.GetPageId())
	if err != nil {
		return nil, err
	}

	f := req.Msg.GetField()
	if f == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("field required"))
	}
	err = s.repo.AddColumn(ctx, tc.Db.String, tc.Table.String, repo.FieldSpec{
		Name:                      f.GetName(),
		Type:                      f.GetType(),
		Required:                  f.GetRequired(),
		DefaultToCurrentTimestamp: f.GetDefaultToCurrentTimestamp(),
	})
	if err != nil {
		return nil, err
	}
	if semantic := columnSemanticForType(f.GetType()); semantic != "" {
		if err := s.repo.SetColumnSemantic(ctx, req.Msg.GetPageId(), f.GetName(), semantic); err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("save column semantic: %w", err))
		}
	}
	return s.GetTableStructure(ctx, &connect.Request[sickrockpb.GetTableStructureRequest]{Msg: &sickrockpb.GetTableStructureRequest{PageId: req.Msg.GetPageId()}})
}

// columnSemanticForType returns the column semantic recorded for a new column of fieldType, or "".
func columnSemanticForType(fieldType string) string {
	switch {
	case fieldType == repo.SemanticTypeUserRef:
		return repo.SemanticTypeUserRef
	case fieldType == repo.SemanticTypeImage:
		return repo.SemanticTypeImage
	case repo.IsDatetimeTypeName(fieldType):
		return repo.SemanticTypeDatetime
	}
	return ""
}

func (s *SickRockServer) CreateTableView(ctx context.Context, req *connect.Request[sickrockpb.CreateTableViewRequest]) (*connect.Response[sickrockpb.CreateTableViewResponse], error) {
	tableName := req.Msg.GetTableName()
	viewName := req.Msg.GetViewName()

	if tableName == "" || viewName == "" {
		return connect.NewResponse(&sickrockpb.CreateTableViewResponse{
			Success: false,
			Message: "Table name and view name are required",
		}), nil
	}
	if err := s.requireDataTableAccess(ctx, tableName, repo.ResourceActionEdit); err != nil {
		return nil, err
	}

	viewType, layoutJSON, columns, err := tableViewInput(req.Msg.GetViewType(), req.Msg.GetLayoutJson(), req.Msg.GetColumns())
	if err != nil {
		return nil, err
	}

	viewID, err := s.repo.CreateTableView(ctx, tableName, viewName, viewType, layoutJSON, columns)
	if err != nil {
		log.Errorf("Failed to create table view: %v", err)
		return connect.NewResponse(&sickrockpb.CreateTableViewResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to create table view: %v", err),
		}), nil
	}

	return connect.NewResponse(&sickrockpb.CreateTableViewResponse{
		Success: true,
		Message: "Table view created successfully",
		ViewId:  toInt32(viewID),
	}), nil
}

// tableViewInput converts a view request's type (default "table"), layout JSON (validated) and columns.
func tableViewInput(viewType, layoutJSON string, pbColumns []*sickrockpb.TableViewColumn) (string, string, []repo.TableViewColumn, error) {
	layoutJSON, err := validateViewLayoutJSON(layoutJSON)
	if err != nil {
		return "", "", nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	var columns []repo.TableViewColumn
	for _, pbCol := range pbColumns {
		columns = append(columns, repo.TableViewColumn{
			ColumnName:  pbCol.GetColumnName(),
			IsVisible:   pbCol.GetIsVisible(),
			ColumnOrder: int(pbCol.GetColumnOrder()),
			SortOrder:   pbCol.GetSortOrder(),
		})
	}
	return cmp.Or(viewType, "table"), layoutJSON, columns, nil
}

func (s *SickRockServer) UpdateTableView(ctx context.Context, req *connect.Request[sickrockpb.UpdateTableViewRequest]) (*connect.Response[sickrockpb.UpdateTableViewResponse], error) {
	viewID := int(req.Msg.GetViewId())
	tableName := req.Msg.GetTableName()
	viewName := req.Msg.GetViewName()

	if viewID <= 0 || tableName == "" || viewName == "" {
		return connect.NewResponse(&sickrockpb.UpdateTableViewResponse{
			Success: false,
			Message: "View ID, table name and view name are required",
		}), nil
	}
	if err := s.requireDataTableAccess(ctx, tableName, repo.ResourceActionEdit); err != nil {
		return nil, err
	}

	viewType, layoutJSON, columns, err := tableViewInput(req.Msg.GetViewType(), req.Msg.GetLayoutJson(), req.Msg.GetColumns())
	if err != nil {
		return nil, err
	}

	err = s.repo.UpdateTableView(ctx, viewID, tableName, viewName, viewType, layoutJSON, columns)
	if err != nil {
		log.Errorf("Failed to update table view: %v", err)
		return connect.NewResponse(&sickrockpb.UpdateTableViewResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to update table view: %v", err),
		}), nil
	}

	return connect.NewResponse(&sickrockpb.UpdateTableViewResponse{
		Success: true,
		Message: "Table view updated successfully",
	}), nil
}

func (s *SickRockServer) GetTableViews(ctx context.Context, req *connect.Request[sickrockpb.GetTableViewsRequest]) (*connect.Response[sickrockpb.GetTableViewsResponse], error) {
	tableName := req.Msg.GetTableName()
	if tableName == "" {
		return connect.NewResponse(&sickrockpb.GetTableViewsResponse{
			Views: []*sickrockpb.TableView{},
		}), nil
	}
	if err := s.requireDataTableAccess(ctx, tableName, repo.ResourceActionView); err != nil {
		return nil, err
	}

	views, err := s.repo.GetTableViews(ctx, tableName)
	if err != nil {
		log.Errorf("Failed to get table views: %v", err)
		return connect.NewResponse(&sickrockpb.GetTableViewsResponse{
			Views: []*sickrockpb.TableView{},
		}), nil
	}

	// Convert repository views to protobuf views
	var pbViews []*sickrockpb.TableView
	for _, view := range views {
		var pbColumns []*sickrockpb.TableViewColumn
		for _, col := range view.Columns {
			pbColumns = append(pbColumns, &sickrockpb.TableViewColumn{
				ColumnName:  col.ColumnName,
				IsVisible:   col.IsVisible,
				ColumnOrder: toInt32(col.ColumnOrder),
				SortOrder:   col.SortOrder,
			})
		}

		viewType := view.ViewType
		if viewType == "" {
			viewType = "table" // Default to "table"
		}

		pbViews = append(pbViews, &sickrockpb.TableView{
			Id:         toInt32(view.ID),
			TableName:  view.TableName,
			ViewName:   view.ViewName,
			IsDefault:  view.IsDefault,
			Columns:    pbColumns,
			ViewType:   viewType,
			LayoutJson: view.LayoutJSON,
		})
	}

	return connect.NewResponse(&sickrockpb.GetTableViewsResponse{
		Views:             pbViews,
		UserDefaultViewId: toInt32(s.userDefaultTableView(ctx, tableName)),
	}), nil
}

// userDefaultTableView returns the calling user's own default view for a table (0 when unset or unknown).
func (s *SickRockServer) userDefaultTableView(ctx context.Context, tableName string) int {
	userID, err := s.getUserIDFromContext(ctx)
	if err != nil {
		return 0
	}
	viewID, err := s.repo.GetUserDefaultTableView(ctx, userID, tableName)
	if err != nil {
		log.WithError(err).Warn("Failed to load user default table view")
		return 0
	}
	return viewID
}

// SetUserDefaultTableView sets (or with view_id 0, clears) the caller's own default view.
// It is a personal preference, so viewing the table is enough.
func (s *SickRockServer) SetUserDefaultTableView(ctx context.Context, req *connect.Request[sickrockpb.SetUserDefaultTableViewRequest]) (*connect.Response[sickrockpb.SetUserDefaultTableViewResponse], error) {
	tableName, userID, err := s.userDefaultTableViewAccess(ctx, req.Msg.GetTableName())
	if err != nil {
		return nil, err
	}
	if err := s.repo.SetUserDefaultTableView(ctx, userID, tableName, int(req.Msg.GetViewId())); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	return connect.NewResponse(&sickrockpb.SetUserDefaultTableViewResponse{}), nil
}

func (s *SickRockServer) userDefaultTableViewAccess(ctx context.Context, tableName string) (string, int, error) {
	tableName = strings.TrimSpace(tableName)
	if tableName == "" {
		return "", 0, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("table_name is required"))
	}
	userID, err := s.getUserIDFromContext(ctx)
	if err != nil {
		return "", 0, err
	}
	if err := s.requireDataTableAccess(ctx, tableName, repo.ResourceActionView); err != nil {
		return "", 0, err
	}
	return tableName, userID, nil
}

const maxViewLayoutJSONBytes = 64 * 1024

// validateViewLayoutJSON checks that an optional view layout is a JSON object of a sensible size.
func validateViewLayoutJSON(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	if len(raw) > maxViewLayoutJSONBytes {
		return "", fmt.Errorf("layout_json is larger than %d bytes", maxViewLayoutJSONBytes)
	}
	var layout map[string]any
	if err := json.Unmarshal([]byte(raw), &layout); err != nil {
		return "", fmt.Errorf("layout_json must be a JSON object: %w", err)
	}
	return raw, nil
}

func (s *SickRockServer) DeleteTableView(ctx context.Context, req *connect.Request[sickrockpb.DeleteTableViewRequest]) (*connect.Response[sickrockpb.DeleteTableViewResponse], error) {
	viewID := int(req.Msg.GetViewId())
	if viewID <= 0 {
		return connect.NewResponse(&sickrockpb.DeleteTableViewResponse{
			Success: false,
			Message: "Invalid view ID",
		}), nil
	}

	viewTable, err := s.repo.GetTableViewTableName(ctx, viewID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if err = s.requireDataTableAccess(ctx, viewTable, repo.ResourceActionEdit); err != nil {
		return nil, err
	}

	return deleteTableViewResponse(s.repo.DeleteTableView(ctx, viewID)), nil
}

func deleteTableViewResponse(err error) *connect.Response[sickrockpb.DeleteTableViewResponse] {
	if err != nil {
		log.Errorf("Failed to delete table view: %v", err)
		return connect.NewResponse(&sickrockpb.DeleteTableViewResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to delete view: %v", err),
		})
	}
	return connect.NewResponse(&sickrockpb.DeleteTableViewResponse{
		Success: true,
		Message: "Table view deleted successfully",
	})
}

func (s *SickRockServer) CreateForeignKey(ctx context.Context, req *connect.Request[sickrockpb.CreateForeignKeyRequest]) (*connect.Response[sickrockpb.CreateForeignKeyResponse], error) {
	if problem := createForeignKeyRequestProblem(req.Msg); problem != "" {
		return connect.NewResponse(&sickrockpb.CreateForeignKeyResponse{Success: false, Message: problem}), nil
	}

	err := s.repo.CreateForeignKey(ctx, req.Msg.GetTableName(), req.Msg.GetColumnName(), req.Msg.GetReferencedTable(), req.Msg.GetReferencedColumn(), req.Msg.GetOnDeleteAction(), req.Msg.GetOnUpdateAction())
	if err != nil {
		log.Errorf("Failed to create foreign key: %v", err)
		return connect.NewResponse(&sickrockpb.CreateForeignKeyResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to create foreign key: %v", err),
		}), nil
	}

	return connect.NewResponse(&sickrockpb.CreateForeignKeyResponse{
		Success: true,
		Message: "Foreign key created successfully",
	}), nil
}

// createForeignKeyRequestProblem returns why a create-foreign-key request is invalid, or "".
func createForeignKeyRequestProblem(msg *sickrockpb.CreateForeignKeyRequest) string {
	if slices.Contains([]string{msg.GetTableName(), msg.GetColumnName(), msg.GetReferencedTable(), msg.GetReferencedColumn()}, "") {
		return "Table name, column name, referenced table, and referenced column are required"
	}
	validActions := []string{"CASCADE", "SET NULL", "RESTRICT", "NO ACTION"}
	if !contains(validActions, msg.GetOnDeleteAction()) || !contains(validActions, msg.GetOnUpdateAction()) {
		return "Invalid action. Must be one of: CASCADE, SET NULL, RESTRICT, NO ACTION"
	}
	return ""
}

func (s *SickRockServer) GetForeignKeys(ctx context.Context, req *connect.Request[sickrockpb.GetForeignKeysRequest]) (*connect.Response[sickrockpb.GetForeignKeysResponse], error) {
	tableName := req.Msg.GetTableName()
	if tableName == "" {
		return connect.NewResponse(&sickrockpb.GetForeignKeysResponse{
			ForeignKeys: []*sickrockpb.ForeignKey{},
		}), nil
	}

	foreignKeys, err := s.repo.GetForeignKeys(ctx, tableName)
	if err != nil {
		log.Errorf("Failed to get foreign keys: %v", err)
		return connect.NewResponse(&sickrockpb.GetForeignKeysResponse{
			ForeignKeys: []*sickrockpb.ForeignKey{},
		}), nil
	}

	// Convert repository foreign keys to protobuf foreign keys
	var pbForeignKeys []*sickrockpb.ForeignKey
	for _, fk := range foreignKeys {
		pbForeignKeys = append(pbForeignKeys, &sickrockpb.ForeignKey{
			RelationId:       toInt32(fk.RelationID),
			ConstraintName:   fk.ConstraintName,
			TableName:        fk.TableName,
			ColumnName:       fk.ColumnName,
			ReferencedTable:  fk.ReferencedTable,
			ReferencedColumn: fk.ReferencedColumn,
			OnDeleteAction:   fk.OnDeleteAction,
			OnUpdateAction:   fk.OnUpdateAction,
			DisplayName:      fk.DisplayName,
			AbsorbIntoItem:   fk.AbsorbIntoItem,
			AbsorbType:       fk.AbsorbType,
		})
	}

	return connect.NewResponse(&sickrockpb.GetForeignKeysResponse{
		ForeignKeys: pbForeignKeys,
	}), nil
}

func (s *SickRockServer) UpdateTableRelation(ctx context.Context, req *connect.Request[sickrockpb.UpdateTableRelationRequest]) (*connect.Response[sickrockpb.UpdateTableRelationResponse], error) {
	constraintName := strings.TrimSpace(req.Msg.GetConstraintName())
	if constraintName == "" {
		return connect.NewResponse(&sickrockpb.UpdateTableRelationResponse{
			Success: false,
			Message: "Constraint name is required",
		}), nil
	}
	rel, err := s.repo.GetTableRelationByConstraintName(ctx, constraintName)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("relation not found"))
	}
	if err := s.requireRelationEditAccess(ctx, rel); err != nil {
		return nil, err
	}
	if req.Msg.AbsorbIntoItem != nil {
		if err := s.repo.SetTableRelationAbsorb(ctx, constraintName, req.Msg.GetAbsorbIntoItem(), req.Msg.GetAbsorbType()); err != nil {
			return connect.NewResponse(&sickrockpb.UpdateTableRelationResponse{
				Success: false,
				Message: fmt.Sprintf("Failed to update relation: %v", err),
			}), nil
		}
	}
	pullThrough := pullThroughFieldsFromProto(req.Msg.GetPullThroughFields())
	if err := s.repo.UpdateTableRelation(ctx, constraintName, req.Msg.GetDisplayName(), pullThrough); err != nil {
		log.Errorf("Failed to update table relation: %v", err)
		return connect.NewResponse(&sickrockpb.UpdateTableRelationResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to update relation: %v", err),
		}), nil
	}
	return connect.NewResponse(&sickrockpb.UpdateTableRelationResponse{
		Success: true,
		Message: "Relation updated",
	}), nil
}

// requireRelationEditAccess allows editing a relation to callers who may edit either of its tables.
func (s *SickRockServer) requireRelationEditAccess(ctx context.Context, rel *repo.TableRelation) error {
	if s.requireTableAccess(ctx, rel.SourceTableKey, repo.ResourceActionEdit) == nil ||
		s.requireTableAccess(ctx, rel.ReferencedTableKey, repo.ResourceActionEdit) == nil {
		return nil
	}
	return connect.NewError(connect.CodePermissionDenied, fmt.Errorf("table edit access required"))
}

func pullThroughFieldsFromProto(in []*sickrockpb.RelationPullThroughField) []repo.RelationPullThroughField {
	if len(in) == 0 {
		return nil
	}
	out := make([]repo.RelationPullThroughField, 0, len(in))
	for _, pt := range in {
		if pt == nil {
			continue
		}
		expr := strings.TrimSpace(pt.GetExpr())
		if expr != "" {
			out = append(out, repo.RelationPullThroughField{
				DisplayLabel: strings.TrimSpace(pt.GetDisplayLabel()),
				Expr:         expr,
			})
			continue
		}
		fkCol := strings.TrimSpace(pt.GetFkColumn())
		field := strings.TrimSpace(pt.GetField())
		if fkCol == "" || field == "" {
			continue
		}
		out = append(out, repo.RelationPullThroughField{FKColumn: fkCol, Field: field})
	}
	return out
}

func pullThroughFieldsToProto(in []repo.RelationPullThroughField) []*sickrockpb.RelationPullThroughField {
	if len(in) == 0 {
		return nil
	}
	out := make([]*sickrockpb.RelationPullThroughField, 0, len(in))
	for _, pt := range in {
		if pt.IsCalculated() {
			out = append(out, &sickrockpb.RelationPullThroughField{
				DisplayLabel: pt.DisplayLabel,
				Expr:         pt.Expr,
			})
			continue
		}
		out = append(out, &sickrockpb.RelationPullThroughField{
			FkColumn: pt.FKColumn,
			Field:    pt.Field,
		})
	}
	return out
}

func (s *SickRockServer) SetItemPageLayout(ctx context.Context, req *connect.Request[sickrockpb.SetItemPageLayoutRequest]) (*connect.Response[sickrockpb.SetItemPageLayoutResponse], error) {
	tcName := strings.TrimSpace(req.Msg.GetTableConfiguration())
	if tcName == "" {
		return connect.NewResponse(&sickrockpb.SetItemPageLayoutResponse{
			Success: false,
			Message: "Table configuration is required",
		}), nil
	}
	if err := s.requireDataTableAccess(ctx, tcName, repo.ResourceActionEdit); err != nil {
		return nil, err
	}
	inputs := make([]repo.PageLayoutBlockInput, 0, len(req.Msg.GetBlocks()))
	for i, b := range req.Msg.GetBlocks() {
		ord := int(b.GetOrdinal())
		if ord <= 0 {
			ord = i + 1
		}
		inputs = append(inputs, repo.PageLayoutBlockInput{
			Ordinal:    ord,
			BlockType:  b.GetBlockType(),
			ConfigJSON: b.GetConfigJson(),
		})
	}
	if err := s.repo.SetItemPageLayoutBlocks(ctx, tcName, inputs); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&sickrockpb.SetItemPageLayoutResponse{
		Success: true,
		Message: "Item page layout saved",
	}), nil
}

func (s *SickRockServer) DeleteForeignKey(ctx context.Context, req *connect.Request[sickrockpb.DeleteForeignKeyRequest]) (*connect.Response[sickrockpb.DeleteForeignKeyResponse], error) {
	constraintName := req.Msg.GetConstraintName()
	if constraintName == "" {
		return connect.NewResponse(&sickrockpb.DeleteForeignKeyResponse{
			Success: false,
			Message: "Constraint name is required",
		}), nil
	}

	err := s.repo.DeleteForeignKey(ctx, constraintName)
	if err != nil {
		log.Errorf("Failed to delete foreign key: %v", err)
		return connect.NewResponse(&sickrockpb.DeleteForeignKeyResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to delete foreign key: %v", err),
		}), nil
	}

	return connect.NewResponse(&sickrockpb.DeleteForeignKeyResponse{
		Success: true,
		Message: "Foreign key deleted successfully",
	}), nil
}

// Helper function to check if a string is in a slice
func contains(slice []string, item string) bool {
	return slices.Contains(slice, item)
}

func (s *SickRockServer) ChangeColumnType(ctx context.Context, req *connect.Request[sickrockpb.ChangeColumnTypeRequest]) (*connect.Response[sickrockpb.ChangeColumnTypeResponse], error) {
	tableName := req.Msg.GetTableName()
	columnName := req.Msg.GetColumnName()
	newType := req.Msg.GetNewType()

	if tableName == "" || columnName == "" || newType == "" {
		return connect.NewResponse(&sickrockpb.ChangeColumnTypeResponse{
			Success: false,
			Message: "Table name, column name, and new type are required",
		}), nil
	}

	// Validate the new type - now accepting native database types
	validTypes := []string{
		"TEXT", "VARCHAR(255)", "VARCHAR(500)", "VARCHAR(1000)",
		"INT", "INT(11)", "INT(10)", "INT(8)",
		"BIGINT", "BIGINT(20)",
		"TINYINT(1)", "TINYINT(4)",
		"DATETIME", "DATE", "TIME", "TIMESTAMP",
		"DOUBLE", "FLOAT", "DECIMAL(10,2)", "DECIMAL(15,4)",
		"BOOLEAN", "CHAR(1)", "LONGTEXT", "MEDIUMTEXT",
	}
	if !contains(validTypes, newType) {
		return connect.NewResponse(&sickrockpb.ChangeColumnTypeResponse{
			Success: false,
			Message: "Invalid type. Must be a valid database type like: " + strings.Join(validTypes[:10], ", ") + "...",
		}), nil
	}

	err := s.repo.ChangeColumnType(ctx, tableName, columnName, newType)
	if err != nil {
		log.Errorf("Failed to change column type: %v", err)
		return connect.NewResponse(&sickrockpb.ChangeColumnTypeResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to change column type: %v", err),
		}), nil
	}

	return connect.NewResponse(&sickrockpb.ChangeColumnTypeResponse{
		Success: true,
		Message: "Column type changed successfully",
	}), nil
}

func (s *SickRockServer) DropColumn(ctx context.Context, req *connect.Request[sickrockpb.DropColumnRequest]) (*connect.Response[sickrockpb.DropColumnResponse], error) {
	tableName := req.Msg.GetTableName()
	columnName := req.Msg.GetColumnName()

	if msg := dropColumnValidation(tableName, columnName); msg != "" {
		return connect.NewResponse(&sickrockpb.DropColumnResponse{
			Success: false,
			Message: msg,
		}), nil
	}

	err := s.repo.DropColumn(ctx, tableName, columnName)
	if err != nil {
		log.Errorf("Failed to drop column: %v", err)
		return connect.NewResponse(&sickrockpb.DropColumnResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to drop column: %v", err),
		}), nil
	}

	return connect.NewResponse(&sickrockpb.DropColumnResponse{
		Success: true,
		Message: "Column dropped successfully",
	}), nil
}

func dropColumnValidation(tableName, columnName string) string {
	if tableName == "" || columnName == "" {
		return "Table name and column name are required"
	}
	if columnName == "id" {
		return "Cannot drop system columns (id)"
	}
	return ""
}

func (s *SickRockServer) ChangeColumnName(ctx context.Context, req *connect.Request[sickrockpb.ChangeColumnNameRequest]) (*connect.Response[sickrockpb.ChangeColumnNameResponse], error) {
	tableName := req.Msg.GetTableName()
	oldColumnName := req.Msg.GetOldColumnName()
	newColumnName := req.Msg.GetNewColumnName()

	if missingChangeColumnNameArgs(tableName, oldColumnName, newColumnName) {
		return connect.NewResponse(&sickrockpb.ChangeColumnNameResponse{
			Success: false,
			Message: "Table name, old column name, and new column name are required",
		}), nil
	}

	if err := s.repo.ChangeColumnName(ctx, tableName, oldColumnName, newColumnName); err != nil {
		log.Errorf("Failed to rename column: %v", err)
		return connect.NewResponse(&sickrockpb.ChangeColumnNameResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to rename column: %v", err),
		}), nil
	}

	return connect.NewResponse(&sickrockpb.ChangeColumnNameResponse{
		Success: true,
		Message: "Column renamed successfully",
	}), nil
}

func missingChangeColumnNameArgs(tableName, oldColumnName, newColumnName string) bool {
	return tableName == "" || oldColumnName == "" || newColumnName == ""
}

func (s *SickRockServer) GetMostRecentlyViewed(ctx context.Context, req *connect.Request[sickrockpb.GetMostRecentlyViewedRequest]) (*connect.Response[sickrockpb.GetMostRecentlyViewedResponse], error) {
	userID, err := s.getUserIDFromContext(ctx)
	if err != nil || userID <= 0 {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	limit := int(req.Msg.GetLimit())
	if limit <= 0 {
		limit = 10 // Default limit
	}

	// Over-fetch so rows from tables the caller can no longer view can be skipped.
	items, err := s.repo.GetMostRecentlyViewed(ctx, userID, max(limit*5, limit))
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&sickrockpb.GetMostRecentlyViewedResponse{
		Items: s.viewableRecentlyViewed(ctx, items, limit),
	}), nil
}

// viewableRecentlyViewed returns up to limit items from tables the caller may view.
func (s *SickRockServer) viewableRecentlyViewed(ctx context.Context, items []repo.RecentlyViewedItem, limit int) []*sickrockpb.RecentlyViewedItem {
	pbItems := make([]*sickrockpb.RecentlyViewedItem, 0, limit)
	for _, item := range items {
		if len(pbItems) >= limit {
			break
		}
		if s.requireTableAccess(ctx, item.Name, repo.ResourceActionView) != nil {
			continue
		}
		pbItems = append(pbItems, &sickrockpb.RecentlyViewedItem{
			Name:          item.Name,
			TableId:       item.TableID,
			Icon:          item.Icon,
			UpdatedAtUnix: item.UpdatedAtUnix,
			ItemName:      item.ItemName,
			TableTitle:    displayNavigationTitle(item.TableTitle),
		})
	}
	return pbItems
}

func (s *SickRockServer) GetSystemInfo(ctx context.Context, req *connect.Request[sickrockpb.GetSystemInfoRequest]) (*connect.Response[sickrockpb.GetSystemInfoResponse], error) {
	if err := s.requireSystemSettings(ctx); err != nil {
		return nil, err
	}
	total, err := s.repo.GetApproxTotalRows(ctx)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&sickrockpb.GetSystemInfoResponse{ApproxTotalRows: total}), nil
}

func (s *SickRockServer) CreateDashboard(ctx context.Context, req *connect.Request[sickrockpb.CreateDashboardRequest]) (*connect.Response[sickrockpb.CreateDashboardResponse], error) {
	name, createNavigation, ok := createDashboardArgs(req.Msg)
	if !ok {
		return connect.NewResponse(&sickrockpb.CreateDashboardResponse{
			Success: false,
			Message: "Dashboard name is required",
		}), nil
	}

	dashboardID, err := s.repo.CreateDashboard(ctx, name, createNavigation)
	if err != nil {
		log.Errorf("Failed to create dashboard: %v", err)
		return connect.NewResponse(&sickrockpb.CreateDashboardResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to create dashboard: %v", err),
		}), nil
	}
	if grantErr := s.repo.EnsureEveryoneDashboardGrant(ctx, name); grantErr != nil {
		log.Warnf("Failed to seed Everyone grant for dashboard %s: %v", name, grantErr)
	}

	return connect.NewResponse(&sickrockpb.CreateDashboardResponse{
		Success:     true,
		Message:     "Dashboard created successfully",
		DashboardId: toInt32(dashboardID),
	}), nil
}

func createDashboardArgs(msg *sickrockpb.CreateDashboardRequest) (string, bool, bool) {
	name := strings.TrimSpace(msg.GetName())
	if name == "" {
		return "", false, false
	}
	createNavigation := true
	if msg.CreateNavigationEntry != nil {
		createNavigation = msg.GetCreateNavigationEntry()
	}
	return name, createNavigation, true
}

func (s *SickRockServer) GetDashboards(ctx context.Context, req *connect.Request[sickrockpb.GetDashboardsRequest]) (*connect.Response[sickrockpb.GetDashboardsResponse], error) {
	dashboards, err := s.repo.ListDashboards(ctx)
	if err != nil {
		return nil, err
	}

	log.Infof("Dashboards: %v", dashboards)

	out := make([]*sickrockpb.Dashboard, 0, len(dashboards))
	for _, d := range dashboards {
		if err := s.requireDashboardAccess(ctx, d.Name); err != nil {
			continue
		}
		comps, err := s.repo.ListDashboardComponents(ctx, d.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, &sickrockpb.Dashboard{Id: toInt32(d.ID), Name: d.Name, Components: s.dashboardComponentsToProto(ctx, comps)})
	}
	return connect.NewResponse(&sickrockpb.GetDashboardsResponse{Dashboards: out}), nil
}

// dashboardComponentsToProto evaluates each component in order; each component's value is exposed by
// name to the formulas of later components.
func (s *SickRockServer) dashboardComponentsToProto(ctx context.Context, comps []repo.DashboardComponent) []*sickrockpb.DashboardComponent {
	runningEnv := make(map[string]any)
	runningEnv["round1"] = func(in float64) (float64, error) {
		return math.Round(in*10) / 10, nil
	}

	pbComps := make([]*sickrockpb.DashboardComponent, 0, len(comps))
	for _, c := range comps {
		data, err := s.getDashboardComponentData(ctx, c, &runningEnv)
		runningEnv[c.Name] = data
		pbComps = append(pbComps, s.dashboardComponentToProto(ctx, c, data, err))
	}
	return pbComps
}

// dashboardComponentToProto converts an evaluated component, applying its rules (loaded server side
// and not returned to the client), or reports dataErr on the component.
func (s *SickRockServer) dashboardComponentToProto(ctx context.Context, c repo.DashboardComponent, data any, dataErr error) *sickrockpb.DashboardComponent {
	if dataErr != nil {
		log.WithError(dataErr).WithField("component", c.ID).Warn("Failed to load dashboard component data")
		return &sickrockpb.DashboardComponent{
			Id:    toInt32(c.ID),
			Name:  c.Name,
			Error: dataErr.Error(),
		}
	}
	rules, err := s.repo.GetDashboardComponentRules(ctx, &c.ID)
	if err != nil {
		log.WithError(err).WithField("component", c.ID).Warn("Failed to load dashboard component rules")
	}
	pbComp := &sickrockpb.DashboardComponent{
		Id:         toInt32(c.ID),
		Name:       c.Name,
		DataString: fmt.Sprintf("%v", data),
	}
	s.applyRules(pbComp, rules)
	return pbComp
}

func (s *SickRockServer) getDashboardComponentData(ctx context.Context, comp repo.DashboardComponent, runningEnv *map[string]any) (any, error) {
	formula := strings.TrimSpace(comp.Formula.String)

	switch formula {
	case "":
		return "", nil
	case "latest":
		// Special case: the column's value in the table's latest row
		fields, err := s.latestComponentRow(ctx, comp)
		if err != nil {
			return "", err
		}
		return fields[comp.ColumnName.String], nil
	}

	// Evaluate the expression with earlier components' values and latest() available
	env := *runningEnv
	env["latest"] = func() (map[string]any, error) {
		return s.latestComponentRow(ctx, comp)
	}

	program, err := expr.Compile(formula, expr.Env(env))
	if err != nil {
		return "", fmt.Errorf("failed to compile expression '%s': %w", formula, err)
	}

	result, err := expr.Run(program, env)
	if err != nil {
		return "", fmt.Errorf("failed to evaluate expression '%s': %w", formula, err)
	}

	return result, nil
}

// latestComponentRow returns the fields of the latest row in the component's table.
func (s *SickRockServer) latestComponentRow(ctx context.Context, comp repo.DashboardComponent) (map[string]any, error) {
	if !comp.TcID.Valid {
		return nil, fmt.Errorf("tc_id is not valid for component %d", comp.ID)
	}
	item, err := s.repo.GetLastItem(ctx, int(comp.TcID.Int32))
	if err != nil {
		return nil, err
	}
	return item.Fields, nil
}

func (s *SickRockServer) applyRules(comp *sickrockpb.DashboardComponent, rules []repo.DashboardComponentRule) {
	for _, r := range rules {
		log.Infof("Applying rule %+v to component %v", r, comp.Name)

		switch r.Operation {
		case "suffix":
			comp.Suffix = r.Operand
		default:
			log.Warnf("Unknown operation %s", r.Operation)
		}
	}
}

func (s *SickRockServer) GetDashboardComponentRules(ctx context.Context, req *connect.Request[sickrockpb.GetDashboardComponentRulesRequest]) (*connect.Response[sickrockpb.GetDashboardComponentRulesResponse], error) {
	rules, err := s.repo.GetDashboardComponentRules(ctx, dashboardComponentFilter(req.Msg))
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&sickrockpb.GetDashboardComponentRulesResponse{Rules: dashboardComponentRulesToProto(rules)}), nil
}

func dashboardComponentFilter(msg *sickrockpb.GetDashboardComponentRulesRequest) *int {
	if msg == nil || msg.GetComponent() == 0 {
		return nil
	}
	v := int(msg.GetComponent())
	return &v
}

func dashboardComponentRulesToProto(rules []repo.DashboardComponentRule) []*sickrockpb.DashboardComponentRule {
	out := make([]*sickrockpb.DashboardComponentRule, 0, len(rules))
	for _, rle := range rules {
		out = append(out, &sickrockpb.DashboardComponentRule{
			Id:        toInt32(rle.ID),
			Component: toInt32(rle.Component),
			Ordinal:   toInt32(rle.Ordinal),
			Operation: rle.Operation,
			Operand:   rle.Operand,
		})
	}
	return out
}

func (s *SickRockServer) CreateDashboardComponentRule(ctx context.Context, req *connect.Request[sickrockpb.CreateDashboardComponentRuleRequest]) (*connect.Response[sickrockpb.CreateDashboardComponentRuleResponse], error) {
	component := int(req.Msg.GetComponent())
	ordinal := int(req.Msg.GetOrdinal())
	operation := strings.TrimSpace(req.Msg.GetOperation())
	operand := req.Msg.GetOperand()
	if component <= 0 || operation == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("component and operation are required"))
	}
	rule, err := s.repo.CreateDashboardComponentRule(ctx, component, ordinal, operation, operand)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&sickrockpb.CreateDashboardComponentRuleResponse{Rule: &sickrockpb.DashboardComponentRule{
		Id:        toInt32(rule.ID),
		Component: toInt32(rule.Component),
		Ordinal:   toInt32(rule.Ordinal),
		Operation: rule.Operation,
		Operand:   rule.Operand,
	}}), nil
}

// GetUserBookmarks retrieves all bookmarks for the authenticated user
func (s *SickRockServer) GetUserBookmarks(ctx context.Context, req *connect.Request[sickrockpb.GetUserBookmarksRequest]) (*connect.Response[sickrockpb.GetUserBookmarksResponse], error) {
	userID, err := s.getUserIDFromContext(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	bookmarks, err := s.repo.GetUserBookmarks(ctx, userID)
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&sickrockpb.GetUserBookmarksResponse{Bookmarks: userBookmarksToProto(bookmarks)}), nil
}

func userBookmarksToProto(bookmarks []repo.UserBookmark) []*sickrockpb.UserBookmark {
	pbBookmarks := make([]*sickrockpb.UserBookmark, 0, len(bookmarks))
	for _, bookmark := range bookmarks {
		pbBookmarks = append(pbBookmarks, userBookmarkToProto(bookmark, true))
	}
	return pbBookmarks
}

func userBookmarkToProto(bookmark repo.UserBookmark, includeTitle bool) *sickrockpb.UserBookmark {
	pb := &sickrockpb.UserBookmark{
		Id:               toInt32(bookmark.ID),
		UserId:           toInt32(bookmark.UserID),
		NavigationItemId: toInt32(bookmark.NavigationItemID),
		NavigationItem:   bookmarkNavigationItem(bookmark),
	}
	if includeTitle {
		pb.Title = bookmarkLinkTitle(bookmark)
	}
	return pb
}

func bookmarkNavigationItem(bookmark repo.UserBookmark) *sickrockpb.NavigationItem {
	if bookmark.NavigationItem == nil {
		return nil
	}
	return navigationItemRepoToProto(*bookmark.NavigationItem, bookmark.NavigationItem.TableView.String)
}

// CreateUserBookmark creates a new bookmark for the authenticated user
func (s *SickRockServer) CreateUserBookmark(ctx context.Context, req *connect.Request[sickrockpb.CreateUserBookmarkRequest]) (*connect.Response[sickrockpb.CreateUserBookmarkResponse], error) {
	userID, err := s.getUserIDFromContext(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	navigationItemID := int(req.Msg.GetNavigationItemId())
	if navigationItemID <= 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("navigation item ID is required"))
	}

	bookmark, err := s.repo.CreateUserBookmark(ctx, userID, navigationItemID)
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&sickrockpb.CreateUserBookmarkResponse{Bookmark: userBookmarkToProto(*bookmark, false)}), nil
}

// DeleteUserBookmark removes a bookmark for the authenticated user
func (s *SickRockServer) DeleteUserBookmark(ctx context.Context, req *connect.Request[sickrockpb.DeleteUserBookmarkRequest]) (*connect.Response[sickrockpb.DeleteUserBookmarkResponse], error) {
	userID, err := s.getUserIDFromContext(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	bookmarkID := int(req.Msg.GetBookmarkId())
	if bookmarkID <= 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("bookmark ID is required"))
	}

	err = s.repo.DeleteUserBookmark(ctx, userID, bookmarkID)
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&sickrockpb.DeleteUserBookmarkResponse{Deleted: true}), nil
}

// Tick list state (shared across clients)

// GetTickListState returns the completion state for a table's tick list.
func (s *SickRockServer) GetTickListState(ctx context.Context, req *connect.Request[sickrockpb.GetTickListStateRequest]) (*connect.Response[sickrockpb.GetTickListStateResponse], error) {
	if _, err := s.getUserIDFromContext(ctx); err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}
	tcName := strings.TrimSpace(req.Msg.GetTcName())
	if tcName == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("tc_name is required"))
	}
	state, err := s.repo.GetTickListState(ctx, tcName)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&sickrockpb.GetTickListStateResponse{CompletedByItemId: state}), nil
}

// SetTickListCompletion sets one item's completed state for a table's tick list.
func (s *SickRockServer) SetTickListCompletion(ctx context.Context, req *connect.Request[sickrockpb.SetTickListCompletionRequest]) (*connect.Response[sickrockpb.SetTickListCompletionResponse], error) {
	if _, err := s.getUserIDFromContext(ctx); err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}
	tcName, itemID, err := tickListCompletionArgs(req.Msg)
	if err != nil {
		return nil, err
	}
	if err := s.repo.SetTickListCompletion(ctx, tcName, itemID, req.Msg.GetCompleted()); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&sickrockpb.SetTickListCompletionResponse{}), nil
}

func tickListCompletionArgs(msg *sickrockpb.SetTickListCompletionRequest) (string, string, error) {
	tcName := strings.TrimSpace(msg.GetTcName())
	itemID := strings.TrimSpace(msg.GetItemId())
	if tcName == "" || itemID == "" {
		return "", "", connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("tc_name and item_id are required"))
	}
	return tcName, itemID, nil
}

// ClearTickListState removes all tick list completion state for a table.
func (s *SickRockServer) ClearTickListState(ctx context.Context, req *connect.Request[sickrockpb.ClearTickListStateRequest]) (*connect.Response[sickrockpb.ClearTickListStateResponse], error) {
	if _, err := s.getUserIDFromContext(ctx); err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}
	tcName := strings.TrimSpace(req.Msg.GetTcName())
	if tcName == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("tc_name is required"))
	}
	if err := s.repo.ClearTickListState(ctx, tcName); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&sickrockpb.ClearTickListStateResponse{}), nil
}

// API Key Management Methods

// CreateAPIKey creates a new API key for the authenticated user
func (s *SickRockServer) CreateAPIKey(ctx context.Context, req *connect.Request[sickrockpb.CreateAPIKeyRequest]) (*connect.Response[sickrockpb.CreateAPIKeyResponse], error) {
	userID, err := s.getUserIDFromContext(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	name := strings.TrimSpace(req.Msg.GetName())
	if name == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("API key name is required"))
	}

	apiKey, keyID, err := s.createStoredAPIKey(ctx, userID, name, req.Msg.GetReadOnly())
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&sickrockpb.CreateAPIKeyResponse{
		Success:  true,
		Message:  "API key created successfully",
		ApiKey:   apiKey,
		ApiKeyId: toInt32(keyID),
	}), nil
}

func (s *SickRockServer) createStoredAPIKey(ctx context.Context, userID int, name string, readOnly bool) (string, int, error) {
	apiKey, err := armpassword.GenerateAPIKey("sk_")
	if err != nil {
		return "", 0, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to generate API key: %w", err))
	}
	keyID, err := s.auth.Store.CreateAPIKey(ctx, userID, name, apiKey, readOnly)
	if err != nil {
		return "", 0, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to create API key: %w", err))
	}
	return apiKey, keyID, nil
}

// GetAPIKeys retrieves all API keys for the authenticated user
func (s *SickRockServer) GetAPIKeys(ctx context.Context, req *connect.Request[sickrockpb.GetAPIKeysRequest]) (*connect.Response[sickrockpb.GetAPIKeysResponse], error) {
	userID, err := s.getUserIDFromContext(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	apiKeys, err := s.auth.Store.ListAPIKeysForUser(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to retrieve API keys: %w", err))
	}

	var pbAPIKeys []*sickrockpb.APIKey
	for _, apiKey := range apiKeys {
		pbAPIKeys = append(pbAPIKeys, &sickrockpb.APIKey{
			Id:         toInt32(apiKey.ID),
			UserId:     toInt32(apiKey.UserAccountID),
			Name:       apiKey.Name,
			CreatedAt:  parseIAMTimestamp(apiKey.CreatedAt),
			LastUsedAt: parseIAMTimestamp(apiKey.LastUsedAt),
			IsActive:   true,
			ReadOnly:   apiKey.ReadOnly,
		})
	}

	return connect.NewResponse(&sickrockpb.GetAPIKeysResponse{
		ApiKeys: pbAPIKeys,
	}), nil
}

// DeleteAPIKey permanently deletes an API key
func (s *SickRockServer) DeleteAPIKey(ctx context.Context, req *connect.Request[sickrockpb.DeleteAPIKeyRequest]) (*connect.Response[sickrockpb.DeleteAPIKeyResponse], error) {
	userID, err := s.getUserIDFromContext(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	apiKeyID := int(req.Msg.GetApiKeyId())
	if apiKeyID <= 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("API key ID is required"))
	}

	err = s.auth.Store.DeleteAPIKey(ctx, apiKeyID, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to delete API key: %w", err))
	}

	return connect.NewResponse(&sickrockpb.DeleteAPIKeyResponse{
		Success: true,
		Message: "API key deleted successfully",
	}), nil
}

// DeactivateAPIKey deactivates an API key
func (s *SickRockServer) DeactivateAPIKey(ctx context.Context, req *connect.Request[sickrockpb.DeactivateAPIKeyRequest]) (*connect.Response[sickrockpb.DeactivateAPIKeyResponse], error) {
	userID, err := s.getUserIDFromContext(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	apiKeyID := int(req.Msg.GetApiKeyId())
	if apiKeyID <= 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("API key ID is required"))
	}

	err = s.auth.Store.DeleteAPIKey(ctx, apiKeyID, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to deactivate API key: %w", err))
	}

	return connect.NewResponse(&sickrockpb.DeactivateAPIKeyResponse{
		Success: true,
		Message: "API key deleted successfully",
	}), nil
}

// UpdateAPIKey is not supported with armature-iam keys (delete and recreate instead).
func (s *SickRockServer) UpdateAPIKey(ctx context.Context, req *connect.Request[sickrockpb.UpdateAPIKeyRequest]) (*connect.Response[sickrockpb.UpdateAPIKeyResponse], error) {
	if _, err := s.getUserIDFromContext(ctx); err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}
	return nil, connect.NewError(connect.CodeUnimplemented, fmt.Errorf("update API key is not supported; delete and create a new key"))
}

// GetConditionalFormattingRules retrieves conditional formatting rules
func (s *SickRockServer) GetConditionalFormattingRules(ctx context.Context, req *connect.Request[sickrockpb.GetConditionalFormattingRulesRequest]) (*connect.Response[sickrockpb.GetConditionalFormattingRulesResponse], error) {
	userID, err := s.getUserIDFromContext(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	tableName := req.Msg.GetTableName()
	rules, err := s.repo.GetConditionalFormattingRules(ctx, userID, tableName)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get conditional formatting rules: %w", err))
	}

	var pbRules []*sickrockpb.ConditionalFormattingRule
	for _, rule := range rules {
		pbRules = append(pbRules, &sickrockpb.ConditionalFormattingRule{
			Id:             toInt32(rule.ID),
			TableName:      rule.TableName,
			ColumnName:     rule.ColumnName,
			ConditionType:  rule.ConditionType,
			ConditionValue: rule.ConditionValue,
			FormatType:     rule.FormatType,
			FormatValue:    rule.FormatValue,
			Priority:       toInt32(rule.Priority),
			IsActive:       rule.IsActive,
			SrCreated:      rule.SrCreated.Unix(),
			UpdatedAtUnix:  rule.UpdatedAtUnix,
		})
	}

	return connect.NewResponse(&sickrockpb.GetConditionalFormattingRulesResponse{
		Rules: pbRules,
	}), nil
}

// CreateConditionalFormattingRule creates a new conditional formatting rule
func (s *SickRockServer) CreateConditionalFormattingRule(ctx context.Context, req *connect.Request[sickrockpb.CreateConditionalFormattingRuleRequest]) (*connect.Response[sickrockpb.CreateConditionalFormattingRuleResponse], error) {
	userID, err := s.getUserIDFromContext(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	ruleID, err := s.repo.CreateConditionalFormattingRule(ctx, userID, &repo.ConditionalFormattingRule{
		TableName:      req.Msg.GetTableName(),
		ColumnName:     req.Msg.GetColumnName(),
		ConditionType:  req.Msg.GetConditionType(),
		ConditionValue: req.Msg.GetConditionValue(),
		FormatType:     req.Msg.GetFormatType(),
		FormatValue:    req.Msg.GetFormatValue(),
		Priority:       int(req.Msg.GetPriority()),
		IsActive:       true,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to create conditional formatting rule: %w", err))
	}

	return connect.NewResponse(&sickrockpb.CreateConditionalFormattingRuleResponse{
		Success: true,
		Message: "Conditional formatting rule created successfully",
		RuleId:  toInt32(ruleID),
	}), nil
}

// DeleteConditionalFormattingRule deletes a conditional formatting rule
func (s *SickRockServer) DeleteConditionalFormattingRule(ctx context.Context, req *connect.Request[sickrockpb.DeleteConditionalFormattingRuleRequest]) (*connect.Response[sickrockpb.DeleteConditionalFormattingRuleResponse], error) {
	userID, err := s.getUserIDFromContext(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	ruleID := int(req.Msg.GetRuleId())
	if ruleID <= 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("rule ID is required"))
	}

	err = s.repo.DeleteConditionalFormattingRule(ctx, userID, ruleID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to delete conditional formatting rule: %w", err))
	}

	return connect.NewResponse(&sickrockpb.DeleteConditionalFormattingRuleResponse{
		Success: true,
		Message: "Conditional formatting rule deleted successfully",
	}), nil
}

// UpdateConditionalFormattingRule updates an existing conditional formatting rule
func (s *SickRockServer) UpdateConditionalFormattingRule(ctx context.Context, req *connect.Request[sickrockpb.UpdateConditionalFormattingRuleRequest]) (*connect.Response[sickrockpb.UpdateConditionalFormattingRuleResponse], error) {
	userID, err := s.getUserIDFromContext(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	ruleID := int(req.Msg.GetRuleId())
	if ruleID <= 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("rule ID is required"))
	}

	err = s.repo.UpdateConditionalFormattingRule(ctx, userID, &repo.ConditionalFormattingRule{
		ID:             ruleID,
		TableName:      req.Msg.GetTableName(),
		ColumnName:     req.Msg.GetColumnName(),
		ConditionType:  req.Msg.GetConditionType(),
		ConditionValue: req.Msg.GetConditionValue(),
		FormatType:     req.Msg.GetFormatType(),
		FormatValue:    req.Msg.GetFormatValue(),
		Priority:       int(req.Msg.GetPriority()),
		IsActive:       req.Msg.GetIsActive(),
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to update conditional formatting rule: %w", err))
	}

	return connect.NewResponse(&sickrockpb.UpdateConditionalFormattingRuleResponse{
		Success: true,
		Message: "Conditional formatting rule updated successfully",
	}), nil
}
