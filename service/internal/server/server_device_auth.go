package server

import (
	"context"
	"fmt"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	sickrockpb "github.com/jamesread/SickRock/gen/proto"
)

func (s *SickRockServer) GenerateDeviceCode(ctx context.Context, req *connect.Request[sickrockpb.GenerateDeviceCodeRequest]) (*connect.Response[sickrockpb.GenerateDeviceCodeResponse], error) {
	code, err := s.repo.GenerateDeviceCode()
	if err != nil {
		return connect.NewResponse(&sickrockpb.GenerateDeviceCodeResponse{
			Code:      "",
			ExpiresAt: 0,
		}), fmt.Errorf("failed to generate device code: %w", err)
	}

	expiresAt := time.Now().Add(10 * time.Minute)
	if err := s.repo.CreateDeviceCode(ctx, code, expiresAt); err != nil {
		return connect.NewResponse(&sickrockpb.GenerateDeviceCodeResponse{
			Code:      "",
			ExpiresAt: 0,
		}), fmt.Errorf("failed to store device code: %w", err)
	}

	return connect.NewResponse(&sickrockpb.GenerateDeviceCodeResponse{
		Code:      code,
		ExpiresAt: expiresAt.Unix(),
	}), nil
}

func (s *SickRockServer) ClaimDeviceCode(ctx context.Context, req *connect.Request[sickrockpb.ClaimDeviceCodeRequest]) (*connect.Response[sickrockpb.ClaimDeviceCodeResponse], error) {
	code := req.Msg.GetCode()
	if code == "" {
		return connect.NewResponse(&sickrockpb.ClaimDeviceCodeResponse{
			Success: false,
			Message: "Device code is required",
		}), nil
	}

	au := s.authUser(ctx)
	if au == nil || au.User == nil {
		return connect.NewResponse(&sickrockpb.ClaimDeviceCodeResponse{
			Success: false,
			Message: "Authentication required to claim device code",
		}), nil
	}

	if err := s.repo.ClaimDeviceCode(ctx, code, au.User.Username); err != nil {
		return connect.NewResponse(&sickrockpb.ClaimDeviceCodeResponse{
			Success: false,
			Message: "Device code not found, expired, or already claimed",
		}), nil
	}

	sid := uuid.New().String()
	if err := s.auth.Store.CreateSession(ctx, sid, au.User.ID, nil); err != nil {
		return connect.NewResponse(&sickrockpb.ClaimDeviceCodeResponse{
			Success: false,
			Message: "Failed to create session",
		}), nil
	}

	return connect.NewResponse(&sickrockpb.ClaimDeviceCodeResponse{
		Success:   true,
		Message:   "Device code claimed successfully",
		Token:     sid,
		ExpiresAt: sessionExpiresAt(),
	}), nil
}

func (s *SickRockServer) CheckDeviceCode(ctx context.Context, req *connect.Request[sickrockpb.CheckDeviceCodeRequest]) (*connect.Response[sickrockpb.CheckDeviceCodeResponse], error) {
	code := req.Msg.GetCode()
	if code == "" {
		return connect.NewResponse(&sickrockpb.CheckDeviceCodeResponse{Valid: false}), nil
	}

	deviceCode, err := s.repo.GetDeviceCode(ctx, code)
	if err != nil || deviceCode == nil {
		return connect.NewResponse(&sickrockpb.CheckDeviceCodeResponse{Valid: false}), nil
	}

	claimed := deviceCode.ClaimedBy.Valid && deviceCode.ClaimedBy.String != ""
	response := &sickrockpb.CheckDeviceCodeResponse{
		Valid:     true,
		Claimed:   claimed,
		ExpiresAt: deviceCode.ExpiresAt.Unix(),
	}

	if claimed {
		user, err := s.auth.Store.GetUserByUsername(ctx, deviceCode.ClaimedBy.String)
		if err == nil && user != nil {
			if sid, err := s.latestSessionID(ctx, user.ID); err == nil && sid != "" {
				response.Token = sid
				response.Username = user.Username
			}
		}
	}

	return connect.NewResponse(response), nil
}

func (s *SickRockServer) GetDeviceCodeSession(ctx context.Context, req *connect.Request[sickrockpb.GetDeviceCodeSessionRequest]) (*connect.Response[sickrockpb.GetDeviceCodeSessionResponse], error) {
	code := req.Msg.GetCode()
	if code == "" {
		return connect.NewResponse(&sickrockpb.GetDeviceCodeSessionResponse{
			Success: false,
			Message: "Device code is required",
		}), nil
	}

	deviceCode, err := s.repo.GetDeviceCode(ctx, code)
	if err != nil || deviceCode == nil {
		return connect.NewResponse(&sickrockpb.GetDeviceCodeSessionResponse{
			Success: false,
			Message: "Device code not found or expired",
		}), nil
	}

	if !deviceCode.ClaimedBy.Valid || deviceCode.ClaimedBy.String == "" {
		return connect.NewResponse(&sickrockpb.GetDeviceCodeSessionResponse{
			Success: false,
			Message: "Device code not yet claimed",
		}), nil
	}

	user, err := s.auth.Store.GetUserByUsername(ctx, deviceCode.ClaimedBy.String)
	if err != nil || user == nil {
		return connect.NewResponse(&sickrockpb.GetDeviceCodeSessionResponse{
			Success: false,
			Message: "Failed to retrieve user",
		}), nil
	}

	sid, err := s.latestSessionID(ctx, user.ID)
	if err != nil || sid == "" {
		return connect.NewResponse(&sickrockpb.GetDeviceCodeSessionResponse{
			Success: false,
			Message: "Session not found",
		}), nil
	}

	return connect.NewResponse(&sickrockpb.GetDeviceCodeSessionResponse{
		Success:   true,
		Message:   "Session retrieved successfully",
		Token:     sid,
		ExpiresAt: sessionExpiresAt(),
		Username:  user.Username,
	}), nil
}
