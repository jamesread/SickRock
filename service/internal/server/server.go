package server

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode"

	"connectrpc.com/connect"
	"github.com/expr-lang/expr"
	armpassword "github.com/jamesread/armature-iam/password"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"

	sickrockpb "github.com/jamesread/SickRock/gen/proto"
	"github.com/jamesread/SickRock/internal/buildinfo"
	"github.com/jamesread/SickRock/internal/health"
	"github.com/jamesread/SickRock/internal/iam"
	"github.com/jamesread/SickRock/internal/notifications"
	repo "github.com/jamesread/SickRock/internal/repo"
	log "github.com/sirupsen/logrus"
)

type SickRockServer struct {
	repo                *repo.Repository
	notificationService *notifications.NotificationService
	auth                *iam.AuthLayer
	health              *health.Service
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

	var currentUsername string
	var rbacPerms []string
	var rbacSuperuser bool

	if au := s.authUser(ctx); au != nil && au.User != nil {
		currentUsername, rbacPerms, rbacSuperuser = iamUserToProto(au)
	} else if sessionUser, err := s.resolveSessionUser(ctx, req); err == nil && sessionUser != nil {
		currentUsername, rbacPerms, rbacSuperuser = iamUserToProto(sessionUser)
	}

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
			Ordinal:  int32(config.Ordinal),
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

	dbTables := make([]*sickrockpb.DatabaseTable, 0, len(tables))
	for _, table := range tables {
		dbTables = append(dbTables, &sickrockpb.DatabaseTable{
			TableName:         table.TableName,
			HasConfiguration:  table.HasConfiguration,
			ConfigurationName: table.ConfigurationName.String,
			View:              "table", // Default view type is always "table"
		})
	}

	res := connect.NewResponse(&sickrockpb.GetDatabaseTablesResponse{Tables: dbTables})
	return res, nil
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
		Id:      int32(item.ID),
		Ordinal: int32(item.Ordinal),
		TableConfiguration: func() int32 {
			if item.TableConfiguration.Valid {
				return int32(item.TableConfiguration.Int64)
			}
			return 0
		}(),
		TableName:  item.TableName.String,
		TableTitle: displayNavigationTitle(item.TableTitle.String),
		Icon:       item.Icon.String,
		TableView:  tableView,
		DashboardId: func() int32 {
			if item.DashboardID.Valid {
				return int32(item.DashboardID.Int64)
			}
			return 0
		}(),
		DashboardName: item.DashboardName.String,
		Title:         navigationLinkTitle(item),
		WorkflowId: func() int32 {
			if item.WorkflowID.Valid {
				return int32(item.WorkflowID.Int64)
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

	visibleByID := make(map[int]repo.NavigationItem)
	navigationItems := make([]*sickrockpb.NavigationItem, 0, len(items))
	for _, item := range items {
		if !s.canViewNavigationItem(ctx, item) {
			continue
		}
		visibleByID[item.ID] = item
		navigationItems = append(navigationItems, navigationItemRepoToProto(item, ""))
	}

	var navigationApps []*sickrockpb.NavigationApp
	assignedNavIDs, errAssigned := s.repo.NavigationItemIDsInApps(ctx)
	if errAssigned != nil {
		log.Warnf("Failed to load app navigation assignments: %v", errAssigned)
		assignedNavIDs = map[int]struct{}{}
	}
	navApps, errApps := s.repo.GetNavigationApps(ctx)
	if errApps != nil {
		log.Warnf("Failed to load navigation apps: %v", errApps)
	} else {
		for _, app := range navApps {
			members := make([]*sickrockpb.NavigationItem, 0, len(app.MemberIDs))
			for _, navID := range app.MemberIDs {
				if item, ok := visibleByID[navID]; ok {
					members = append(members, navigationItemRepoToProto(item, ""))
				}
			}
			if len(members) == 0 {
				continue
			}
			navigationApps = append(navigationApps, &sickrockpb.NavigationApp{
				Id:      int32(app.ID),
				Title:   app.Title,
				Ordinal: int32(app.Ordinal),
				Items:   members,
			})
		}
	}
	ungroupedItems := make([]*sickrockpb.NavigationItem, 0, len(visibleByID))
	for _, item := range items {
		if _, visible := visibleByID[item.ID]; !visible {
			continue
		}
		if _, assigned := assignedNavIDs[item.ID]; assigned {
			continue
		}
		ungroupedItems = append(ungroupedItems, navigationItemRepoToProto(item, ""))
	}

	// Get user bookmarks if authenticated
	var bookmarks []*sickrockpb.UserBookmark
	userID, err := s.getUserIDFromContext(ctx)
	if err == nil && userID > 0 {
		// User is authenticated, get their bookmarks
		userBookmarks, err := s.repo.GetUserBookmarks(ctx, userID)
		if err != nil {
			log.Warnf("Failed to load user bookmarks: %v", err)
		} else {
			// Convert to protobuf format
			for _, bookmark := range userBookmarks {
				if bookmark.NavigationItem != nil && !s.canViewNavigationItem(ctx, *bookmark.NavigationItem) {
					continue
				}
				var navItem *sickrockpb.NavigationItem
				if bookmark.NavigationItem != nil {
					navItem = navigationItemRepoToProto(*bookmark.NavigationItem, bookmark.NavigationItem.TableView.String)
				}

				bookmarks = append(bookmarks, &sickrockpb.UserBookmark{
					Id:               int32(bookmark.ID),
					UserId:           int32(bookmark.UserID),
					NavigationItemId: int32(bookmark.NavigationItemID),
					NavigationItem:   navItem,
					Title:            bookmarkLinkTitle(bookmark),
				})
			}
		}
	}

	// Get workflows and group navigation items by workflow
	workflows, err := s.repo.GetWorkflows(ctx)
	if err != nil {
		log.Warnf("Failed to load workflows: %v", err)
		workflows = []repo.Workflow{}
	}

	// Workflow hub members (workflow_navigation_members), not table_navigation.workflow_id
	workflowItemsMap := make(map[int32][]*sickrockpb.NavigationItem)
	memberGroups, err := s.repo.GetNavigationItemsForWorkflowMembership(ctx)
	if err != nil {
		log.Warnf("Failed to load workflow navigation members: %v", err)
	} else {
		for workflowID, memberItems := range memberGroups {
			if s.requireWorkflowAccess(ctx, workflowID, repo.ResourceActionView) != nil {
				continue
			}
			for _, memberItem := range memberItems {
				if !s.canViewNavigationItem(ctx, memberItem) {
					continue
				}
				workflowItemsMap[int32(workflowID)] = append(workflowItemsMap[int32(workflowID)], navigationItemRepoToProto(memberItem, ""))
			}
		}
	}

	// Convert workflows to protobuf format (only workflows the user may view and use)
	workflowProtos := make([]*sickrockpb.Workflow, 0, len(workflows))
	for _, workflow := range workflows {
		if s.requireWorkflowAccess(ctx, workflow.ID, repo.ResourceActionView) != nil {
			continue
		}
		members := workflowItemsMap[int32(workflow.ID)]
		if len(members) == 0 {
			hasHubNavLink := false
			for _, item := range items {
				if !item.WorkflowID.Valid || int(item.WorkflowID.Int64) != workflow.ID {
					continue
				}
				if item.TableName.Valid && item.TableName.String != "" {
					continue
				}
				if item.DashboardName.Valid && item.DashboardName.String != "" {
					continue
				}
				if s.canViewNavigationItem(ctx, item) {
					hasHubNavLink = true
					break
				}
			}
			if !hasHubNavLink {
				continue
			}
		}
		workflowProto := &sickrockpb.Workflow{
			Id:      int32(workflow.ID),
			Name:    workflow.Name,
			Ordinal: int32(workflow.Ordinal),
			Icon:    workflow.Icon.String,
			Items:   members,
		}
		workflowProtos = append(workflowProtos, workflowProto)
	}

	res := connect.NewResponse(&sickrockpb.GetNavigationResponse{
		Items:          navigationItems,
		Bookmarks:      bookmarks,
		Workflows:      workflowProtos,
		Apps:           navigationApps,
		UngroupedItems: ungroupedItems,
	})
	return res, nil
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
		return connect.NewResponse(&sickrockpb.SetWorkflowNavigationMembersResponse{
			Success: false,
			Message: err.Error(),
		}), nil
	}

	members := make([]repo.WorkflowNavigationMemberInput, 0, len(req.Msg.GetMembers()))
	for _, m := range req.Msg.GetMembers() {
		members = append(members, repo.WorkflowNavigationMemberInput{
			NavigationItemID: int(m.GetNavigationItemId()),
			Ordinal:          int(m.GetOrdinal()),
		})
	}

	if err := s.repo.SetWorkflowNavigationMembers(ctx, workflowID, members); err != nil {
		return connect.NewResponse(&sickrockpb.SetWorkflowNavigationMembersResponse{
			Success: false,
			Message: err.Error(),
		}), nil
	}

	return connect.NewResponse(&sickrockpb.SetWorkflowNavigationMembersResponse{
		Success: true,
		Message: "Workflow navigation members updated",
	}), nil
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

	// Build where map from request
	where := map[string]string{}
	for k, v := range req.Msg.GetWhere() {
		if k == "" {
			continue
		}
		where[k] = v
	}

	items, err := s.repo.ListItemsInTable(ctx, table, where)

	if err != nil {
		return nil, err
	}

	constraintName := strings.TrimSpace(req.Msg.GetRelationConstraintName())
	if constraintName != "" {
		rel, relErr := s.repo.GetTableRelationByConstraintName(ctx, constraintName)
		if relErr != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("relation not found: %w", relErr))
		}
		if !strings.EqualFold(strings.TrimSpace(rel.SourceTableKey), strings.TrimSpace(table)) {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("relation %q is not for table %q", constraintName, table))
		}
		columns, parseErr := repo.ParseRelationPullThroughJSON(rel.PullThroughJSON)
		if parseErr != nil {
			return nil, connect.NewError(connect.CodeInternal, parseErr)
		}
		enriched, enrichErr := s.repo.ApplyRelationCalculatedColumns(ctx, rel.SourceTableKey, columns, items)
		if enrichErr != nil {
			log.WithError(enrichErr).WithFields(log.Fields{
				"constraint": constraintName,
				"table":      table,
			}).Warn("ListItems: relation calculated columns skipped")
		} else {
			items = enriched
		}
	} else {
		enriched, enrichErr := s.repo.ApplyAllSourceRelationCalculatedColumns(ctx, table, items)
		if enrichErr != nil {
			log.WithError(enrichErr).WithField("table", table).Warn("ListItems: source relation calculated columns skipped")
		} else {
			items = enriched
		}
	}

	// Get conditional formatting rules once for all items (not per item)
	var rules []*repo.ConditionalFormattingRule
	userID, err := s.getUserIDFromContext(ctx)
	if err == nil {
		rules, err = s.repo.GetConditionalFormattingRules(ctx, userID, table)
		if err == nil {
			log.WithFields(log.Fields{
				"table":     table,
				"userID":    userID,
				"ruleCount": len(rules),
			}).Info("ListItems: Retrieved conditional formatting rules")
		} else {
			log.WithError(err).WithFields(log.Fields{
				"table":  table,
				"userID": userID,
			}).Error("ListItems: Failed to get conditional formatting rules")
		}
	} else {
		log.WithError(err).Error("ListItems: Failed to get user ID from context")
	}

	out := make([]*sickrockpb.Item, 0, len(items))
	for _, it := range items {
		// Convert dynamic fields to string map for protobuf
		additionalFields := make(map[string]string)
		for key, value := range it.Fields {
			if value != nil {
				// Special handling for time.Time values to ensure consistent formatting
				if timeVal, ok := value.(time.Time); ok {
					// Format as MySQL datetime format (YYYY-MM-DD HH:MM:SS) without timezone
					additionalFields[key] = timeVal.Format("2006-01-02 15:04:05")
				} else {
					additionalFields[key] = fmt.Sprintf("%v", value)
				}
			}
		}

		// Process markdown formatting rules (using rules fetched once above)
		if err == nil && rules != nil {

			// Find markdown rules and render markdown for applicable fields
			for _, rule := range rules {
				if rule.FormatType == "markdown" && rule.IsActive {
					// Check if this rule applies to the current item
					fieldValue := ""
					if val, exists := it.Fields[rule.ColumnName]; exists && val != nil {
						fieldValue = fmt.Sprintf("%v", val)
					}

					shouldApply := false
					switch rule.ConditionType {
					case "always":
						shouldApply = true
					case "equals":
						shouldApply = fieldValue == rule.ConditionValue
					case "contains":
						shouldApply = strings.Contains(strings.ToLower(fieldValue), strings.ToLower(rule.ConditionValue))
					case "greater_than":
						if fieldNum, err := strconv.ParseFloat(fieldValue, 64); err == nil {
							if conditionNum, err := strconv.ParseFloat(rule.ConditionValue, 64); err == nil {
								shouldApply = fieldNum > conditionNum
							}
						}
					case "less_than":
						if fieldNum, err := strconv.ParseFloat(fieldValue, 64); err == nil {
							if conditionNum, err := strconv.ParseFloat(rule.ConditionValue, 64); err == nil {
								shouldApply = fieldNum < conditionNum
							}
						}
					}

					if shouldApply {
						// Prepare markdown content
						markdownContent := fieldValue
						if rule.FormatValue != "" {
							markdownContent = fieldValue + "\n\n" + rule.FormatValue
						}

						// Render markdown and add to additional fields
						markdownFieldName := rule.ColumnName + "Markdown"
						renderedMarkdown := renderMarkdown(markdownContent)
						additionalFields[markdownFieldName] = renderedMarkdown
					}
				}
			}
		}

		// Calculate relative time in seconds from now
		var srCreatedRelative int32
		if !it.SrCreated.IsZero() {
			srCreatedRelative = safeInt64ToInt32(int64(time.Since(it.SrCreated).Seconds()))
		}

		var srUpdatedRelative int32
		if !it.SrUpdated.IsZero() {
			srUpdatedRelative = safeInt64ToInt32(int64(time.Since(it.SrUpdated).Seconds()))
		}

		item := &sickrockpb.Item{
			Id:                it.ID,
			SrCreated:         it.SrCreated.Unix(),
			SrCreatedRelative: srCreatedRelative,
			SrUpdated:         it.SrUpdated.Unix(),
			SrUpdatedRelative: srUpdatedRelative,
			AdditionalFields:  additionalFields,
		}

		out = append(out, item)
	}
	return connect.NewResponse(&sickrockpb.ListItemsResponse{Items: out}), nil
}

