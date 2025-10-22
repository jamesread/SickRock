package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jamesread/SickRock/internal/repo"
	"golang.org/x/crypto/bcrypt"
)

type AuthService struct {
	jwtSecret []byte
	repo      *repo.Repository
}

type User struct {
	Username string
	Password string // hashed
}

type Claims struct {
	Username  string `json:"username"`
	SessionID string `json:"session_id"`
	jwt.RegisteredClaims
}

func NewAuthService(repository *repo.Repository) *AuthService {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		// Use a fixed secret for development
		secret = "supersecretkey"
	}

	return &AuthService{
		jwtSecret: []byte(secret),
		repo:      repository,
	}
}

func (a *AuthService) Login(ctx context.Context, username, password, userAgent, ipAddress string) (string, time.Time, error) {
	user, err := a.repo.GetUserByUsername(ctx, username)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("database error: %w", err)
	}
	if user == nil {
		return "", time.Time{}, fmt.Errorf("invalid credentials")
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password))
	if err != nil {
		return "", time.Time{}, fmt.Errorf("invalid credentials")
	}

	expirationTime := time.Now().Add(10 * 365 * 24 * time.Hour) // 10 years

	// Generate a unique session ID
	sessionID, err := a.generateSessionID()
	if err != nil {
		return "", time.Time{}, fmt.Errorf("failed to generate session ID: %w", err)
	}

	// Create session in database
	err = a.repo.CreateSession(ctx, sessionID, username, expirationTime, userAgent, ipAddress)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("failed to create session: %w", err)
	}

	// Create JWT token with session ID
	claims := &Claims{
		Username:  username,
		SessionID: sessionID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(a.jwtSecret)
	if err != nil {
		// Clean up session if token creation fails
		a.repo.DeleteSession(ctx, sessionID)
		return "", time.Time{}, err
	}

	return tokenString, expirationTime, nil
}

func (a *AuthService) ValidateToken(ctx context.Context, tokenString string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		return a.jwtSecret, nil
	})

	if err != nil {
		return nil, err
	}

	if !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	// Validate session exists in database
	if claims.SessionID != "" {
		session, err := a.repo.GetSession(ctx, claims.SessionID)
		if err != nil {
			return nil, fmt.Errorf("session validation error: %w", err)
		}
		if session == nil {
			return nil, fmt.Errorf("session not found or expired")
		}

		// Update last accessed time
		err = a.repo.UpdateSessionLastAccessed(ctx, claims.SessionID)
		if err != nil {
			// Log error but don't fail validation
			fmt.Printf("Warning: failed to update session last accessed: %v\n", err)
		}
	}

	return claims, nil
}

func (a *AuthService) GetUserFromContext(ctx context.Context) (string, error) {
	claims, ok := ctx.Value("user").(*Claims)
	if !ok {
		return "", fmt.Errorf("no user in context")
	}
	return claims.Username, nil
}

func (a *AuthService) Logout(ctx context.Context, tokenString string) error {
	claims, err := a.ValidateToken(ctx, tokenString)
	if err != nil {
		return err
	}

	if claims.SessionID != "" {
		return a.repo.DeleteSession(ctx, claims.SessionID)
	}

	return nil
}

func (a *AuthService) LogoutAllUserSessions(ctx context.Context, username string) error {
	return a.repo.DeleteUserSessions(ctx, username)
}

func (a *AuthService) generateSessionID() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// API Key validation methods

// ValidateAPIKey validates an API key and returns the associated API key record
func (a *AuthService) ValidateAPIKey(ctx context.Context, apiKey string) (*repo.APIKey, error) {
	// Hash the provided API key
	keyHash, err := a.hashAPIKey(apiKey)
	if err != nil {
		return nil, err
	}

	// Look up the API key by hash
	return a.repo.GetAPIKeyByHash(ctx, keyHash)
}

// UpdateAPIKeyLastUsed updates the last used timestamp for an API key
func (a *AuthService) UpdateAPIKeyLastUsed(ctx context.Context, apiKey string) error {
	keyHash, err := a.hashAPIKey(apiKey)
	if err != nil {
		return err
	}

	return a.repo.UpdateAPIKeyLastUsed(ctx, keyHash)
}

// hashAPIKey hashes an API key using SHA256
func (a *AuthService) hashAPIKey(apiKey string) (string, error) {
	hash := sha256.Sum256([]byte(apiKey))
	return hex.EncodeToString(hash[:]), nil
}
