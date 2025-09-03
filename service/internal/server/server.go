package server

import (
	"context"
	"fmt"
	"time"

	"connectrpc.com/connect"

	sickrockpb "github.com/jamesread/SickRock/gen/proto"
	repo "github.com/jamesread/SickRock/internal/repo"
	log "github.com/sirupsen/logrus"
)

type SickRockServer struct {
	repo *repo.Repository
}

func NewSickRockServer(r *repo.Repository) *SickRockServer {
	return &SickRockServer{repo: r}
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
	names, err := s.repo.ListTableConfigurations(ctx)
	if err != nil {
		return nil, err
	}
	pages := make([]*sickrockpb.Page, 0, len(names))
	for _, n := range names {
		pages = append(pages, &sickrockpb.Page{Id: n, Title: n, Slug: n})
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
			log.Infof("key: %s, value: %v", key, value)
			if value != nil {
				additionalFields[key] = fmt.Sprintf("%v", value)
			}
		}

		log.Infof("name: %s", it.Name)

		out = append(out, &sickrockpb.Item{
			Id:               it.ID,
			Name:             it.Name,
			CreatedAtUnix:    it.CreatedAtUnix,
			AdditionalFields: additionalFields,
		})
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
	it, err := s.repo.CreateItemInTable(ctx, table, req.Msg.GetName())
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
		Name:             it.Name,
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
		Name:             it.Name,
		CreatedAtUnix:    it.CreatedAtUnix,
		AdditionalFields: additionalFields,
	}}), nil
}

func (s *SickRockServer) EditItem(ctx context.Context, req *connect.Request[sickrockpb.EditItemRequest]) (*connect.Response[sickrockpb.EditItemResponse], error) {
	it, err := s.repo.EditItem(ctx, req.Msg.GetId(), req.Msg.GetName())
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

	return connect.NewResponse(&sickrockpb.EditItemResponse{Item: &sickrockpb.Item{
		Id:               it.ID,
		Name:             it.Name,
		CreatedAtUnix:    it.CreatedAtUnix,
		AdditionalFields: additionalFields,
	}}), nil
}

func (s *SickRockServer) DeleteItem(ctx context.Context, req *connect.Request[sickrockpb.DeleteItemRequest]) (*connect.Response[sickrockpb.DeleteItemResponse], error) {
	ok, err := s.repo.DeleteItem(ctx, req.Msg.GetId())
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
	cols, err := s.repo.ListColumns(ctx, table)
	if err != nil {
		log.Errorf("list columns: %v, table: %s", err, table)
	} else {
		log.Infof("list columns: %v, table: %s", cols, table)
	}
	fields := make([]*sickrockpb.Field, 0, len(cols))
	for _, c := range cols {
		fields = append(fields, &sickrockpb.Field{Name: c.Name, Type: c.Type, Required: c.Required})
	}
	return connect.NewResponse(&sickrockpb.GetTableStructureResponse{Fields: fields}), nil
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
	err := s.repo.AddColumn(ctx, table, repo.FieldSpec{Name: f.GetName(), Type: f.GetType(), Required: f.GetRequired()})
	if err != nil {
		return nil, err
	}
	return s.GetTableStructure(ctx, &connect.Request[sickrockpb.GetTableStructureRequest]{Msg: &sickrockpb.GetTableStructureRequest{PageId: table}})
}