func (s *SickRockServer) CreateItem(ctx context.Context, req *connect.Request[sickrockpb.CreateItemRequest]) (*connect.Response[sickrockpb.CreateItemResponse], error) {
	table := req.Msg.GetPageId()
	if table == "" {
		table = "items"
	}
	if err := s.requireExportsManageForTable(ctx, table); err != nil {
		return nil, err
	}
	if err := s.denyAuditLogMutation(ctx, table); err != nil {
		return nil, err
	}
	if err := s.requireDataTableAccess(ctx, table, repo.ResourceActionInsert); err != nil {
		return nil, err
	}

	it, err := s.repo.CreateItemInTableWithTimestamp(ctx, table, req.Msg.GetAdditionalFields())
	if err != nil {
		return nil, err
	}
	// Convert dynamic fields to string map for protobuf
	additionalFields := make(map[string]string)
	for key, value := range it.Fields {
		if value != nil {
			// Special handling for time.Time values to ensure consistent formatting
			if timeVal, ok := value.(time.Time); ok {
				// Format as MySQL datetime format (YYYY-MM-DD HH:MM:SS) without timezone
				additionalFields[key] = timeVal.Format("2006-01-02 15:04:05")
			} else {
				additionalFields[key] = fmt.Sprintf("%v", value)
			}
		} else {
			// Include null values as empty strings so they appear in JSON output
			additionalFields[key] = ""
		}
	}

	// Calculate relative time in seconds from now
	var srCreatedRelative int32
	if !it.SrCreated.IsZero() {
		srCreatedRelative = safeInt64ToInt32(int64(time.Since(it.SrCreated).Seconds()))
	}

	var srUpdatedRelative int32
	if !it.SrUpdated.IsZero() {
		srUpdatedRelative = safeInt64ToInt32(int64(time.Since(it.SrUpdated).Seconds()))
	}

	return connect.NewResponse(&sickrockpb.CreateItemResponse{Item: &sickrockpb.Item{
		Id:                it.ID,
		SrCreated:         it.SrCreated.Unix(),
		SrCreatedRelative: srCreatedRelative,
		SrUpdated:         it.SrUpdated.Unix(),
		SrUpdatedRelative: srUpdatedRelative,
		AdditionalFields:  additionalFields,
	}}), nil
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

	// Track this item as recently viewed for the authenticated user
	if userID, uerr := s.getUserIDFromContext(ctx); uerr == nil && userID > 0 {
		if err := s.repo.InsertRecentlyViewed(ctx, userID, table, req.Msg.GetId()); err != nil {
			// Log the error but don't fail the request
			log.WithError(err).WithFields(log.Fields{
				"table": table,
				"id":    req.Msg.GetId(),
			}).Warn("Failed to track recently viewed item")
		}
	}

	// Convert dynamic fields to string map for protobuf
	additionalFields := make(map[string]string)
	for key, value := range it.Fields {
		if value != nil {
			// Special handling for time.Time values to ensure consistent formatting
			if timeVal, ok := value.(time.Time); ok {
				// Format as MySQL datetime format (YYYY-MM-DD HH:MM:SS) without timezone
				additionalFields[key] = timeVal.Format("2006-01-02 15:04:05")
			} else {
				additionalFields[key] = fmt.Sprintf("%v", value)
			}
		} else {
			// Include null values as empty strings so they appear in JSON output
			additionalFields[key] = ""
		}
	}

	log.WithFields(log.Fields{
		"table":  table,
		"itemID": it.ID,
		"fields": it.Fields,
	}).Info("GetItem: Item fields before markdown processing")

	// Process markdown formatting rules
	userID, err := s.getUserIDFromContext(ctx)
	if err == nil {
		// Get conditional formatting rules for this table
		rules, err := s.repo.GetConditionalFormattingRules(ctx, userID, table)
		if err == nil {
			log.WithFields(log.Fields{
				"table":     table,
				"userID":    userID,
				"ruleCount": len(rules),
			}).Info("Retrieved conditional formatting rules")

			// Find markdown rules and render markdown for applicable fields
			for _, rule := range rules {
				log.WithFields(log.Fields{
					"ruleID":         rule.ID,
					"tableName":      rule.TableName,
					"columnName":     rule.ColumnName,
					"formatType":     rule.FormatType,
					"isActive":       rule.IsActive,
					"conditionType":  rule.ConditionType,
					"conditionValue": rule.ConditionValue,
				}).Info("Processing conditional formatting rule")

				if rule.FormatType == "markdown" && rule.IsActive {
					// Check if this rule applies to the current item
					fieldValue := ""
					if val, exists := it.Fields[rule.ColumnName]; exists && val != nil {
						fieldValue = fmt.Sprintf("%v", val)
					}

					log.WithFields(log.Fields{
						"ruleID":         rule.ID,
						"columnName":     rule.ColumnName,
						"fieldValue":     fieldValue,
						"conditionType":  rule.ConditionType,
						"conditionValue": rule.ConditionValue,
					}).Info("Evaluating markdown rule condition")

					shouldApply := false
					switch rule.ConditionType {
					case "always":
						shouldApply = true
					case "equals":
						shouldApply = fieldValue == rule.ConditionValue
					case "contains":
						shouldApply = strings.Contains(strings.ToLower(fieldValue), strings.ToLower(rule.ConditionValue))
					case "greater_than":
						if fieldNum, err := strconv.ParseFloat(fieldValue, 64); err == nil {
							if conditionNum, err := strconv.ParseFloat(rule.ConditionValue, 64); err == nil {
								shouldApply = fieldNum > conditionNum
							}
						}
					case "less_than":
						if fieldNum, err := strconv.ParseFloat(fieldValue, 64); err == nil {
							if conditionNum, err := strconv.ParseFloat(rule.ConditionValue, 64); err == nil {
								shouldApply = fieldNum < conditionNum
							}
						}
					}

					log.WithFields(log.Fields{
						"ruleID":      rule.ID,
						"shouldApply": shouldApply,
					}).Info("Markdown rule evaluation result")

					if shouldApply {
						// Prepare markdown content
						markdownContent := fieldValue
						if rule.FormatValue != "" {
							markdownContent = fieldValue + "\n\n" + rule.FormatValue
						}

						// Render markdown and add to additional fields
						markdownFieldName := rule.ColumnName + "Markdown"
						renderedMarkdown := renderMarkdown(markdownContent)
						additionalFields[markdownFieldName] = renderedMarkdown

						log.WithFields(log.Fields{
							"ruleID":            rule.ID,
							"markdownFieldName": markdownFieldName,
							"markdownContent":   markdownContent,
							"renderedMarkdown":  renderedMarkdown,
						}).Info("Added markdown field to additional fields")
					}
				}
			}
		} else {
			log.WithError(err).WithFields(log.Fields{
				"table":  table,
				"userID": userID,
			}).Error("Failed to get conditional formatting rules")
		}
	} else {
		log.WithError(err).Error("Failed to get user ID from context")
	}

	log.WithFields(log.Fields{
		"table":            table,
		"itemID":           it.ID,
		"additionalFields": additionalFields,
	}).Info("GetItem: Additional fields after markdown processing")

	// Calculate relative time in seconds from now
	var srCreatedRelative int32
	if !it.SrCreated.IsZero() {
		srCreatedRelative = safeInt64ToInt32(int64(time.Since(it.SrCreated).Seconds()))
	}

	var srUpdatedRelative int32
	if !it.SrUpdated.IsZero() {
		srUpdatedRelative = safeInt64ToInt32(int64(time.Since(it.SrUpdated).Seconds()))
	}

	return connect.NewResponse(&sickrockpb.GetItemResponse{Item: &sickrockpb.Item{
		Id:                it.ID,
		SrCreated:         it.SrCreated.Unix(),
		SrCreatedRelative: srCreatedRelative,
		SrUpdated:         it.SrUpdated.Unix(),
		SrUpdatedRelative: srUpdatedRelative,
		AdditionalFields:  additionalFields,
	}}), nil
}

