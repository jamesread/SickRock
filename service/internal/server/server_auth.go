package server

import (
	"context"
	"net"
	"strings"
	"time"

	"connectrpc.com/connect"
	armlayer "github.com/jamesread/armature-iam/layer"
	"github.com/google/uuid"

	sickrockpb "github.com/jamesread/SickRock/gen/proto"
	"github.com/jamesread/SickRock/internal/iam"
	log "github.com/sirupsen/logrus"
)

func (s *SickRockServer) Login(ctx context.Context, req *connect.Request[sickrockpb.LoginRequest]) (*connect.Response[sickrockpb.LoginResponse], error) {
	username := strings.TrimSpace(req.Msg.GetUsername())
	password := req.Msg.GetPassword()

	if username == "" || password == "" {
		return connect.NewResponse(&sickrockpb.LoginResponse{
			Success: false,
			Message: "Username and password are required",
		}), nil
	}

	user, err := s.auth.Store.GetUserByUsername(ctx, username)
	if err != nil || user == nil {
		return connect.NewResponse(&sickrockpb.LoginResponse{
			Success: false,
			Message: "Invalid credentials",
		}), nil
	}

	ok, err := iam.VerifyPassword(user.PasswordHash, password)
	if err != nil || !ok {
		return connect.NewResponse(&sickrockpb.LoginResponse{
			Success: false,
			Message: "Invalid credentials",
		}), nil
	}

	userAgent := req.Header().Get("User-Agent")
	ipAddress := getClientIP(req)

	sid := uuid.New().String()
	if err := s.auth.Store.CreateSession(ctx, sid, user.ID, nil); err != nil {
		return connect.NewResponse(&sickrockpb.LoginResponse{
			Success: false,
			Message: "Failed to create session",
		}), nil
	}

	go func() {
		notificationCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		data := map[string]interface{}{
			"username":   username,
			"user_agent": userAgent,
			"ip_address": ipAddress,
		}
		if err := s.notificationService.SendNotification(notificationCtx, "user.logged_in", data); err != nil {
			log.WithError(err).WithField("username", username).Warn("Failed to send login notification")
		}
	}()

	res := connect.NewResponse(&sickrockpb.LoginResponse{
		Success:   true,
		Message:   "Login successful",
		Token:     sid,
		ExpiresAt: sessionExpiresAt(),
	})
	sessionCookie := s.auth.NewSessionCookie(sid)
	res.Header().Add("Set-Cookie", (&sessionCookie).String())
	return res, nil
}

func (s *SickRockServer) Logout(ctx context.Context, req *connect.Request[sickrockpb.LogoutRequest]) (*connect.Response[sickrockpb.LogoutResponse], error) {
	sid := sessionIDFromRequest(req)
	if sid != "" {
		_ = s.auth.Store.DeleteSession(ctx, sid)
	}

	res := connect.NewResponse(&sickrockpb.LogoutResponse{
		Success: true,
		Message: "Logout successful",
	})
	clearCookie := s.auth.ClearSessionCookie()
	res.Header().Add("Set-Cookie", (&clearCookie).String())
	return res, nil
}

func (s *SickRockServer) ValidateToken(ctx context.Context, req *connect.Request[sickrockpb.ValidateTokenRequest]) (*connect.Response[sickrockpb.ValidateTokenResponse], error) {
	sid := strings.TrimSpace(req.Msg.GetToken())
	if sid == "" {
		sid = sessionIDFromRequest(req)
	}
	if sid == "" {
		return connect.NewResponse(&sickrockpb.ValidateTokenResponse{Valid: false}), nil
	}

	sess, err := s.auth.Store.GetSessionBySID(ctx, sid)
	if err != nil || sess == nil {
		return connect.NewResponse(&sickrockpb.ValidateTokenResponse{Valid: false}), nil
	}

	user, err := s.auth.Store.GetUserByID(ctx, sess.UserAccountID)
	if err != nil || user == nil {
		return connect.NewResponse(&sickrockpb.ValidateTokenResponse{Valid: false}), nil
	}

	rb, err := s.auth.Store.LoadEffectiveRBAC(ctx, user.ID)
	if err != nil {
		return connect.NewResponse(&sickrockpb.ValidateTokenResponse{Valid: false}), nil
	}

	au := &armlayer.AuthenticatedUser{User: user, RBAC: rb}
	_, rbacPerms, rbacSuperuser := iamUserToProto(au)

	return connect.NewResponse(&sickrockpb.ValidateTokenResponse{
		Valid:           true,
		Username:        user.Username,
		ExpiresAt:       sessionExpiresAt(),
		InitialRoute:    s.getInitialRoute(ctx, user.ID),
		RbacPermissions: rbacPerms,
		RbacIsSuperuser: rbacSuperuser,
	}), nil
}

func (s *SickRockServer) ResetUserPassword(ctx context.Context, req *connect.Request[sickrockpb.ResetUserPasswordRequest]) (*connect.Response[sickrockpb.ResetUserPasswordResponse], error) {
	username := strings.TrimSpace(req.Msg.GetUsername())
	newPassword := req.Msg.GetNewPassword()

	if username == "" || newPassword == "" {
		return connect.NewResponse(&sickrockpb.ResetUserPasswordResponse{Success: false, Message: "username and new_password are required"}), nil
	}
	if len(newPassword) < 8 {
		return connect.NewResponse(&sickrockpb.ResetUserPasswordResponse{Success: false, Message: "password must be at least 8 characters"}), nil
	}

	user, err := s.auth.Store.GetUserByUsername(ctx, username)
	if err != nil || user == nil {
		return connect.NewResponse(&sickrockpb.ResetUserPasswordResponse{Success: false, Message: "user not found"}), nil
	}

	hash, err := iam.HashPassword(newPassword)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if err := s.auth.Store.UpdateUserPassword(ctx, user.ID, hash); err != nil {
		return connect.NewResponse(&sickrockpb.ResetUserPasswordResponse{Success: false, Message: err.Error()}), nil
	}

	return connect.NewResponse(&sickrockpb.ResetUserPasswordResponse{Success: true, Message: "password updated"}), nil
}

func getClientIP(req connect.AnyRequest) string {
	if forwardedFor := req.Header().Get("X-Forwarded-For"); forwardedFor != "" {
		if ip := net.ParseIP(forwardedFor); ip != nil {
			return ip.String()
		}
	}
	if realIP := req.Header().Get("X-Real-IP"); realIP != "" {
		if ip := net.ParseIP(realIP); ip != nil {
			return ip.String()
		}
	}
	return "unknown"
}
