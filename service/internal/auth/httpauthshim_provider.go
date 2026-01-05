package auth

import (
	"context"
	"strings"

	types "github.com/jamesread/httpauthshim/authpublic"
	"github.com/jamesread/SickRock/internal/repo"
	log "github.com/sirupsen/logrus"
)

// DatabaseAuthProvider provides authentication using database-backed JWT tokens and sessions
type DatabaseAuthProvider struct {
	repo        *repo.Repository
	jwtSecret   []byte
	authService *AuthService
}

// NewDatabaseAuthProvider creates a new database authentication provider
func NewDatabaseAuthProvider(repository *repo.Repository, jwtSecret []byte, authService *AuthService) *DatabaseAuthProvider {
	return &DatabaseAuthProvider{
		repo:        repository,
		jwtSecret:   jwtSecret,
		authService: authService,
	}
}

// CheckUserFromDatabaseAuth checks authentication using database-backed JWT tokens
// This is the provider function that httpauthshim will call
func (p *DatabaseAuthProvider) CheckUserFromDatabaseAuth(authCtx *types.AuthCheckingContext) *types.AuthenticatedUser {
	req := authCtx.Request
	if req == nil {
		return nil
	}

	// Get token from Authorization header
	authHeader := req.Header.Get("Authorization")
	if authHeader == "" {
		// Also check Session-Token header as fallback
		authHeader = req.Header.Get("Session-Token")
		if authHeader == "" {
			return nil
		}
		// If Session-Token header is used, treat it as a Bearer token
		authHeader = "Bearer " + authHeader
	}

	// Extract token from "Bearer <token>"
	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || parts[0] != "Bearer" {
		return nil
	}

	token := parts[1]

	// Check if this is an API key (starts with "sk_")
	if strings.HasPrefix(token, "sk_") {
		return p.checkAPIKey(authCtx.Context, token)
	}

	// Validate JWT token
	claims, err := p.authService.ValidateToken(authCtx.Context, token)
	if err != nil {
		log.WithError(err).Trace("JWT token validation failed")
		return nil
	}

	if claims == nil {
		return nil
	}

	// Get user from database to ensure they still exist
	user, err := p.repo.GetUserByUsername(authCtx.Context, claims.Username)
	if err != nil || user == nil {
		log.WithError(err).WithField("username", claims.Username).Trace("User not found in database")
		return nil
	}

	// Return authenticated user
	return &types.AuthenticatedUser{
		Username:      claims.Username,
		UsergroupLine: "", // We don't use usergroups in SickRock currently
		Provider:      "database",
		SID:           claims.SessionID,
	}
}

// checkAPIKey validates an API key and returns an authenticated user
func (p *DatabaseAuthProvider) checkAPIKey(ctx context.Context, apiKey string) *types.AuthenticatedUser {
	apiKeyRecord, err := p.authService.ValidateAPIKey(ctx, apiKey)
	if err != nil {
		log.WithError(err).Trace("API key validation failed")
		return nil
	}
	if apiKeyRecord == nil {
		return nil
	}

	// Update last used timestamp
	p.authService.UpdateAPIKeyLastUsed(ctx, apiKey)

	// Get the user associated with this API key
	user, err := p.repo.GetUserByID(ctx, apiKeyRecord.UserID)
	if err != nil || user == nil {
		log.WithError(err).Trace("Failed to get user for API key")
		return nil
	}

	return &types.AuthenticatedUser{
		Username:      user.Username,
		UsergroupLine: "",
		Provider:      "api_key",
		SID:           "",
	}
}
