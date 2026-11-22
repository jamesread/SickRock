package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"connectrpc.com/connect"
	sickrockpb "github.com/jamesread/SickRock/gen/proto"
)

// maxHomeLayoutBytes bounds a stored home layout (a handful of sections and bookmark ids).
const maxHomeLayoutBytes = 16 * 1024

// GetHomeLayout returns the caller's own home page layout ("" for the default).
func (s *SickRockServer) GetHomeLayout(ctx context.Context, _ *connect.Request[sickrockpb.GetHomeLayoutRequest]) (*connect.Response[sickrockpb.GetHomeLayoutResponse], error) {
	userID, err := s.getUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	layout, err := s.repo.HomeLayout(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("load home layout: %w", err))
	}
	return connect.NewResponse(&sickrockpb.GetHomeLayoutResponse{LayoutJson: layout}), nil
}

// SetHomeLayout saves the caller's own home page layout. It must be a JSON object; the frontend
// interprets it and ignores what it does not know, so older and newer layouts both load.
func (s *SickRockServer) SetHomeLayout(ctx context.Context, req *connect.Request[sickrockpb.SetHomeLayoutRequest]) (*connect.Response[sickrockpb.SetHomeLayoutResponse], error) {
	userID, err := s.getUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	layout := strings.TrimSpace(req.Msg.GetLayoutJson())
	if err := validateHomeLayout(layout); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	if err := s.repo.SetHomeLayout(ctx, userID, layout); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("save home layout: %w", err))
	}
	return connect.NewResponse(&sickrockpb.SetHomeLayoutResponse{}), nil
}

func validateHomeLayout(layout string) error {
	if layout == "" {
		return nil
	}
	if len(layout) > maxHomeLayoutBytes {
		return fmt.Errorf("home layout is larger than %d bytes", maxHomeLayoutBytes)
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(layout), &obj); err != nil || obj == nil {
		return errors.New("home layout must be a JSON object")
	}
	return nil
}
