package server

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/jamesread/SickRock/internal/iam"
	armlayer "github.com/jamesread/armature-iam/layer"
)

func (s *SickRockServer) authUser(ctx context.Context) *armlayer.AuthenticatedUser {
	return armlayer.UserFromContext(ctx)
}

func sessionIDFromRequest(req connect.AnyRequest) string {
	if h := req.Header().Get("Session-Token"); h != "" {
		return h
	}
	if sid := bearerSessionID(req.Header().Get("Authorization")); sid != "" {
		return sid
	}
	return cookieSessionID(req.Header().Get("Cookie"))
}

// bearerSessionID returns a bearer token that is a session ID (API keys, prefixed sk_, are not).
func bearerSessionID(authorization string) string {
	parts := strings.SplitN(authorization, " ", 2)
	if len(parts) == 2 && parts[0] == "Bearer" && parts[1] != "" && !strings.HasPrefix(parts[1], "sk_") {
		return parts[1]
	}
	return ""
}

// cookieSessionID returns the session cookie's value from a Cookie header, or "".
func cookieSessionID(cookieHeader string) string {
	prefix := iam.CookieName + "="
	for part := range strings.SplitSeq(cookieHeader, ";") {
		if after, ok := strings.CutPrefix(strings.TrimSpace(part), prefix); ok {
			return after
		}
	}
	return ""
}

func (s *SickRockServer) resolveSessionUser(ctx context.Context, req connect.AnyRequest) (*armlayer.AuthenticatedUser, error) {
	sid := sessionIDFromRequest(req)
	if sid == "" {
		return nil, nil
	}
	sess, err := s.auth.Store.GetSessionBySID(ctx, sid)
	if err != nil || sess == nil {
		return nil, err
	}
	user, err := s.auth.Store.GetUserByID(ctx, sess.UserAccountID)
	if err != nil || user == nil {
		return nil, err
	}
	rb, err := s.auth.Store.LoadEffectiveRBAC(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	return &armlayer.AuthenticatedUser{User: user, RBAC: rb}, nil
}

func (s *SickRockServer) latestSessionID(ctx context.Context, userID int) (string, error) {
	var sid string
	err := s.repo.DB().GetContext(ctx, &sid,
		`SELECT sid FROM sessions WHERE user_account_id = ? ORDER BY id DESC LIMIT 1`, userID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return sid, err
}

func rbacPermissionNames(au *armlayer.AuthenticatedUser) []string {
	if au == nil || au.RBAC == nil {
		return nil
	}
	if au.RBAC.IsSuperuser {
		names := make([]string, 0, len(au.RBAC.Permissions))
		for n := range au.RBAC.Permissions {
			names = append(names, n)
		}
		sort.Strings(names)
		return names
	}
	names := make([]string, 0, len(au.RBAC.Permissions))
	for n, ok := range au.RBAC.Permissions {
		if ok {
			names = append(names, n)
		}
	}
	sort.Strings(names)
	return names
}

func (s *SickRockServer) getUserIDFromContext(ctx context.Context) (int, error) {
	au := s.authUser(ctx)
	if !isAuthenticated(au) {
		return 0, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}
	return au.User.ID, nil
}

func (s *SickRockServer) getInitialRoute(ctx context.Context, userID int) string {
	var route string
	err := s.repo.DB().GetContext(ctx, &route, `SELECT initial_route FROM user_accounts WHERE id = ?`, userID)
	if err != nil || route == "" {
		return "/"
	}
	return route
}

func sessionExpiresAt() int64 {
	return time.Now().Add(10 * 365 * 24 * time.Hour).Unix()
}

func iamUserToProto(u *armlayer.AuthenticatedUser) (username string, perms []string, superuser bool) {
	if u == nil || u.User == nil {
		return "", nil, false
	}
	return u.User.Username, rbacPermissionNames(u), u.RBAC != nil && u.RBAC.IsSuperuser
}

func parseIAMTimestamp(value string) int64 {
	if value == "" {
		return 0
	}
	layouts := []string{
		"2006-01-02 15:04:05",
		time.RFC3339,
		"2006-01-02T15:04:05Z",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, value); err == nil {
			return t.Unix()
		}
	}
	return 0
}
