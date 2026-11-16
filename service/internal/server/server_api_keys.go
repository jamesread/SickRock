package server

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"connectrpc.com/connect"
	sickrockpb "github.com/jamesread/SickRock/gen/proto"
	"github.com/jamesread/SickRock/internal/audit"
	"github.com/jamesread/SickRock/internal/iam"
	"github.com/jamesread/SickRock/internal/repo"
	armlayer "github.com/jamesread/armature-iam/layer"
	armpassword "github.com/jamesread/armature-iam/password"
	log "github.com/sirupsen/logrus"
)

// API keys are stored by armature-iam (api_keys); SickRock adds an expiry and a scope
// (iam.APIKeyScope), which limits a key to some of its owner's permissions and to some tables,
// dashboards and workflows. API keys cannot call these procedures (iam.AuthorizeAPIKeyProcedure).

// CreateAPIKey creates a new API key for the authenticated user.
func (s *SickRockServer) CreateAPIKey(ctx context.Context, req *connect.Request[sickrockpb.CreateAPIKeyRequest]) (*connect.Response[sickrockpb.CreateAPIKeyResponse], error) {
	userID, err := s.getUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	name := strings.TrimSpace(req.Msg.GetName())
	if name == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("API key name is required"))
	}
	expiresAt := req.Msg.GetExpiresAt()
	if err := validateAPIKeyExpiry(expiresAt); err != nil {
		return nil, err
	}

	apiKey, keyID, err := s.createStoredAPIKey(ctx, userID, name, req.Msg.GetReadOnly())
	if err != nil {
		return nil, err
	}
	update := iam.APIKeyUpdate{ExpiresAt: &expiresAt, Scope: apiKeyScopeFromProto(req.Msg.GetScope())}
	if err := iam.UpdateAPIKey(ctx, s.repo.DB().DB, keyID, userID, update); err != nil {
		// Never leave behind a key with more access than was asked for.
		if delErr := s.auth.Store.DeleteAPIKey(ctx, keyID, userID); delErr != nil {
			log.WithError(delErr).Error("delete API key after failed scope")
		}
		return nil, apiKeyUpdateError(err)
	}

	return connect.NewResponse(&sickrockpb.CreateAPIKeyResponse{
		Success:  true,
		Message:  "API key created successfully",
		ApiKey:   apiKey,
		ApiKeyId: toInt32(keyID),
	}), nil
}

func (s *SickRockServer) createStoredAPIKey(ctx context.Context, userID int, name string, readOnly bool) (string, int, error) {
	apiKey, err := armpassword.GenerateAPIKey(iam.APIKeyPrefix)
	if err != nil {
		return "", 0, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to generate API key: %w", err))
	}
	keyID, err := s.auth.Store.CreateAPIKey(ctx, userID, name, apiKey, readOnly)
	if err != nil {
		return "", 0, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to create API key: %w", err))
	}
	return apiKey, keyID, nil
}

// GetAPIKeys lists the caller's API keys, and what a key's scope can be chosen from.
func (s *SickRockServer) GetAPIKeys(ctx context.Context, _ *connect.Request[sickrockpb.GetAPIKeysRequest]) (*connect.Response[sickrockpb.GetAPIKeysResponse], error) {
	au := s.authUser(ctx)
	if !isAuthenticated(au) {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	apiKeys, err := s.auth.Store.ListAPIKeysForUser(ctx, au.User.ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to retrieve API keys: %w", err))
	}

	now := time.Now()
	pbAPIKeys := make([]*sickrockpb.APIKey, 0, len(apiKeys))
	for _, apiKey := range apiKeys {
		details, err := iam.LoadAPIKeyDetails(ctx, s.repo.DB().DB, apiKey.ID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to load API key scope: %w", err))
		}
		pbAPIKeys = append(pbAPIKeys, &sickrockpb.APIKey{
			Id:         toInt32(apiKey.ID),
			UserId:     toInt32(apiKey.UserAccountID),
			Name:       apiKey.Name,
			CreatedAt:  parseIAMTimestamp(apiKey.CreatedAt),
			LastUsedAt: parseIAMTimestamp(apiKey.LastUsedAt),
			ExpiresAt:  details.ExpiresAt,
			IsActive:   !details.Expired(now),
			ReadOnly:   apiKey.ReadOnly,
			Scope:      apiKeyScopeToProto(&details.Scope),
		})
	}

	return connect.NewResponse(&sickrockpb.GetAPIKeysResponse{
		ApiKeys:              pbAPIKeys,
		AvailablePermissions: rbacPermissionNames(au),
		AvailableResources:   s.apiKeyResourceOptions(ctx, au),
	}), nil
}

// UpdateAPIKey changes the name, expiry, access level or scope of one of the caller's keys. The
// key's secret stays the same.
func (s *SickRockServer) UpdateAPIKey(ctx context.Context, req *connect.Request[sickrockpb.UpdateAPIKeyRequest]) (*connect.Response[sickrockpb.UpdateAPIKeyResponse], error) {
	userID, err := s.getUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	apiKeyID := int(req.Msg.GetApiKeyId())
	if apiKeyID <= 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("API key ID is required"))
	}

	update := iam.APIKeyUpdate{Scope: apiKeyScopeFromProto(req.Msg.GetScope())}
	if req.Msg.Name != nil {
		name := strings.TrimSpace(req.Msg.GetName())
		if name == "" {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("API key name is required"))
		}
		update.Name = &name
	}
	if req.Msg.ExpiresAt != nil {
		expiresAt := req.Msg.GetExpiresAt()
		if err := validateAPIKeyExpiry(expiresAt); err != nil {
			return nil, err
		}
		update.ExpiresAt = &expiresAt
	}
	update.ReadOnly = req.Msg.ReadOnly

	if err := iam.UpdateAPIKey(ctx, s.repo.DB().DB, apiKeyID, userID, update); err != nil {
		return nil, apiKeyUpdateError(err)
	}
	return connect.NewResponse(&sickrockpb.UpdateAPIKeyResponse{
		Success: true,
		Message: "API key updated successfully",
	}), nil
}

