package server

import (
	"context"
	"net"
	"strings"
	"time"

	"connectrpc.com/connect"

	sickrockpb "github.com/jamesread/SickRock/gen/proto"
	log "github.com/sirupsen/logrus"
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

	// Extract client information
	userAgent := req.Header().Get("User-Agent")
	ipAddress := getClientIP(req)

	// Validate against database and create session
	token, expiresAt, err := s.authService.Login(ctx, username, password, userAgent, ipAddress)
	if err != nil {
		return connect.NewResponse(&sickrockpb.LoginResponse{
			Success: false,
			Message: "Invalid credentials",
		}), nil
	}

	// Trigger notification for user login (async, don't block login)
	go func() {
		notificationCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		data := map[string]interface{}{
			"username":  username,
			"user_agent": userAgent,
			"ip_address": ipAddress,
		}

		if err := s.notificationService.SendNotification(notificationCtx, "user.logged_in", data); err != nil {
			log.WithError(err).WithField("username", username).Warn("Failed to send login notification")
		}
	}()

	return connect.NewResponse(&sickrockpb.LoginResponse{
		Success:   true,
		Message:   "Login successful",
		Token:     token,
		ExpiresAt: expiresAt.Unix(),
	}), nil
}

func (s *SickRockServer) Logout(ctx context.Context, req *connect.Request[sickrockpb.LogoutRequest]) (*connect.Response[sickrockpb.LogoutResponse], error) {
	// Get token from Authorization header
	authHeader := req.Header().Get("Authorization")
	if authHeader == "" {
		return connect.NewResponse(&sickrockpb.LogoutResponse{
			Success: false,
			Message: "Authorization header required",
		}), nil
	}

	// Extract token from "Bearer <token>"
	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || parts[0] != "Bearer" {
		return connect.NewResponse(&sickrockpb.LogoutResponse{
			Success: false,
			Message: "Invalid authorization header format",
		}), nil
	}

	token := parts[1]

	// Invalidate session in database
	err := s.authService.Logout(ctx, token)
	if err != nil {
		return connect.NewResponse(&sickrockpb.LogoutResponse{
			Success: false,
			Message: "Logout failed",
		}), nil
	}

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

	claims, err := s.authService.ValidateToken(ctx, token)
	if err != nil {
		return connect.NewResponse(&sickrockpb.ValidateTokenResponse{
			Valid: false,
		}), nil
	}

	// Get user information to retrieve initial_route
	user, err := s.repo.GetUserByUsername(ctx, claims.Username)
	if err != nil || user == nil {
		return connect.NewResponse(&sickrockpb.ValidateTokenResponse{
			Valid: false,
		}), nil
	}

	// Set default initial_route if empty
	initialRoute := user.InitialRoute
	if initialRoute == "" {
		initialRoute = "/"
	}

	return connect.NewResponse(&sickrockpb.ValidateTokenResponse{
		Valid:        true,
		Username:     claims.Username,
		ExpiresAt:    claims.ExpiresAt.Time.Unix(),
		InitialRoute: initialRoute,
	}), nil
}

// ResetUserPassword allows an authenticated admin to reset a user's password.
func (s *SickRockServer) ResetUserPassword(ctx context.Context, req *connect.Request[sickrockpb.ResetUserPasswordRequest]) (*connect.Response[sickrockpb.ResetUserPasswordResponse], error) {
	username := strings.TrimSpace(req.Msg.GetUsername())
	newPassword := req.Msg.GetNewPassword()

	if username == "" || newPassword == "" {
		return connect.NewResponse(&sickrockpb.ResetUserPasswordResponse{Success: false, Message: "username and new_password are required"}), nil
	}

	if err := s.repo.UpdateUserPassword(ctx, username, newPassword); err != nil {
		return connect.NewResponse(&sickrockpb.ResetUserPasswordResponse{Success: false, Message: err.Error()}), nil
	}

	return connect.NewResponse(&sickrockpb.ResetUserPasswordResponse{Success: true, Message: "password updated"}), nil
}

func getClientIP(req connect.AnyRequest) string {
	// Try to get IP from X-Forwarded-For header first
	if forwardedFor := req.Header().Get("X-Forwarded-For"); forwardedFor != "" {
		// X-Forwarded-For can contain multiple IPs, take the first one
		if ip := net.ParseIP(forwardedFor); ip != nil {
			return ip.String()
		}
	}

	// Try X-Real-IP header
	if realIP := req.Header().Get("X-Real-IP"); realIP != "" {
		if ip := net.ParseIP(realIP); ip != nil {
			return ip.String()
		}
	}

	// Fallback to remote address
	// Note: This might not work in all cases with ConnectRPC
	return "unknown"
}
