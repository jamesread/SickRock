package server

import (
	"context"
	"net"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	armlayer "github.com/jamesread/armature-iam/layer"
	armstore "github.com/jamesread/armature-iam/store"

	sickrockpb "github.com/jamesread/SickRock/gen/proto"
	"github.com/jamesread/SickRock/internal/audit"
	"github.com/jamesread/SickRock/internal/iam"
	"github.com/jamesread/SickRock/internal/repo"
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

	userAgent := req.Header().Get("User-Agent")
	ipAddress := getClientIP(req)

	user, err := s.verifyLoginCredentials(ctx, username, password)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if user == nil {
		s.recordAudit(ctx, repo.AuditLogEntry{
			EventType:   audit.EventLoginFailed,
			Message:     "invalid credentials",
			RelatedUser: username,
			IPAddress:   ipAddress,
			UserAgent:   userAgent,
			Success:     false,
		})
		return connect.NewResponse(&sickrockpb.LoginResponse{
			Success: false,
			Message: "Invalid credentials",
		}), nil
	}

	sid := uuid.New().String()
	if err := s.auth.Store.CreateSession(ctx, sid, user.ID, nil); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	s.recordAudit(ctx, repo.AuditLogEntry{
		EventType:     audit.EventLogin,
		Message:       "login successful",
		ActorUserID:   user.ID,
		ActorUsername: username,
		RelatedUser:   username,
		IPAddress:     ipAddress,
		UserAgent:     userAgent,
		Success:       true,
	})

	s.notifyLogin(ctx, username, userAgent, ipAddress)

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

// verifyLoginCredentials returns the user when the username exists and the password matches, or nil
// for invalid credentials.
func (s *SickRockServer) verifyLoginCredentials(ctx context.Context, username, password string) (*armstore.UserAccountRow, error) {
	user, err := s.auth.Store.GetUserByUsername(ctx, username)
	if err != nil || user == nil {
		return nil, err
	}
	ok, err := iam.VerifyPassword(user.PasswordHash, password)
	if err != nil || !ok {
		return nil, err
	}
	return user, nil
}

// notifyLogin sends the user.logged_in notification in the background.
func (s *SickRockServer) notifyLogin(ctx context.Context, username, userAgent, ipAddress string) {
	go func() {
		notificationCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		data := map[string]any{
			"username":   username,
			"user_agent": userAgent,
			"ip_address": ipAddress,
		}
		if err := s.notificationService.SendNotification(notificationCtx, "user.logged_in", data); err != nil {
			log.WithError(err).WithField("username", username).Warn("Failed to send login notification")
		}
	}()
}

func (s *SickRockServer) Logout(ctx context.Context, req *connect.Request[sickrockpb.LogoutRequest]) (*connect.Response[sickrockpb.LogoutResponse], error) {
	actorID, actorName := s.auditActorFromContext(ctx)
	ipAddress := getClientIP(req)
	userAgent := req.Header().Get("User-Agent")

	sid := sessionIDFromRequest(req)
	if sid != "" {
		_ = s.auth.Store.DeleteSession(ctx, sid)
	}

	s.recordAudit(ctx, repo.AuditLogEntry{
		EventType:     audit.EventLogout,
		Message:       "logout",
		ActorUserID:   actorID,
		ActorUsername: actorName,
		RelatedUser:   actorName,
		IPAddress:     ipAddress,
		UserAgent:     userAgent,
		Success:       true,
	})

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

	user, err := s.sessionUser(ctx, sid)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if user == nil {
		return connect.NewResponse(&sickrockpb.ValidateTokenResponse{Valid: false}), nil
	}

	rb, err := s.auth.Store.LoadEffectiveRBAC(ctx, user.ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
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

// sessionUser returns the user a session ID belongs to, or nil when the ID is empty or the session or
// user no longer exists.
func (s *SickRockServer) sessionUser(ctx context.Context, sid string) (*armstore.UserAccountRow, error) {
	if sid == "" {
		return nil, nil
	}
	sess, err := s.auth.Store.GetSessionBySID(ctx, sid)
	if err != nil || sess == nil {
		return nil, err
	}
	return s.auth.Store.GetUserByID(ctx, sess.UserAccountID)
}

func (s *SickRockServer) ResetUserPassword(ctx context.Context, req *connect.Request[sickrockpb.ResetUserPasswordRequest]) (*connect.Response[sickrockpb.ResetUserPasswordResponse], error) {
	username := strings.TrimSpace(req.Msg.GetUsername())
	newPassword := req.Msg.GetNewPassword()

	if problem := passwordResetProblem(username, newPassword); problem != "" {
		return connect.NewResponse(&sickrockpb.ResetUserPasswordResponse{Success: false, Message: problem}), nil
	}

	user, err := s.auth.Store.GetUserByUsername(ctx, username)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if user == nil {
		return connect.NewResponse(&sickrockpb.ResetUserPasswordResponse{Success: false, Message: "user not found"}), nil
	}

	hash, err := iam.HashPassword(newPassword)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if err := s.auth.Store.UpdateUserPassword(ctx, user.ID, hash); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&sickrockpb.ResetUserPasswordResponse{Success: true, Message: "password updated"}), nil
}

// passwordResetProblem returns why a password reset request is invalid, or "".
func passwordResetProblem(username, newPassword string) string {
	if username == "" || newPassword == "" {
		return "username and new_password are required"
	}
	if len(newPassword) < 8 {
		return "password must be at least 8 characters"
	}
	return ""
}

func getClientIP(req connect.AnyRequest) string {
	if ip := headerClientIP(req, "X-Forwarded-For"); ip != "" {
		return ip
	}
	if ip := headerClientIP(req, "X-Real-IP"); ip != "" {
		return ip
	}
	return "unknown"
}

func headerClientIP(req connect.AnyRequest, header string) string {
	value := req.Header().Get(header)
	if value == "" {
		return ""
	}
	ip := net.ParseIP(value)
	if ip == nil {
		return ""
	}
	return ip.String()
}
