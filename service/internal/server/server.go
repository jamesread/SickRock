package server

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/expr-lang/expr"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"

	sickrockpb "github.com/jamesread/SickRock/gen/proto"
	"github.com/jamesread/SickRock/internal/auth"
	"github.com/jamesread/SickRock/internal/buildinfo"
	repo "github.com/jamesread/SickRock/internal/repo"
	log "github.com/sirupsen/logrus"
)

type SickRockServer struct {
	repo *repo.Repository
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

func NewSickRockServer(r *repo.Repository) *SickRockServer {
	return &SickRockServer{repo: r}
}

// getUserIDFromContext extracts the user ID from the context
func (s *SickRockServer) getUserIDFromContext(ctx context.Context) (int, error) {
	authService := auth.NewAuthService(s.repo)
	username, err := authService.GetUserFromContext(ctx)
	if err != nil {
		return 0, err
	}

	user, err := s.repo.GetUserByUsername(ctx, username)
	if err != nil {
		return 0, err
	}
	if user == nil {
		return 0, fmt.Errorf("user not found")
	}

	return user.ID, nil
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

// lookupTableConfigName looks up the table configuration name for a given database and table
func (s *SickRockServer) lookupTableConfigName(ctx context.Context, database, table string) string {
	// Query table_configurations for a match
	var tcName string
	query := "SELECT name FROM table_configurations WHERE `db` = ? AND `table` = ? LIMIT 1"
	err := s.repo.DB().GetContext(ctx, &tcName, query, database, table)
	if err != nil {
		log.WithError(err).WithFields(log.Fields{
			"database": database,
			"table":    table,
		}).Debug("No table configuration found")
		return ""
	}
	log.WithFields(log.Fields{
		"database": database,
		"table":    table,
		"tcName":   tcName,
	}).Debug("Found table configuration")
	return tcName
}

func (s *SickRockServer) Init(ctx context.Context, req *connect.Request[sickrockpb.InitRequest]) (*connect.Response[sickrockpb.InitResponse], error) {
	dbName := strings.TrimSpace(os.Getenv("DB_NAME"))

	var currentUsername string
	if claims, ok := ctx.Value("user").(*auth.Claims); ok && claims != nil {
		currentUsername = claims.Username
	}

	res := connect.NewResponse(&sickrockpb.InitResponse{
		Version:         buildinfo.Version,
		Commit:          buildinfo.Commit,
		Date:            buildinfo.Date,
		DbName:          dbName,
		CurrentUsername: currentUsername,
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

	// Create the table configuration
	err := s.repo.CreateTableConfiguration(ctx, name, database, table)
	if err != nil {
		log.Errorf("Failed to create table configuration: %v", err)
		return connect.NewResponse(&sickrockpb.CreateTableConfigurationResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to create table configuration: %v", err),
		}), nil
	}

	return connect.NewResponse(&sickrockpb.CreateTableConfigurationResponse{
		Success: true,
		Message: "Table configuration created successfully",
	}), nil
}

func (s *SickRockServer) GetDatabaseTables(ctx context.Context, req *connect.Request[sickrockpb.GetDatabaseTablesRequest]) (*connect.Response[sickrockpb.GetDatabaseTablesResponse], error) {
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

func (s *SickRockServer) GetNavigation(ctx context.Context, req *connect.Request[sickrockpb.GetNavigationRequest]) (*connect.Response[sickrockpb.GetNavigationResponse], error) {
	items, err := s.repo.GetNavigation(ctx)
	if err != nil {
		return nil, err
	}

	navigationItems := make([]*sickrockpb.NavigationItem, 0, len(items))
	for _, item := range items {
		navigationItems = append(navigationItems, &sickrockpb.NavigationItem{
			Id:      int32(item.ID),
			Ordinal: int32(item.Ordinal),
			TableConfiguration: func() int32 {
				if item.TableConfiguration.Valid {
					return int32(item.TableConfiguration.Int64)
				}
				return 0
			}(),
			TableName:  item.TableName.String,
			TableTitle: item.TableTitle.String,
			Icon:       item.Icon.String,
			TableView:  "", // View type is now stored on views, not table configurations
			DashboardId: func() int32 {
				if item.DashboardID.Valid {
					return int32(item.DashboardID.Int64)
				}
				return 0
			}(),
			DashboardName: item.DashboardName.String,
			Title:         item.Navigation.String,
			WorkflowId: func() int32 {
				if item.WorkflowID.Valid {
					return int32(item.WorkflowID.Int64)
				}
				return 0
			}(),
			WorkflowName: item.WorkflowName.String,
		})
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
				var navItem *sickrockpb.NavigationItem
				if bookmark.NavigationItem != nil {
					navItem = &sickrockpb.NavigationItem{
						Id:      int32(bookmark.NavigationItem.ID),
						Ordinal: int32(bookmark.NavigationItem.Ordinal),
						TableConfiguration: func() int32 {
							if bookmark.NavigationItem.TableConfiguration.Valid {
								return int32(bookmark.NavigationItem.TableConfiguration.Int64)
							}
							return 0
						}(),
						TableName:  bookmark.NavigationItem.TableName.String,
						TableTitle: bookmark.NavigationItem.TableTitle.String,
						Icon:       bookmark.NavigationItem.Icon.String,
						TableView:  bookmark.NavigationItem.TableView.String,
						DashboardId: func() int32 {
							if bookmark.NavigationItem.DashboardID.Valid {
								return int32(bookmark.NavigationItem.DashboardID.Int64)
							}
							return 0
						}(),
						DashboardName: bookmark.NavigationItem.DashboardName.String,
						Title:         bookmark.NavigationItem.Navigation.String,
						WorkflowId: func() int32 {
							if bookmark.NavigationItem.WorkflowID.Valid {
								return int32(bookmark.NavigationItem.WorkflowID.Int64)
							}
							return 0
						}(),
						WorkflowName: bookmark.NavigationItem.WorkflowName.String,
					}
				}

				bookmarks = append(bookmarks, &sickrockpb.UserBookmark{
					Id:               int32(bookmark.ID),
					UserId:           int32(bookmark.UserID),
					NavigationItemId: int32(bookmark.NavigationItemID),
					NavigationItem:   navItem,
					Title:            bookmark.Title.String,
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

	// Create a map of workflow ID to navigation items
	workflowItemsMap := make(map[int32][]*sickrockpb.NavigationItem)
	for _, item := range navigationItems {
		if item.WorkflowId > 0 {
			workflowItemsMap[item.WorkflowId] = append(workflowItemsMap[item.WorkflowId], item)
		}
	}

	// Convert workflows to protobuf format
	workflowProtos := make([]*sickrockpb.Workflow, 0, len(workflows))
	for _, workflow := range workflows {
		workflowProto := &sickrockpb.Workflow{
			Id:      int32(workflow.ID),
			Name:    workflow.Name,
			Ordinal: int32(workflow.Ordinal),
			Icon:    workflow.Icon.String,
			Items:   workflowItemsMap[int32(workflow.ID)],
		}
		workflowProtos = append(workflowProtos, workflowProto)
	}

	res := connect.NewResponse(&sickrockpb.GetNavigationResponse{
		Items:     navigationItems,
		Bookmarks: bookmarks,
		Workflows: workflowProtos,
	})
	return res, nil
}

func (s *SickRockServer) ListItems(ctx context.Context, req *connect.Request[sickrockpb.ListItemsRequest]) (*connect.Response[sickrockpb.ListItemsResponse], error) {
	// Use page_id as table name for this simple mapping
	table := req.Msg.GetTcName()

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

	tc, err := s.repo.GetTableConfiguration(ctx, table)
	if err != nil {
		return nil, err
	}

	it, err := s.repo.GetItemInTable(ctx, tc, req.Msg.GetId())
	if err != nil {
		return nil, err
	}

	// Track this item as recently viewed
	if err := s.repo.InsertRecentlyViewed(ctx, table, req.Msg.GetId()); err != nil {
		// Log the error but don't fail the request
		log.WithError(err).WithFields(log.Fields{
			"table": table,
			"id":    req.Msg.GetId(),
		}).Warn("Failed to track recently viewed item")
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

	ok, err := s.repo.DeleteItemInTable(ctx, table, req.Msg.GetId())
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&sickrockpb.DeleteItemResponse{Deleted: ok}), nil
}

func (s *SickRockServer) GetTableStructure(ctx context.Context, req *connect.Request[sickrockpb.GetTableStructureRequest]) (*connect.Response[sickrockpb.GetTableStructureResponse], error) {
	tcName := req.Msg.GetPageId()

	tc, err := s.repo.GetTableConfiguration(ctx, tcName)
	if err != nil {
		return nil, err
	}

	cols, err := s.repo.ListColumns(ctx, tc)
	if err != nil {
		log.Errorf("list columns: %v, table: %s", err, tcName)
	} else {
		log.Infof("list columns: %v, table: %s", cols, tcName)
	}
	fields := make([]*sickrockpb.Field, 0, len(cols))
	for _, c := range cols {
		fields = append(fields, &sickrockpb.Field{
			Name:                      c.Name,
			Type:                      c.Type,
			Required:                  c.Required,
			DefaultToCurrentTimestamp: false, // This information is not stored in database metadata
		})
	}

	log.Infof("GetTableStructureResponse: %+v", tc)

	createButtonText := "Insert Row"
	if tc.CreateButtonText.Valid {
		createButtonText = tc.CreateButtonText.String
	}

	// Get foreign keys for this table
	foreignKeys, err := s.repo.GetForeignKeys(ctx, tcName)
	var pbForeignKeys []*sickrockpb.ForeignKey
	if err != nil {
		log.Errorf("Failed to get foreign keys: %v", err)
		pbForeignKeys = []*sickrockpb.ForeignKey{}
	} else {
		log.WithFields(log.Fields{
			"tcName": tcName,
			"numFks": len(foreignKeys),
			"fks":    foreignKeys,
		}).Debug("Retrieved foreign keys")

		// Convert repository foreign keys to protobuf foreign keys
		pbForeignKeys = make([]*sickrockpb.ForeignKey, 0, len(foreignKeys))
		for _, fk := range foreignKeys {
			// Look up table configuration names for both tables
			// Use TableSchema for the table, ReferencedSchema for the referenced table
			tableDb := fk.TableSchema
			if tableDb == "" {
				tableDb = tc.Db.String
			}
			referencedDb := fk.ReferencedSchema
			if referencedDb == "" {
				referencedDb = tc.Db.String
			}

			log.WithFields(log.Fields{
				"tableSchema":      fk.TableSchema,
				"tableName":        fk.TableName,
				"referencedSchema": fk.ReferencedSchema,
				"referencedTable":  fk.ReferencedTable,
				"tableDb":          tableDb,
				"referencedDb":     referencedDb,
				"currentTcDb":      tc.Db.String,
			}).Debug("Looking up table config names for foreign key")

			tableTcName := s.lookupTableConfigName(ctx, tableDb, fk.TableName)
			referencedTcName := s.lookupTableConfigName(ctx, referencedDb, fk.ReferencedTable)

			log.WithFields(log.Fields{
				"tableTcName":        tableTcName,
				"referencedTcName":   referencedTcName,
				"fk_TableName":       fk.TableName,
				"fk_ReferencedTable": fk.ReferencedTable,
				"tableDb":            tableDb,
				"referencedDb":       referencedDb,
			}).Info("Completed table config lookup")

			pbForeignKeys = append(pbForeignKeys, &sickrockpb.ForeignKey{
				ConstraintName:        fk.ConstraintName,
				TableName:             fk.TableName,
				ColumnName:            fk.ColumnName,
				ReferencedTable:       fk.ReferencedTable,
				ReferencedColumn:      fk.ReferencedColumn,
				OnDeleteAction:        fk.OnDeleteAction,
				OnUpdateAction:        fk.OnUpdateAction,
				TableTcName:           tableTcName,
				ReferencedTableTcName: referencedTcName,
			})
		}
	}

	// Get view type from default view if it exists, otherwise default to "table"
	viewType := "table"
	views, err := s.repo.GetTableViews(ctx, tcName)
	if err == nil && len(views) > 0 {
		defaultView := views[0] // GetTableViews returns views ordered, first is usually default
		for _, v := range views {
			if v.IsDefault {
				defaultView = v
				break
			}
		}
		if defaultView.ViewType != "" {
			viewType = defaultView.ViewType
		}
	}

	return connect.NewResponse(&sickrockpb.GetTableStructureResponse{
		Fields:           fields,
		CreateButtonText: createButtonText,
		View:             viewType,
		ForeignKeys:      pbForeignKeys,
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

	err := s.repo.CreateTableView(ctx, tableName, viewName, viewType, columns)
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

	err := s.repo.UpdateTableView(ctx, viewID, tableName, viewName, viewType, columns)
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
			Id:        int32(view.ID),
			TableName: view.TableName,
			ViewName:  view.ViewName,
			IsDefault: view.IsDefault,
			Columns:   pbColumns,
			ViewType:  viewType,
		})
	}

	return connect.NewResponse(&sickrockpb.GetTableViewsResponse{
		Views: pbViews,
	}), nil
}

func (s *SickRockServer) DeleteTableView(ctx context.Context, req *connect.Request[sickrockpb.DeleteTableViewRequest]) (*connect.Response[sickrockpb.DeleteTableViewResponse], error) {
	viewID := int(req.Msg.GetViewId())
	if viewID <= 0 {
		return connect.NewResponse(&sickrockpb.DeleteTableViewResponse{
			Success: false,
			Message: "Invalid view ID",
		}), nil
	}

	err := s.repo.DeleteTableView(ctx, viewID)
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
			ConstraintName:   fk.ConstraintName,
			TableName:        fk.TableName,
			ColumnName:       fk.ColumnName,
			ReferencedTable:  fk.ReferencedTable,
			ReferencedColumn: fk.ReferencedColumn,
			OnDeleteAction:   fk.OnDeleteAction,
			OnUpdateAction:   fk.OnUpdateAction,
		})
	}

	return connect.NewResponse(&sickrockpb.GetForeignKeysResponse{
		ForeignKeys: pbForeignKeys,
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
	limit := int(req.Msg.GetLimit())
	if limit <= 0 {
		limit = 10 // Default limit
	}

	items, err := s.repo.GetMostRecentlyViewed(ctx, limit)
	if err != nil {
		return nil, err
	}

	// Convert repository items to protobuf items
	var pbItems []*sickrockpb.RecentlyViewedItem
	for _, item := range items {
		pbItems = append(pbItems, &sickrockpb.RecentlyViewedItem{
			Name:          item.Name,
			TableId:       item.TableID,
			Icon:          item.Icon,
			UpdatedAtUnix: item.UpdatedAtUnix,
			ItemName:      item.ItemName,
			TableTitle:    item.TableTitle,
		})
	}

	return connect.NewResponse(&sickrockpb.GetMostRecentlyViewedResponse{
		Items: pbItems,
	}), nil
}

func (s *SickRockServer) GetSystemInfo(ctx context.Context, req *connect.Request[sickrockpb.GetSystemInfoRequest]) (*connect.Response[sickrockpb.GetSystemInfoResponse], error) {
	total, err := s.repo.GetApproxTotalRows(ctx)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&sickrockpb.GetSystemInfoResponse{ApproxTotalRows: total}), nil
}

func (s *SickRockServer) GetDashboards(ctx context.Context, req *connect.Request[sickrockpb.GetDashboardsRequest]) (*connect.Response[sickrockpb.GetDashboardsResponse], error) {
	dashboards, err := s.repo.ListDashboards(ctx)
	if err != nil {
		return nil, err
	}

	log.Infof("Dashboards: %v", dashboards)

	out := make([]*sickrockpb.Dashboard, 0, len(dashboards))
	for _, d := range dashboards {
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
			navItem = &sickrockpb.NavigationItem{
				Id:      int32(bookmark.NavigationItem.ID),
				Ordinal: int32(bookmark.NavigationItem.Ordinal),
				TableConfiguration: func() int32 {
					if bookmark.NavigationItem.TableConfiguration.Valid {
						return int32(bookmark.NavigationItem.TableConfiguration.Int64)
					}
					return 0
				}(),
				TableName:  bookmark.NavigationItem.TableName.String,
				TableTitle: bookmark.NavigationItem.TableTitle.String,
				Icon:       bookmark.NavigationItem.Icon.String,
				TableView:  bookmark.NavigationItem.TableView.String,
				DashboardId: func() int32 {
					if bookmark.NavigationItem.DashboardID.Valid {
						return int32(bookmark.NavigationItem.DashboardID.Int64)
					}
					return 0
				}(),
				DashboardName: bookmark.NavigationItem.DashboardName.String,
			}
		}

		pbBookmarks = append(pbBookmarks, &sickrockpb.UserBookmark{
			Id:               int32(bookmark.ID),
			UserId:           int32(bookmark.UserID),
			NavigationItemId: int32(bookmark.NavigationItemID),
			NavigationItem:   navItem,
			Title:            bookmark.Title.String,
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
		navItem = &sickrockpb.NavigationItem{
			Id:      int32(bookmark.NavigationItem.ID),
			Ordinal: int32(bookmark.NavigationItem.Ordinal),
			TableConfiguration: func() int32 {
				if bookmark.NavigationItem.TableConfiguration.Valid {
					return int32(bookmark.NavigationItem.TableConfiguration.Int64)
				}
				return 0
			}(),
			TableName:  bookmark.NavigationItem.TableName.String,
			TableTitle: bookmark.NavigationItem.TableTitle.String,
			Icon:       bookmark.NavigationItem.Icon.String,
			TableView:  bookmark.NavigationItem.TableView.String,
			DashboardId: func() int32 {
				if bookmark.NavigationItem.DashboardID.Valid {
					return int32(bookmark.NavigationItem.DashboardID.Int64)
				}
				return 0
			}(),
			DashboardName: bookmark.NavigationItem.DashboardName.String,
		}
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

	expiresAt := req.Msg.GetExpiresAt()
	var expiresAtTime *time.Time
	if expiresAt > 0 {
		t := time.Unix(expiresAt, 0)
		expiresAtTime = &t
	}

	// Generate a secure API key
	apiKey, err := s.generateSecureAPIKey()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to generate API key: %w", err))
	}

	// Hash the API key for storage
	keyHash, err := s.hashAPIKey(apiKey)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to hash API key: %w", err))
	}

	// Create the API key in the database
	createdAPIKey, err := s.repo.CreateAPIKey(ctx, userID, name, keyHash, expiresAtTime)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to create API key: %w", err))
	}

	return connect.NewResponse(&sickrockpb.CreateAPIKeyResponse{
		Success:  true,
		Message:  "API key created successfully",
		ApiKey:   apiKey, // Return the plain text key only once
		ApiKeyId: int32(createdAPIKey.ID),
	}), nil
}

// GetAPIKeys retrieves all API keys for the authenticated user
func (s *SickRockServer) GetAPIKeys(ctx context.Context, req *connect.Request[sickrockpb.GetAPIKeysRequest]) (*connect.Response[sickrockpb.GetAPIKeysResponse], error) {
	userID, err := s.getUserIDFromContext(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	apiKeys, err := s.repo.GetUserAPIKeys(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to retrieve API keys: %w", err))
	}

	var pbAPIKeys []*sickrockpb.APIKey
	for _, apiKey := range apiKeys {
		pbAPIKeys = append(pbAPIKeys, &sickrockpb.APIKey{
			Id:         int32(apiKey.ID),
			UserId:     int32(apiKey.UserID),
			Name:       apiKey.Name,
			CreatedAt:  apiKey.CreatedAt.Unix(),
			LastUsedAt: s.timeToUnixPtr(apiKey.LastUsedAt),
			ExpiresAt:  s.timeToUnixPtr(apiKey.ExpiresAt),
			IsActive:   apiKey.IsActive,
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

	err = s.repo.DeleteAPIKey(ctx, userID, apiKeyID)
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

	err = s.repo.DeactivateAPIKey(ctx, userID, apiKeyID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to deactivate API key: %w", err))
	}

	return connect.NewResponse(&sickrockpb.DeactivateAPIKeyResponse{
		Success: true,
		Message: "API key deactivated successfully",
	}), nil
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

// Helper methods for API key generation and hashing

func (s *SickRockServer) generateSecureAPIKey() (string, error) {
	// Generate 32 random bytes
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}

	// Convert to hex string and add prefix
	key := "sk_" + hex.EncodeToString(bytes)
	return key, nil
}

func (s *SickRockServer) hashAPIKey(apiKey string) (string, error) {
	hash := sha256.Sum256([]byte(apiKey))
	return hex.EncodeToString(hash[:]), nil
}

func (s *SickRockServer) timeToUnixPtr(t *time.Time) int64 {
	if t == nil {
		return 0
	}
	return t.Unix()
}
