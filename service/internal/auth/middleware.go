package auth

import (
	"context"
	"net/http"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	log "github.com/sirupsen/logrus"
)

func AuthMiddleware(authService *AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip authentication for login, init, validate-token, and device code endpoints
		if c.Request.URL.Path == "/api/sickrock.SickRock/Login" ||
			c.Request.URL.Path == "/api/sickrock.SickRock/Init" ||
			c.Request.URL.Path == "/api/sickrock.SickRock/ValidateToken" ||
			c.Request.URL.Path == "/api/sickrock.SickRock/GenerateDeviceCode" ||
			c.Request.URL.Path == "/api/sickrock.SickRock/CheckDeviceCode" ||
			c.Request.URL.Path == "/api/sickrock.SickRock/GetDeviceCodeSession" {
			c.Next()
			return
		}

		// Get token from Authorization header
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header required"})
			c.Abort()
			return
		}

		// Extract token from "Bearer <token>"
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid authorization header format"})
			c.Abort()
			return
		}

		token := parts[1]
		claims, err := authService.ValidateToken(c.Request.Context(), token)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			c.Abort()
			return
		}

		// Add user info to context
		c.Set("user", claims)
		c.Next()
	}
}

func ConnectAuthMiddleware(authService *AuthService) connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			log.Tracef("Auth interceptor called for procedure: %s", req.Spec().Procedure)

			// Skip authentication for login, init, validate-token, and device code methods
			if req.Spec().Procedure == "/sickrock.SickRock/Login" ||
				req.Spec().Procedure == "/sickrock.SickRock/Init" ||
				req.Spec().Procedure == "/sickrock.SickRock/ValidateToken" ||
				req.Spec().Procedure == "/sickrock.SickRock/GenerateDeviceCode" ||
				req.Spec().Procedure == "/sickrock.SickRock/CheckDeviceCode" ||
				req.Spec().Procedure == "/sickrock.SickRock/GetDeviceCodeSession" {
				log.Trace("Skipping auth for public endpoints")
				return next(ctx, req)
			}

			// Get token from Authorization header
			authHeader := req.Header().Get("Authorization")
			log.Tracef("Authorization header: %s", authHeader)

			if authHeader == "" {
				log.Trace("No authorization header")
				return nil, connect.NewError(connect.CodeUnauthenticated, nil)
			}

			// Extract token from "Bearer <token>"
			parts := strings.Split(authHeader, " ")
			if len(parts) != 2 || parts[0] != "Bearer" {
				log.Trace("Invalid authorization header format")
				return nil, connect.NewError(connect.CodeUnauthenticated, nil)
			}

			token := parts[1]

			// Check if this is an API key (starts with "sk_")
			if strings.HasPrefix(token, "sk_") {
				log.Trace("Attempting API key authentication")
				apiKey, err := authService.ValidateAPIKey(ctx, token)
				if err != nil {
					log.Tracef("API key validation failed: %v", err)
					return nil, connect.NewError(connect.CodeUnauthenticated, nil)
				}
				if apiKey == nil {
					log.Trace("API key not found or expired")
					return nil, connect.NewError(connect.CodeUnauthenticated, nil)
				}

				// Update last used timestamp
				authService.UpdateAPIKeyLastUsed(ctx, token)

				// Create a claims-like object for API key authentication
				claims := &Claims{
					Username:  "", // We'll get this from the API key
					SessionID: "", // No session for API keys
					RegisteredClaims: jwt.RegisteredClaims{
						ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)), // API keys don't expire via JWT
					},
				}

				// Get the user associated with this API key
				user, err := authService.repo.GetUserByID(ctx, apiKey.UserID)
				if err != nil || user == nil {
					log.Tracef("Failed to get user for API key: %v", err)
					return nil, connect.NewError(connect.CodeUnauthenticated, nil)
				}

				claims.Username = user.Username
				ctx = context.WithValue(ctx, "user", claims)
				ctx = context.WithValue(ctx, "api_key", apiKey)
				return next(ctx, req)
			}

			// Regular JWT token authentication
			claims, err := authService.ValidateToken(ctx, token)
			if err != nil {
				log.Tracef("Token validation failed: %v", err)
				return nil, connect.NewError(connect.CodeUnauthenticated, nil)
			}

			ctx = context.WithValue(ctx, "user", claims)
			return next(ctx, req)
		}
	}
}
