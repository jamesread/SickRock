package server

import (
	"context"
	"fmt"
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
		log.Infof("Processing item: ID=%s, CreatedAtUnix=%d, Fields=%+v", it.ID, it.CreatedAtUnix, it.Fields)

		// Convert dynamic fields to string map for protobuf
		additionalFields := make(map[string]string)
		for key, value := range it.Fields {
			log.Infof("Additional field - key: %s, value: %v", key, value)
			if value != nil {
				additionalFields[key] = fmt.Sprintf("%v", value)
			}
		}

		item := &sickrockpb.Item{
			Id:               it.ID,
			CreatedAtUnix:    it.CreatedAtUnix,
			AdditionalFields: additionalFields,
		}

		log.Infof("Created protobuf item: ID=%s, CreatedAtUnix=%d, AdditionalFields=%+v",
			item.Id, item.CreatedAtUnix, item.AdditionalFields)

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

	// Use custom timestamp if provided, otherwise use current time
	var timestamp int64
	if req.Msg.GetCreatedAtUnix() != 0 {
		timestamp = req.Msg.GetCreatedAtUnix()
	} else {
		timestamp = time.Now().Unix()
	}

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
		CreatedAtUnix:    it.CreatedAtUnix,
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
		CreatedAtUnix:    it.CreatedAtUnix,
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
		CreatedAtUnix:    it.CreatedAtUnix,
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
	return connect.NewResponse(&sickrockpb.GetTableStructureResponse{Fields: fields, CreateButtonText: structure.CreateButtonText}), nil
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
