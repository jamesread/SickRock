package server

import (
	"context"
	"fmt"
	"strings"
	"time"

	"connectrpc.com/connect"

	sickrockpb "github.com/jamesread/SickRock/gen/proto"
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

func (s *SickRockServer) Init(ctx context.Context, req *connect.Request[sickrockpb.InitRequest]) (*connect.Response[sickrockpb.InitResponse], error) {
	res := connect.NewResponse(&sickrockpb.InitResponse{
		Version: buildinfo.Version,
		Commit:  buildinfo.Commit,
		Date:    buildinfo.Date,
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

func (s *SickRockServer) GetPages(ctx context.Context, req *connect.Request[sickrockpb.GetPagesRequest]) (*connect.Response[sickrockpb.GetPagesResponse], error) {
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
	res := connect.NewResponse(&sickrockpb.GetPagesResponse{Pages: pages})
	return res, nil
}

func (s *SickRockServer) ListItems(ctx context.Context, req *connect.Request[sickrockpb.ListItemsRequest]) (*connect.Response[sickrockpb.ListItemsResponse], error) {
	// Use page_id as table name for this simple mapping
	table := req.Msg.GetPageId()
	if table == "" {
		table = "items"
	}
	// Ensure table exists
	if err := s.repo.EnsureSchemaForTable(ctx, table); err != nil {
		return nil, err
	}
	items, err := s.repo.ListItemsInTable(ctx, table)

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

	// Always use current time for sr_created
	timestamp := time.Now()

	it, err := s.repo.CreateItemInTableWithTimestamp(ctx, table, req.Msg.GetAdditionalFields(), timestamp)
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
	// For demo, items are stored in default table
	it, err := s.repo.GetItem(ctx, req.Msg.GetId())
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
	table := req.Msg.GetPageId()
	if table == "" {
		table = "items"
	}
	if err := s.repo.EnsureSchemaForTable(ctx, table); err != nil {
		return nil, err
	}

	structure, err := s.repo.GetTableStructure(ctx, table)
	if err != nil {
		return nil, err
	}

	cols, err := s.repo.ListColumns(ctx, table)
	if err != nil {
		log.Errorf("list columns: %v, table: %s", err, table)
	} else {
		log.Infof("list columns: %v, table: %s", cols, table)
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

	log.Infof("GetTableStructureResponse: %+v", structure)

	return connect.NewResponse(&sickrockpb.GetTableStructureResponse{
		Fields:           fields,
		CreateButtonText: structure.CreateButtonText,
		View:             structure.View,
	}), nil
}

func (s *SickRockServer) AddTableColumn(ctx context.Context, req *connect.Request[sickrockpb.AddTableColumnRequest]) (*connect.Response[sickrockpb.GetTableStructureResponse], error) {
	table := req.Msg.GetPageId()
	if table == "" {
		table = "items"
	}
	if err := s.repo.EnsureSchemaForTable(ctx, table); err != nil {
		return nil, err
	}
	f := req.Msg.GetField()
	if f == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("field required"))
	}
	err := s.repo.AddColumn(ctx, table, repo.FieldSpec{
		Name:                      f.GetName(),
		Type:                      f.GetType(),
		Required:                  f.GetRequired(),
		DefaultToCurrentTimestamp: f.GetDefaultToCurrentTimestamp(),
	})
	if err != nil {
		return nil, err
	}
	return s.GetTableStructure(ctx, &connect.Request[sickrockpb.GetTableStructureRequest]{Msg: &sickrockpb.GetTableStructureRequest{PageId: table}})
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
