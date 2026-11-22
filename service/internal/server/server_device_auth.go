package server

import (
	"context"
	"fmt"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	armstore "github.com/jamesread/armature-iam/store"

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
	if !isAuthenticated(au) {
		return connect.NewResponse(&sickrockpb.ClaimDeviceCodeResponse{
			Success: false,
			Message: "Authentication required to claim device code",
		}), nil
	}

	if err := s.repo.ClaimDeviceCode(ctx, code, au.User.Username); err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("device code not found, expired, or already claimed"))
	}

	sid := uuid.New().String()
	if err := s.auth.Store.CreateSession(ctx, sid, au.User.ID, nil); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
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
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if deviceCode == nil {
		return connect.NewResponse(&sickrockpb.CheckDeviceCodeResponse{Valid: false}), nil
	}

	claimedBy := nullStringOr(deviceCode.ClaimedBy, "")
	response := &sickrockpb.CheckDeviceCodeResponse{
		Valid:     true,
		Claimed:   claimedBy != "",
		ExpiresAt: deviceCode.ExpiresAt.Unix(),
	}

	if claimedBy != "" {
		s.attachClaimantSession(ctx, response, claimedBy)
	}

	return connect.NewResponse(response), nil
}

// attachClaimantSession adds the claimant's session to a check response. Best effort: the polling
// device picks the session up on a later check if this fails.
func (s *SickRockServer) attachClaimantSession(ctx context.Context, response *sickrockpb.CheckDeviceCodeResponse, claimedBy string) {
	user, sid, failure, err := s.claimantSession(ctx, claimedBy)
	if err != nil || failure != "" {
		return
	}
	response.Token = sid
	response.Username = user.Username
}

func (s *SickRockServer) GetDeviceCodeSession(ctx context.Context, req *connect.Request[sickrockpb.GetDeviceCodeSessionRequest]) (*connect.Response[sickrockpb.GetDeviceCodeSessionResponse], error) {
	code := req.Msg.GetCode()
	if code == "" {
		return connect.NewResponse(&sickrockpb.GetDeviceCodeSessionResponse{
			Success: false,
			Message: "Device code is required",
		}), nil
	}

	user, sid, failure, err := s.claimedDeviceCodeSession(ctx, code)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if failure != "" {
		return connect.NewResponse(&sickrockpb.GetDeviceCodeSessionResponse{
			Success: false,
			Message: failure,
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

// claimedDeviceCodeSession returns the user who claimed a device code and their latest session ID,
// or a user-facing failure message when the code is not claimed or the session cannot be found.
func (s *SickRockServer) claimedDeviceCodeSession(ctx context.Context, code string) (user *armstore.UserAccountRow, sid, failure string, err error) {
	claimedBy, failure, err := s.deviceCodeClaimant(ctx, code)
	if err != nil || failure != "" {
		return nil, "", failure, err
	}
	return s.claimantSession(ctx, claimedBy)
}

// deviceCodeClaimant returns the username that claimed a device code, or a user-facing failure
// message when the code is unknown, expired or not yet claimed.
func (s *SickRockServer) deviceCodeClaimant(ctx context.Context, code string) (claimedBy, failure string, err error) {
	deviceCode, err := s.repo.GetDeviceCode(ctx, code)
	if err != nil {
		return "", "", err
	}
	if deviceCode == nil {
		return "", "Device code not found or expired", nil
	}
	claimedBy = nullStringOr(deviceCode.ClaimedBy, "")
	if claimedBy == "" {
		return "", "Device code not yet claimed", nil
	}
	return claimedBy, "", nil
}

// claimantSession returns the user who claimed a device code and their latest session ID, or a
// user-facing failure message when the user or session cannot be found.
func (s *SickRockServer) claimantSession(ctx context.Context, username string) (user *armstore.UserAccountRow, sid, failure string, err error) {
	user, err = s.auth.Store.GetUserByUsername(ctx, username)
	if err != nil {
		return nil, "", "", err
	}
	if user == nil {
		return nil, "", "Failed to retrieve user", nil
	}
	sid, err = s.latestSessionID(ctx, user.ID)
	if err != nil {
		return nil, "", "", err
	}
	if sid == "" {
		return nil, "", "Session not found", nil
	}
	return user, sid, "", nil
}