// DeleteAPIKey permanently deletes an API key.
func (s *SickRockServer) DeleteAPIKey(ctx context.Context, req *connect.Request[sickrockpb.DeleteAPIKeyRequest]) (*connect.Response[sickrockpb.DeleteAPIKeyResponse], error) {
	if err := s.deleteAPIKey(ctx, int(req.Msg.GetApiKeyId())); err != nil {
		return nil, err
	}
	return connect.NewResponse(&sickrockpb.DeleteAPIKeyResponse{
		Success: true,
		Message: "API key deleted successfully",
	}), nil
}

// DeactivateAPIKey deletes an API key (armature-iam keys have no inactive state).
func (s *SickRockServer) DeactivateAPIKey(ctx context.Context, req *connect.Request[sickrockpb.DeactivateAPIKeyRequest]) (*connect.Response[sickrockpb.DeactivateAPIKeyResponse], error) {
	if err := s.deleteAPIKey(ctx, int(req.Msg.GetApiKeyId())); err != nil {
		return nil, err
	}
	return connect.NewResponse(&sickrockpb.DeactivateAPIKeyResponse{
		Success: true,
		Message: "API key deleted successfully",
	}), nil
}

func (s *SickRockServer) deleteAPIKey(ctx context.Context, apiKeyID int) error {
	userID, err := s.getUserIDFromContext(ctx)
	if err != nil {
		return err
	}
	if apiKeyID <= 0 {
		return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("API key ID is required"))
	}
	if err := s.auth.Store.DeleteAPIKey(ctx, apiKeyID, userID); err != nil {
		return connect.NewError(connect.CodeInternal, fmt.Errorf("failed to delete API key: %w", err))
	}
	if err := iam.DeleteAPIKeyDetails(ctx, s.repo.DB().DB, apiKeyID); err != nil {
		log.WithError(err).Warn("clean up API key scope")
	}
	return nil
}

func validateAPIKeyExpiry(expiresAt int64) error {
	if expiresAt < 0 {
		return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("expires_at must be a Unix timestamp, or 0 for no expiry"))
	}
	return nil
}

func apiKeyUpdateError(err error) error {
	var invalid *iam.InvalidScopeError
	switch {
	case errors.Is(err, iam.ErrAPIKeyNotFound):
		return connect.NewError(connect.CodeNotFound, err)
	case errors.As(err, &invalid):
		return connect.NewError(connect.CodeInvalidArgument, err)
	}
	return connect.NewError(connect.CodeInternal, fmt.Errorf("failed to update API key: %w", err))
}

// apiKeyScopeFromProto returns nil for an unset scope, which leaves a key's scope unchanged.
func apiKeyScopeFromProto(pb *sickrockpb.APIKeyScope) *iam.APIKeyScope {
	if pb == nil {
		return nil
	}
	scope := &iam.APIKeyScope{
		RestrictPermissions: pb.GetRestrictPermissions(),
		Permissions:         pb.GetPermissions(),
		RestrictResources:   pb.GetRestrictResources(),
	}
	for _, r := range pb.GetResources() {
		scope.Resources = append(scope.Resources, iam.APIKeyResource{Type: r.GetType(), Key: r.GetKey()})
	}
	return scope
}