func (s *SickRockServer) EditItem(ctx context.Context, req *connect.Request[sickrockpb.EditItemRequest]) (*connect.Response[sickrockpb.EditItemResponse], error) {
	// Get table name from the request, default to "items" for backward compatibility
	table := req.Msg.GetPageId()
	if table == "" {
		table = "items"
	}
	if err := s.requireExportsManageForTable(ctx, table); err != nil {
		return nil, err
	}
	if err := s.denyAuditLogMutation(ctx, table); err != nil {
		return nil, err
	}
	if err := s.requireDataTableAccess(ctx, table, repo.ResourceActionEdit); err != nil {
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

	// Convert dynamic fields to string map for protobuf
	responseAdditionalFields := make(map[string]string)
	for key, value := range it.Fields {
		if value != nil {
			// Special handling for time.Time values to ensure consistent formatting
			if timeVal, ok := value.(time.Time); ok {
				// Format as MySQL datetime format (YYYY-MM-DD HH:MM:SS) without timezone
				responseAdditionalFields[key] = timeVal.Format("2006-01-02 15:04:05")
			} else {
				responseAdditionalFields[key] = fmt.Sprintf("%v", value)
			}
		} else {
			// Include null values as empty strings so they appear in JSON output
			responseAdditionalFields[key] = ""
		}
	}

	// Calculate relative time in seconds from now
	var srCreatedRelative int32
	if !it.SrCreated.IsZero() {
		srCreatedRelative = safeInt64ToInt32(int64(time.Since(it.SrCreated).Seconds()))
	}

	var srUpdatedRelative int32
	if !it.SrUpdated.IsZero() {
		srUpdatedRelative = safeInt64ToInt32(int64(time.Since(it.SrUpdated).Seconds()))
	}

	return connect.NewResponse(&sickrockpb.EditItemResponse{Item: &sickrockpb.Item{
		Id:                it.ID,
		SrCreated:         it.SrCreated.Unix(),
		SrCreatedRelative: srCreatedRelative,
		SrUpdated:         it.SrUpdated.Unix(),
		SrUpdatedRelative: srUpdatedRelative,
		AdditionalFields:  responseAdditionalFields,
	}}), nil
}

func (s *SickRockServer) DeleteItem(ctx context.Context, req *connect.Request[sickrockpb.DeleteItemRequest]) (*connect.Response[sickrockpb.DeleteItemResponse], error) {
	table := req.Msg.GetPageId()
	if err := s.requireExportsManageForTable(ctx, table); err != nil {
		return nil, err
	}
	if err := s.denyAuditLogMutation(ctx, table); err != nil {
		return nil, err
	}
	if err := s.requireDataTableAccess(ctx, table, repo.ResourceActionDelete); err != nil {
		return nil, err
	}

	ok, err := s.repo.DeleteItemInTable(ctx, table, req.Msg.GetId())
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&sickrockpb.DeleteItemResponse{Deleted: ok}), nil
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

	cols, err := s.repo.ListColumns(ctx, tc)
	if err != nil {
		log.Errorf("list columns: %v, table: %s", err, tcName)
	} else {
		log.WithField("columnCount", len(cols)).Debugf("list columns for table %s", tcName)
	}
	cols = s.repo.ApplyColumnSemantics(ctx, tcName, cols)
	displayByColumn, displayErr := s.repo.GetColumnDisplaySettings(ctx, tcName)
	if displayErr != nil {
		log.WithError(displayErr).Warn("Failed to load column display settings")
		displayByColumn = map[string]repo.ColumnDisplaySetting{}
	}
	fields := make([]*sickrockpb.Field, 0, len(cols))
	for _, c := range cols {
		pbField := &sickrockpb.Field{
			Name:                      c.Name,
			Type:                      c.Type,
			Required:                  c.Required,
			DefaultToCurrentTimestamp: false, // This information is not stored in database metadata
		}
		if d, ok := repo.LookupColumnDisplay(displayByColumn, c.Name); ok {
			if d.DisplayLabel.Valid {
				pbField.DisplayLabel = strings.TrimSpace(d.DisplayLabel.String)
			}
			if d.ValuePrefix.Valid {
				pbField.ValuePrefix = d.ValuePrefix.String
			}
			if d.ValueSuffix.Valid {
				pbField.ValueSuffix = d.ValueSuffix.String
			}
		}
		fields = append(fields, pbField)
	}

	createButtonText := "Insert Row"
	if tc.CreateButtonText.Valid {
		createButtonText = tc.CreateButtonText.String
	}
	createDelegate := ""
	if tc.CreateDelegate.Valid {
		createDelegate = strings.TrimSpace(tc.CreateDelegate.String)
	}

	// Get foreign keys for this table
	foreignKeys, err := s.repo.GetForeignKeysForTableConfig(ctx, tc)
	var pbForeignKeys []*sickrockpb.ForeignKey
	if err != nil {
		log.Errorf("Failed to get foreign keys: %v", err)
		pbForeignKeys = []*sickrockpb.ForeignKey{}
	} else {
		log.WithFields(log.Fields{
			"tcName": tcName,
			"numFks": len(foreignKeys),
		}).Debug("Retrieved foreign keys")

		lookupPairs := make([]repo.DbTablePair, 0, len(foreignKeys)*2)
		for _, fk := range foreignKeys {
			tableDb := fk.TableSchema
			if tableDb == "" {
				tableDb = tc.Db.String
			}
			referencedDb := fk.ReferencedSchema
			if referencedDb == "" {
				referencedDb = tc.Db.String
			}
			lookupPairs = append(lookupPairs,
				repo.DbTablePair{Db: tableDb, Table: fk.TableName},
				repo.DbTablePair{Db: referencedDb, Table: fk.ReferencedTable},
			)
		}
		tcNameByDbTable, lookupErr := s.repo.LookupTableConfigNames(ctx, lookupPairs)
		if lookupErr != nil {
			log.WithError(lookupErr).Warn("Failed to batch lookup table configuration names for foreign keys")
			tcNameByDbTable = map[string]string{}
		}

		// Convert repository foreign keys to protobuf foreign keys
		pbForeignKeys = make([]*sickrockpb.ForeignKey, 0, len(foreignKeys))
		for _, fk := range foreignKeys {
			tableDb := fk.TableSchema
			if tableDb == "" {
				tableDb = tc.Db.String
			}
			referencedDb := fk.ReferencedSchema
			if referencedDb == "" {
				referencedDb = tc.Db.String
			}

			tableTcName := tcNameByDbTable[repo.DbTableLookupKey(tableDb, fk.TableName)]
			referencedTcName := tcNameByDbTable[repo.DbTableLookupKey(referencedDb, fk.ReferencedTable)]

			pbForeignKeys = append(pbForeignKeys, &sickrockpb.ForeignKey{
				RelationId:            int32(fk.RelationID),
				ConstraintName:        fk.ConstraintName,
				TableName:             fk.TableName,
				ColumnName:            fk.ColumnName,
				ReferencedTable:       fk.ReferencedTable,
				ReferencedColumn:      fk.ReferencedColumn,
				OnDeleteAction:        fk.OnDeleteAction,
				OnUpdateAction:        fk.OnUpdateAction,
				TableTcName:           tableTcName,
				ReferencedTableTcName: referencedTcName,
				DisplayName:           fk.DisplayName,
				PullThroughFields:     pullThroughFieldsToProto(fk.PullThroughFields),
				AbsorbIntoItem:        fk.AbsorbIntoItem,
				AbsorbType:            fk.AbsorbType,
			})
		}
	}

	// Get view type from default view if it exists, otherwise default to "table"
	viewType := "table"
	views, err := s.repo.GetTableViews(ctx, tcName)
	if err == nil && len(views) > 0 {
		userDefaultID := s.userDefaultTableView(ctx, tcName)
		defaultView := views[0] // GetTableViews returns views ordered, first is usually default
		for _, v := range views {
			if v.IsDefault {
				defaultView = v
				break
			}
		}
		for _, v := range views {
			if userDefaultID > 0 && v.ID == userDefaultID {
				defaultView = v
				break
			}
		}
		if defaultView.ViewType != "" {
			viewType = defaultView.ViewType
		}
	}

	colNames := make([]string, len(cols))
	for i, c := range cols {
		colNames[i] = c.Name
	}
	displayTitle := tc.Name
	if tc.Title != "" {
		displayTitle = tc.Title
	}

	layoutBlocks, layoutErr := s.repo.GetItemPageLayoutBlocks(ctx, tcName)
	if layoutErr != nil {
		log.WithError(layoutErr).Warn("Failed to load item page layout")
		layoutBlocks = nil
	}
	pbLayoutBlocks := make([]*sickrockpb.PageLayoutBlock, 0, len(layoutBlocks))
	for _, b := range layoutBlocks {
		pbLayoutBlocks = append(pbLayoutBlocks, &sickrockpb.PageLayoutBlock{
			Id:         int32(b.ID),
			Ordinal:    int32(b.Ordinal),
			BlockType:  b.BlockType,
			ConfigJson: b.ConfigJSON,
		})
	}

	fieldGroups, fgErr := s.repo.GetItemFieldGroups(ctx, tcName)
	if fgErr != nil {
		log.WithError(fgErr).Warn("Failed to load item field groups")
		fieldGroups = nil
	}
	pbFieldGroups := make([]*sickrockpb.ItemFieldGroup, 0, len(fieldGroups))
	for i, g := range fieldGroups {
		pbFieldGroups = append(pbFieldGroups, &sickrockpb.ItemFieldGroup{
			Id:          int32(i + 1),
			Ordinal:     int32(g.Ordinal),
			Title:       g.Title,
			ColumnNames: append([]string(nil), g.ColumnNames...),
		})
	}

	pbColumnDisplay := make([]*sickrockpb.ColumnDisplaySettingInput, 0, len(displayByColumn))
	for columnName, d := range displayByColumn {
		row := &sickrockpb.ColumnDisplaySettingInput{ColumnName: columnName}
		if d.DisplayLabel.Valid {
			row.DisplayLabel = strings.TrimSpace(d.DisplayLabel.String)
		}
		if d.ValuePrefix.Valid {
			row.ValuePrefix = d.ValuePrefix.String
		}
		if d.ValueSuffix.Valid {
			row.ValueSuffix = d.ValueSuffix.String
		}
		pbColumnDisplay = append(pbColumnDisplay, row)
	}

	return connect.NewResponse(&sickrockpb.GetTableStructureResponse{
		Fields:            fields,
		CreateButtonText:  createButtonText,
		View:              viewType,
		ForeignKeys:       pbForeignKeys,
		PrimaryKeyColumn:  tc.PrimaryKeyColumnName(),
		DefaultSortColumn: tc.SortColumnName(colNames),
		CreateDelegate:    createDelegate,
		Title:             displayTitle,
		RowName:           tc.DisplayRowName(),
		RowNamePlural:     tc.DisplayRowNamePlural(),
		ItemPageLayout: &sickrockpb.ItemPageLayout{
			Blocks: pbLayoutBlocks,
		},
		ItemFieldGroups: &sickrockpb.ItemFieldGroups{
			Groups: pbFieldGroups,
		},
		ColumnDisplaySettings: pbColumnDisplay,
	}), nil
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
		return connect.NewResponse(&sickrockpb.SetItemFieldGroupsResponse{
			Success: false,
			Message: err.Error(),
		}), nil
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
	tc, err := s.repo.GetTableConfiguration(ctx, tcName)
	if err != nil {
		return nil, err
	}
	cols, err := s.repo.ListColumns(ctx, tc)
	if err != nil {
		return connect.NewResponse(&sickrockpb.SetColumnDisplaySettingsResponse{
			Success: false,
			Message: err.Error(),
		}), nil
	}
	allowed := make(map[string]struct{}, len(cols))
	for _, c := range cols {
		allowed[c.Name] = struct{}{}
	}
	syntheticKeys, synErr := s.repo.ListRelationSyntheticColumnKeys(ctx, tcName)
	if synErr != nil {
		log.WithError(synErr).Warn("Failed to list relation synthetic columns for display settings")
	} else {
		for _, key := range syntheticKeys {
			allowed[key] = struct{}{}
		}
	}
	inputs := make([]repo.ColumnDisplayInput, 0, len(req.Msg.GetSettings()))
	for _, s := range req.Msg.GetSettings() {
		inputs = append(inputs, repo.ColumnDisplayInput{
			ColumnName:   s.GetColumnName(),
			DisplayLabel: s.GetDisplayLabel(),
			ValuePrefix:  s.GetValuePrefix(),
			ValueSuffix:  s.GetValueSuffix(),
		})
	}
	if err := s.repo.SetColumnDisplaySettings(ctx, tcName, inputs, allowed); err != nil {
		return connect.NewResponse(&sickrockpb.SetColumnDisplaySettingsResponse{
			Success: false,
			Message: err.Error(),
		}), nil
	}
	return connect.NewResponse(&sickrockpb.SetColumnDisplaySettingsResponse{
		Success: true,
		Message: "Column display settings saved",
	}), nil
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
		return connect.NewResponse(&sickrockpb.SetTableRowLabelsResponse{
			Success: false,
			Message: err.Error(),
		}), nil
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
	fieldType := f.GetType()
	err = s.repo.AddColumn(ctx, tc.Db.String, tc.Table.String, repo.FieldSpec{
		Name:                      f.GetName(),
		Type:                      fieldType,
		Required:                  f.GetRequired(),
		DefaultToCurrentTimestamp: f.GetDefaultToCurrentTimestamp(),
	})
	if err != nil {
		return nil, err
	}
	semantic := ""
	switch {
	case fieldType == repo.SemanticTypeUserRef:
		semantic = repo.SemanticTypeUserRef
	case repo.IsDatetimeTypeName(fieldType):
		semantic = repo.SemanticTypeDatetime
	}
	if semantic != "" {
		if err := s.repo.SetColumnSemantic(ctx, req.Msg.GetPageId(), f.GetName(), semantic); err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("save column semantic: %w", err))
		}
	}
	return s.GetTableStructure(ctx, &connect.Request[sickrockpb.GetTableStructureRequest]{Msg: &sickrockpb.GetTableStructureRequest{PageId: req.Msg.GetPageId()}})
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

	// Convert protobuf columns to repository columns
	var columns []repo.TableViewColumn
	for _, pbCol := range req.Msg.GetColumns() {
		columns = append(columns, repo.TableViewColumn{
			ColumnName:  pbCol.GetColumnName(),
			IsVisible:   pbCol.GetIsVisible(),
			ColumnOrder: int(pbCol.GetColumnOrder()),
			SortOrder:   pbCol.GetSortOrder(),
		})
	}

	viewType := req.Msg.GetViewType()
	if viewType == "" {
		viewType = "table" // Default to "table"
	}
	layoutJSON, err := validateViewLayoutJSON(req.Msg.GetLayoutJson())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
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
		ViewId:  int32(viewID),
	}), nil
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

	// Convert protobuf columns to repository columns
	var columns []repo.TableViewColumn
	for _, pbCol := range req.Msg.GetColumns() {
		columns = append(columns, repo.TableViewColumn{
			ColumnName:  pbCol.GetColumnName(),
			IsVisible:   pbCol.GetIsVisible(),
			ColumnOrder: int(pbCol.GetColumnOrder()),
			SortOrder:   pbCol.GetSortOrder(),
		})
	}

	viewType := req.Msg.GetViewType()
	if viewType == "" {
		viewType = "table" // Default to "table"
	}

	layoutJSON, err := validateViewLayoutJSON(req.Msg.GetLayoutJson())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
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
				ColumnOrder: int32(col.ColumnOrder),
				SortOrder:   col.SortOrder,
			})
		}

		viewType := view.ViewType
		if viewType == "" {
			viewType = "table" // Default to "table"
		}

		pbViews = append(pbViews, &sickrockpb.TableView{
			Id:         int32(view.ID),
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
		UserDefaultViewId: int32(s.userDefaultTableView(ctx, tableName)),
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
	tableName := strings.TrimSpace(req.Msg.GetTableName())
	if tableName == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("table_name is required"))
	}
	userID, err := s.getUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.requireDataTableAccess(ctx, tableName, repo.ResourceActionView); err != nil {
		return nil, err
	}
	if err := s.repo.SetUserDefaultTableView(ctx, userID, tableName, int(req.Msg.GetViewId())); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	return connect.NewResponse(&sickrockpb.SetUserDefaultTableViewResponse{}), nil
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
		return connect.NewResponse(&sickrockpb.DeleteTableViewResponse{
			Success: false,
			Message: err.Error(),
		}), nil
	}
	if err := s.requireDataTableAccess(ctx, viewTable, repo.ResourceActionEdit); err != nil {
		return nil, err
	}

	err = s.repo.DeleteTableView(ctx, viewID)
	if err != nil {
		log.Errorf("Failed to delete table view: %v", err)
		return connect.NewResponse(&sickrockpb.DeleteTableViewResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to delete view: %v", err),
		}), nil
	}

	return connect.NewResponse(&sickrockpb.DeleteTableViewResponse{
		Success: true,
		Message: "Table view deleted successfully",
	}), nil
}

