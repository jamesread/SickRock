package server

import (
	"context"
	"fmt"
	"math"
	"os"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/expr-lang/expr"

	sickrockpb "github.com/jamesread/SickRock/gen/proto"
	"github.com/jamesread/SickRock/internal/auth"
	"github.com/jamesread/SickRock/internal/buildinfo"
	repo "github.com/jamesread/SickRock/internal/repo"
	log "github.com/sirupsen/logrus"
)

type SickRockServer struct {
	repo *repo.Repository
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

func (s *SickRockServer) Init(ctx context.Context, req *connect.Request[sickrockpb.InitRequest]) (*connect.Response[sickrockpb.InitResponse], error) {
	dbName := strings.TrimSpace(os.Getenv("DB_NAME"))

	res := connect.NewResponse(&sickrockpb.InitResponse{
		Version: buildinfo.Version,
		Commit:  buildinfo.Commit,
		Date:    buildinfo.Date,
		DbName:  dbName,
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
		pages = append(pages, &sickrockpb.Page{
			Id:      config.Name,
			Title:   config.Title,
			Slug:    config.Name,
			Ordinal: int32(config.Ordinal),
			Icon:    config.Icon.String,
			View:    config.View.String,
		})
	}
	res := connect.NewResponse(&sickrockpb.GetTableConfigurationsResponse{Pages: pages})
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
			TableView:  item.TableView.String,
			DashboardId: func() int32 {
				if item.DashboardID.Valid {
					return int32(item.DashboardID.Int64)
				}
				return 0
			}(),
			DashboardName: item.DashboardName.String,
			Title:         item.Navigation.String,
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

	res := connect.NewResponse(&sickrockpb.GetNavigationResponse{
		Items:     navigationItems,
		Bookmarks: bookmarks,
	})
	return res, nil
}

func (s *SickRockServer) ListItems(ctx context.Context, req *connect.Request[sickrockpb.ListItemsRequest]) (*connect.Response[sickrockpb.ListItemsResponse], error) {
	// Use page_id as table name for this simple mapping
	table := req.Msg.GetTcName()
	if table == "" {
		table = "items"
	}
	// Ensure table exists
	if err := s.repo.EnsureSchemaForTable(ctx, table); err != nil {
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

	out := make([]*sickrockpb.Item, 0, len(items))
	for _, it := range items {
		// Convert dynamic fields to string map for protobuf
		additionalFields := make(map[string]string)
		for key, value := range it.Fields {
			if value != nil {
				additionalFields[key] = fmt.Sprintf("%v", value)
			}
		}

		item := &sickrockpb.Item{
			Id:               it.ID,
			SrCreated:        it.SrCreated.Unix(),
			AdditionalFields: additionalFields,
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
	if err := s.repo.EnsureSchemaForTable(ctx, table); err != nil {
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
			additionalFields[key] = fmt.Sprintf("%v", value)
		}
	}

	return connect.NewResponse(&sickrockpb.CreateItemResponse{Item: &sickrockpb.Item{
		Id:               it.ID,
		SrCreated:        it.SrCreated.Unix(),
		AdditionalFields: additionalFields,
	}}), nil
}

func (s *SickRockServer) GetItem(ctx context.Context, req *connect.Request[sickrockpb.GetItemRequest]) (*connect.Response[sickrockpb.GetItemResponse], error) {
	// Get table name from the request, default to "items" for backward compatibility
	table := req.Msg.GetPageId()
	if table == "" {
		table = "items"
	}

	// Ensure table exists
	if err := s.repo.EnsureSchemaForTable(ctx, table); err != nil {
		return nil, err
	}

	it, err := s.repo.GetItemInTable(ctx, table, req.Msg.GetId())
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
			additionalFields[key] = fmt.Sprintf("%v", value)
		}
	}

	return connect.NewResponse(&sickrockpb.GetItemResponse{Item: &sickrockpb.Item{
		Id:               it.ID,
		SrCreated:        it.SrCreated.Unix(),
		AdditionalFields: additionalFields,
	}}), nil
}

func (s *SickRockServer) EditItem(ctx context.Context, req *connect.Request[sickrockpb.EditItemRequest]) (*connect.Response[sickrockpb.EditItemResponse], error) {
	// Get table name from the request, default to "items" for backward compatibility
	table := req.Msg.GetPageId()
	if table == "" {
		table = "items"
	}

	// Ensure table exists
	if err := s.repo.EnsureSchemaForTable(ctx, table); err != nil {
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
			responseAdditionalFields[key] = fmt.Sprintf("%v", value)
		}
	}

	return connect.NewResponse(&sickrockpb.EditItemResponse{Item: &sickrockpb.Item{
		Id:               it.ID,
		SrCreated:        it.SrCreated.Unix(),
		AdditionalFields: responseAdditionalFields,
	}}), nil
}

func (s *SickRockServer) DeleteItem(ctx context.Context, req *connect.Request[sickrockpb.DeleteItemRequest]) (*connect.Response[sickrockpb.DeleteItemResponse], error) {
	table := req.Msg.GetPageId()
	if table == "" {
		table = "items"
	}
	if err := s.repo.EnsureSchemaForTable(ctx, table); err != nil {
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

	return connect.NewResponse(&sickrockpb.GetTableStructureResponse{
		Fields:           fields,
		CreateButtonText: createButtonText,
		View:             tc.View.String,
	}), nil
}

func (s *SickRockServer) AddTableColumn(ctx context.Context, req *connect.Request[sickrockpb.AddTableColumnRequest]) (*connect.Response[sickrockpb.GetTableStructureResponse], error) {
	tc, err := s.repo.GetTableConfiguration(ctx, req.Msg.GetPageId())
	if err != nil {
		return nil, err
	}

	if err := s.repo.EnsureSchemaForTable(ctx, tc.Table.String); err != nil {
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

	err := s.repo.CreateTableView(ctx, tableName, viewName, columns)
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

	err := s.repo.UpdateTableView(ctx, viewID, tableName, viewName, columns)
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

		pbViews = append(pbViews, &sickrockpb.TableView{
			Id:        int32(view.ID),
			TableName: view.TableName,
			ViewName:  view.ViewName,
			IsDefault: view.IsDefault,
			Columns:   pbColumns,
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
	if columnName == "id" || columnName == "sr_created" {
		return connect.NewResponse(&sickrockpb.DropColumnResponse{
			Success: false,
			Message: "Cannot drop system columns (id, sr_created)",
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