func apiKeyScopeToProto(scope *iam.APIKeyScope) *sickrockpb.APIKeyScope {
	pb := &sickrockpb.APIKeyScope{
		RestrictPermissions: scope.RestrictPermissions,
		Permissions:         scope.Permissions,
		RestrictResources:   scope.RestrictResources,
		Resources:           make([]*sickrockpb.APIKeyResource, 0, len(scope.Resources)),
	}
	for _, r := range scope.Resources {
		pb.Resources = append(pb.Resources, &sickrockpb.APIKeyResource{Type: r.Type, Key: r.Key})
	}
	return pb
}

// apiKeyResourceOptions lists the tables, dashboards and workflows au can view, which a key's
// scope can be limited to. System tables are governed by permissions, so they are not offered.
func (s *SickRockServer) apiKeyResourceOptions(ctx context.Context, au *armlayer.AuthenticatedUser) []*sickrockpb.APIKeyResourceOption {
	out := []*sickrockpb.APIKeyResourceOption{}
	out = append(out, s.apiKeyTableOptions(ctx, au)...)
	out = append(out, s.apiKeyDashboardOptions(ctx, au)...)
	out = append(out, s.apiKeyWorkflowOptions(ctx, au)...)
	return out
}

func (s *SickRockServer) apiKeyTableOptions(ctx context.Context, au *armlayer.AuthenticatedUser) []*sickrockpb.APIKeyResourceOption {
	configs, err := s.repo.ListTableConfigurationsWithDetails(ctx)
	if err != nil {
		log.WithError(err).Warn("list tables for API key scopes")
		return nil
	}
	out := []*sickrockpb.APIKeyResourceOption{}
	for _, tc := range configs {
		if isPermissionGovernedTable(tc.Name) || !userGrantAllows(au, func(userID int) (bool, error) {
			return s.repo.UserHasTableGrant(ctx, userID, tc.Name, repo.ResourceActionView)
		}) {
			continue
		}
		out = append(out, &sickrockpb.APIKeyResourceOption{Type: iam.ResourceTypeTable, Key: tc.Name, Label: cmp.Or(tc.Title, tc.Name)})
	}
	return sortedOptions(out)
}

func (s *SickRockServer) apiKeyDashboardOptions(ctx context.Context, au *armlayer.AuthenticatedUser) []*sickrockpb.APIKeyResourceOption {
	dashboards, err := s.repo.ListDashboards(ctx)
	if err != nil {
		log.WithError(err).Warn("list dashboards for API key scopes")
		return nil
	}
	out := []*sickrockpb.APIKeyResourceOption{}
	for _, d := range dashboards {
		if userGrantAllows(au, func(userID int) (bool, error) {
			return s.repo.UserHasDashboardGrant(ctx, userID, d.Name, repo.ResourceActionView)
		}) {
			out = append(out, &sickrockpb.APIKeyResourceOption{Type: iam.ResourceTypeDashboard, Key: d.Name, Label: d.Name})
		}
	}
	return sortedOptions(out)
}

func (s *SickRockServer) apiKeyWorkflowOptions(ctx context.Context, au *armlayer.AuthenticatedUser) []*sickrockpb.APIKeyResourceOption {
	workflows, err := s.repo.GetWorkflows(ctx)
	if err != nil {
		log.WithError(err).Warn("list workflows for API key scopes")
		return nil
	}
	out := []*sickrockpb.APIKeyResourceOption{}
	for _, w := range workflows {
		if userGrantAllows(au, func(userID int) (bool, error) {
			return s.repo.UserHasWorkflowGrant(ctx, userID, w.ID, repo.ResourceActionView)
		}) {
			out = append(out, &sickrockpb.APIKeyResourceOption{Type: iam.ResourceTypeWorkflow, Key: strconv.Itoa(w.ID), Label: w.Name})
		}
	}
	return sortedOptions(out)
}

// userGrantAllows reports whether au is a superuser or holds a grant; lookup errors deny.
func userGrantAllows(au *armlayer.AuthenticatedUser, hasGrant func(userID int) (bool, error)) bool {
	if isSuperuser(au) {
		return true
	}
	ok, err := hasGrant(au.User.ID)
	return err == nil && ok
}

// isPermissionGovernedTable reports tables whose access comes from system permissions (audit
// log, read-only exports, system settings) rather than table grants.
func isPermissionGovernedTable(tcName string) bool {
	return tcName == audit.TableConfiguration || tcName == tableReadOnlyExportsTCName || isSystemSettingsTableConfiguration(tcName)
}

func sortedOptions(in []*sickrockpb.APIKeyResourceOption) []*sickrockpb.APIKeyResourceOption {
	slices.SortFunc(in, func(a, b *sickrockpb.APIKeyResourceOption) int {
		return strings.Compare(strings.ToLower(a.GetLabel()), strings.ToLower(b.GetLabel()))
	})
	return in
}