func (s *SickRockServer) CreateForeignKey(ctx context.Context, req *connect.Request[sickrockpb.CreateForeignKeyRequest]) (*connect.Response[sickrockpb.CreateForeignKeyResponse], error) {
	tableName := req.Msg.GetTableName()
	columnName := req.Msg.GetColumnName()
	referencedTable := req.Msg.GetReferencedTable()
	referencedColumn := req.Msg.GetReferencedColumn()
	onDeleteAction := req.Msg.GetOnDeleteAction()
	onUpdateAction := req.Msg.GetOnUpdateAction()

	if tableName == "" || columnName == "" || referencedTable == "" || referencedColumn == "" {
		return connect.NewResponse(&sickrockpb.CreateForeignKeyResponse{
			Success: false,
			Message: "Table name, column name, referenced table, and referenced column are required",
		}), nil
	}

	// Validate actions
	validActions := []string{"CASCADE", "SET NULL", "RESTRICT", "NO ACTION"}
	if !contains(validActions, onDeleteAction) || !contains(validActions, onUpdateAction) {
		return connect.NewResponse(&sickrockpb.CreateForeignKeyResponse{
			Success: false,
			Message: "Invalid action. Must be one of: CASCADE, SET NULL, RESTRICT, NO ACTION",
		}), nil
	}

	err := s.repo.CreateForeignKey(ctx, tableName, columnName, referencedTable, referencedColumn, onDeleteAction, onUpdateAction)
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
			RelationId:       int32(fk.RelationID),
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
		return connect.NewResponse(&sickrockpb.UpdateTableRelationResponse{
			Success: false,
			Message: "Relation not found",
		}), nil
	}
	if s.requireTableAccess(ctx, rel.SourceTableKey, repo.ResourceActionEdit) != nil &&
		s.requireTableAccess(ctx, rel.ReferencedTableKey, repo.ResourceActionEdit) != nil {
		return nil, connect.NewError(connect.CodePermissionDenied, fmt.Errorf("table edit access required"))
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
		return connect.NewResponse(&sickrockpb.SetItemPageLayoutResponse{
			Success: false,
			Message: err.Error(),
		}), nil
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
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
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

	if tableName == "" || columnName == "" {
		return connect.NewResponse(&sickrockpb.DropColumnResponse{
			Success: false,
			Message: "Table name and column name are required",
		}), nil
	}

	// Prevent dropping system columns
	if columnName == "id" {
		return connect.NewResponse(&sickrockpb.DropColumnResponse{
			Success: false,
			Message: "Cannot drop system columns (id)",
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

func (s *SickRockServer) ChangeColumnName(ctx context.Context, req *connect.Request[sickrockpb.ChangeColumnNameRequest]) (*connect.Response[sickrockpb.ChangeColumnNameResponse], error) {
	tableName := req.Msg.GetTableName()
	oldColumnName := req.Msg.GetOldColumnName()
	newColumnName := req.Msg.GetNewColumnName()

	if tableName == "" || oldColumnName == "" || newColumnName == "" {
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

func (s *SickRockServer) GetMostRecentlyViewed(ctx context.Context, req *connect.Request[sickrockpb.GetMostRecentlyViewedRequest]) (*connect.Response[sickrockpb.GetMostRecentlyViewedResponse], error) {
	userID, err := s.getUserIDFromContext(ctx)
	if err != nil || userID <= 0 {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	limit := int(req.Msg.GetLimit())
	if limit <= 0 {
		limit = 10 // Default limit
	}

	fetchLimit := limit * 5
	if fetchLimit < limit {
		fetchLimit = limit
	}

	items, err := s.repo.GetMostRecentlyViewed(ctx, userID, fetchLimit)
	if err != nil {
		return nil, err
	}

	pbItems := make([]*sickrockpb.RecentlyViewedItem, 0, limit)
	for _, item := range items {
		if s.requireTableAccess(ctx, item.Name, repo.ResourceActionView) != nil {
			continue
		}
		pbItems = append(pbItems, &sickrockpb.RecentlyViewedItem{
			Name:          item.Name,
			TableId:       item.TableID,
			Icon:          item.Icon,
			UpdatedAtUnix: item.UpdatedAtUnix,
			ItemName:      item.ItemName,
			TableTitle:    item.TableTitle,
		})
		if len(pbItems) >= limit {
			break
		}
	}

	return connect.NewResponse(&sickrockpb.GetMostRecentlyViewedResponse{
		Items: pbItems,
	}), nil
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
	name := strings.TrimSpace(req.Msg.GetName())
	if name == "" {
		return connect.NewResponse(&sickrockpb.CreateDashboardResponse{
			Success: false,
			Message: "Dashboard name is required",
		}), nil
	}

	createNavigation := true
	if req.Msg.CreateNavigationEntry != nil {
		createNavigation = req.Msg.GetCreateNavigationEntry()
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
		DashboardId: int32(dashboardID),
	}), nil
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

		runningEnv := make(map[string]interface{})
		runningEnv["round1"] = func(in float64) (float64, error) {
			return math.Round(in*10) / 10, nil
		}

		pbComps := make([]*sickrockpb.DashboardComponent, 0, len(comps))
		// Preload rules for each component without returning them to the client
		for _, c := range comps {
			data, err := s.getDashboardComponentData(ctx, c, &runningEnv)

			runningEnv[c.Name] = data

			var pbComp *sickrockpb.DashboardComponent
			if err != nil {
				// If there's an error getting component data, create a component with error info
				pbComp = &sickrockpb.DashboardComponent{
					Id:         int32(c.ID),
					Name:       c.Name,
					DataString: "",
					DataNumber: 0,
					Error:      err.Error(),
					Suffix:     "",
				}
				log.WithError(err).WithField("component", c.ID).Warn("Failed to load dashboard component data")
			} else {
				rules, err := s.repo.GetDashboardComponentRules(ctx, &c.ID)

				if err != nil {
					log.WithError(err).WithField("component", c.ID).Warn("Failed to load dashboard component rules")
				}

				pbComp = &sickrockpb.DashboardComponent{
					Id:         int32(c.ID),
					Name:       c.Name,
					DataString: fmt.Sprintf("%v", data),
					Suffix:     "",
				}

				s.applyRules(pbComp, rules)
			}

			pbComps = append(pbComps, pbComp)
		}
		out = append(out, &sickrockpb.Dashboard{Id: int32(d.ID), Name: d.Name, Components: pbComps})
	}
	return connect.NewResponse(&sickrockpb.GetDashboardsResponse{Dashboards: out}), nil
}

func (s *SickRockServer) getDashboardComponentData(ctx context.Context, comp repo.DashboardComponent, runningEnv *map[string]interface{}) (any, error) {
	formula := strings.TrimSpace(comp.Formula.String)

	if formula == "" {
		return "", nil
	}

	// Handle special case for "latest" query type
	if formula == "latest" {
		if !comp.TcID.Valid {
			return "", fmt.Errorf("tc_id is not valid for component %d", comp.ID)
		}
		item, err := s.repo.GetLastItem(ctx, int(comp.TcID.Int32))

		if err != nil {
			return "", err
		}

		return item.Fields[comp.ColumnName.String], nil
	}

	// Parse expression using expr-lang/expr
	if formula != "" {
		// Create environment with available data
		env := *runningEnv
		env["latest"] = func() (map[string]interface{}, error) {
			if !comp.TcID.Valid {
				return nil, fmt.Errorf("tc_id is not valid for component %d", comp.ID)
			}
			item, err := s.repo.GetLastItem(ctx, int(comp.TcID.Int32))
			if err != nil {
				return nil, err
			}
			return item.Fields, nil
		}

		// Compile and evaluate the expression
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

	return "", fmt.Errorf("no formula specified for component %d", comp.ID)
}

func (s *SickRockServer) applyRules(comp *sickrockpb.DashboardComponent, rules []repo.DashboardComponentRule) {
	for _, r := range rules {
		log.Infof("Applying rule %+v to component %v", r, comp.Name)

		switch r.Operation {
		case "suffix":
			comp.Suffix = r.Operand
			break
		default:
			log.Warnf("Unknown operation %s", r.Operation)
		}
	}
}

func (s *SickRockServer) GetDashboardComponentRules(ctx context.Context, req *connect.Request[sickrockpb.GetDashboardComponentRulesRequest]) (*connect.Response[sickrockpb.GetDashboardComponentRulesResponse], error) {
	var compPtr *int
	if req.Msg != nil && req.Msg.GetComponent() != 0 {
		v := int(req.Msg.GetComponent())
		compPtr = &v
	}
	rules, err := s.repo.GetDashboardComponentRules(ctx, compPtr)
	if err != nil {
		return nil, err
	}
	out := make([]*sickrockpb.DashboardComponentRule, 0, len(rules))
	for _, rle := range rules {
		out = append(out, &sickrockpb.DashboardComponentRule{
			Id:        int32(rle.ID),
			Component: int32(rle.Component),
			Ordinal:   int32(rle.Ordinal),
			Operation: rle.Operation,
			Operand:   rle.Operand,
		})
	}
	return connect.NewResponse(&sickrockpb.GetDashboardComponentRulesResponse{Rules: out}), nil
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
		Id:        int32(rule.ID),
		Component: int32(rule.Component),
		Ordinal:   int32(rule.Ordinal),
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

	pbBookmarks := make([]*sickrockpb.UserBookmark, 0, len(bookmarks))
	for _, bookmark := range bookmarks {
		var navItem *sickrockpb.NavigationItem
		if bookmark.NavigationItem != nil {
			navItem = navigationItemRepoToProto(*bookmark.NavigationItem, bookmark.NavigationItem.TableView.String)
		}

		pbBookmarks = append(pbBookmarks, &sickrockpb.UserBookmark{
			Id:               int32(bookmark.ID),
			UserId:           int32(bookmark.UserID),
			NavigationItemId: int32(bookmark.NavigationItemID),
			NavigationItem:   navItem,
			Title:            bookmarkLinkTitle(bookmark),
		})
	}

	return connect.NewResponse(&sickrockpb.GetUserBookmarksResponse{Bookmarks: pbBookmarks}), nil
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

	var navItem *sickrockpb.NavigationItem
	if bookmark.NavigationItem != nil {
		navItem = navigationItemRepoToProto(*bookmark.NavigationItem, bookmark.NavigationItem.TableView.String)
	}

	pbBookmark := &sickrockpb.UserBookmark{
		Id:               int32(bookmark.ID),
		UserId:           int32(bookmark.UserID),
		NavigationItemId: int32(bookmark.NavigationItemID),
		NavigationItem:   navItem,
	}

	return connect.NewResponse(&sickrockpb.CreateUserBookmarkResponse{Bookmark: pbBookmark}), nil
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
	tcName := strings.TrimSpace(req.Msg.GetTcName())
	itemID := strings.TrimSpace(req.Msg.GetItemId())
	if tcName == "" || itemID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("tc_name and item_id are required"))
	}
	if err := s.repo.SetTickListCompletion(ctx, tcName, itemID, req.Msg.GetCompleted()); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&sickrockpb.SetTickListCompletionResponse{}), nil
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

	readOnly := req.Msg.GetReadOnly()

	apiKey, err := armpassword.GenerateAPIKey("sk_")
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to generate API key: %w", err))
	}

	keyID, err := s.auth.Store.CreateAPIKey(ctx, userID, name, apiKey, readOnly)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to create API key: %w", err))
	}

	return connect.NewResponse(&sickrockpb.CreateAPIKeyResponse{
		Success:  true,
		Message:  "API key created successfully",
		ApiKey:   apiKey,
		ApiKeyId: int32(keyID),
	}), nil
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
			Id:         int32(apiKey.ID),
			UserId:     int32(apiKey.UserAccountID),
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
			Id:             int32(rule.ID),
			TableName:      rule.TableName,
			ColumnName:     rule.ColumnName,
			ConditionType:  rule.ConditionType,
			ConditionValue: rule.ConditionValue,
			FormatType:     rule.FormatType,
			FormatValue:    rule.FormatValue,
			Priority:       int32(rule.Priority),
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
		RuleId:  int32(ruleID),
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
