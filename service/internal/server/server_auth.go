package server

import (
	"context"

	"connectrpc.com/connect"

	sickrockpb "github.com/jamesread/SickRock/gen/proto"
	"github.com/jamesread/SickRock/internal/auth"
)

func (s *SickRockServer) Login(ctx context.Context, req *connect.Request[sickrockpb.LoginRequest]) (*connect.Response[sickrockpb.LoginResponse], error) {
	username := req.Msg.GetUsername()
	password := req.Msg.GetPassword()

	if username == "" || password == "" {
		return connect.NewResponse(&sickrockpb.LoginResponse{
			Success: false,
			Message: "Username and password are required",
		}), nil
	}

	// For now, we'll use a simple hardcoded user
	// In a real application, you'd validate against a database
	authService := auth.NewAuthService()
	token, expiresAt, err := authService.Login(username, password)
	if err != nil {
		return connect.NewResponse(&sickrockpb.LoginResponse{
			Success: false,
			Message: "Invalid credentials",
		}), nil
	}

	return connect.NewResponse(&sickrockpb.LoginResponse{
		Success:   true,
		Message:   "Login successful",
		Token:     token,
		ExpiresAt: expiresAt.Unix(),
	}), nil
}

func (s *SickRockServer) Logout(ctx context.Context, req *connect.Request[sickrockpb.LogoutRequest]) (*connect.Response[sickrockpb.LogoutResponse], error) {
	// For JWT tokens, logout is handled client-side by discarding the token
	// In a more sophisticated system, you might maintain a blacklist
	return connect.NewResponse(&sickrockpb.LogoutResponse{
		Success: true,
		Message: "Logout successful",
	}), nil
}

func (s *SickRockServer) ValidateToken(ctx context.Context, req *connect.Request[sickrockpb.ValidateTokenRequest]) (*connect.Response[sickrockpb.ValidateTokenResponse], error) {
	token := req.Msg.GetToken()
	if token == "" {
		return connect.NewResponse(&sickrockpb.ValidateTokenResponse{
			Valid: false,
		}), nil
	}

	authService := auth.NewAuthService()
	claims, err := authService.ValidateToken(token)
	if err != nil {
		return connect.NewResponse(&sickrockpb.ValidateTokenResponse{
			Valid: false,
		}), nil
	}

	return connect.NewResponse(&sickrockpb.ValidateTokenResponse{
		Valid:      true,
		Username:   claims.Username,
		ExpiresAt:  claims.ExpiresAt.Time.Unix(),
	}), nil
}
