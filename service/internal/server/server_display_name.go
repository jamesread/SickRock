package server

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"connectrpc.com/connect"
	sickrockpb "github.com/jamesread/SickRock/gen/proto"
)

// MaxDisplayNameLength is the most characters a display name may have (user_accounts.display_name).
const MaxDisplayNameLength = 64

// normalizeDisplayName trims a display name and checks it: at most MaxDisplayNameLength characters
// and no control characters. "" (after trimming) clears it.
func normalizeDisplayName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if !utf8.ValidString(name) {
		return "", errors.New("display name must be valid text")
	}
	if utf8.RuneCountInString(name) > MaxDisplayNameLength {
		return "", fmt.Errorf("display name must be at most %d characters", MaxDisplayNameLength)
	}
	if strings.IndexFunc(name, unicode.IsControl) >= 0 {
		return "", errors.New("display name must not contain control characters")
	}
	return name, nil
}

// SetDisplayName saves the caller's own display name, shown in the header instead of the username.
func (s *SickRockServer) SetDisplayName(ctx context.Context, req *connect.Request[sickrockpb.SetDisplayNameRequest]) (*connect.Response[sickrockpb.SetDisplayNameResponse], error) {
	userID, err := s.getUserIDFromContext(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}
	name, err := normalizeDisplayName(req.Msg.GetDisplayName())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	if err := s.repo.SetDisplayName(ctx, userID, name); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("save display name: %w", err))
	}
	return connect.NewResponse(&sickrockpb.SetDisplayNameResponse{DisplayName: name}), nil
}

// displayNameOf returns a user's display name, or "" when unset or it cannot be read (the header
// then shows the username, so a failure here should not fail sign-in).
func (s *SickRockServer) displayNameOf(ctx context.Context, userID int) string {
	name, err := s.repo.DisplayName(ctx, userID)
	if err != nil {
		return ""
	}
	return name
}
