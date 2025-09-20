package auth

import (
	"context"
	"net/http"
	"strings"

	"connectrpc.com/connect"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

func AuthMiddleware(authService *AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip authentication for login, init, and validate-token endpoints
		if c.Request.URL.Path == "/api/sickrock.SickRock/Login" ||
			c.Request.URL.Path == "/api/sickrock.SickRock/Init" ||
			c.Request.URL.Path == "/api/sickrock.SickRock/ValidateToken" {
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
		claims, err := authService.ValidateToken(token)
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

			// Skip authentication for login, init, and validate-token methods
			if req.Spec().Procedure == "/sickrock.SickRock/Login" ||
				req.Spec().Procedure == "/sickrock.SickRock/Init" ||
				req.Spec().Procedure == "/sickrock.SickRock/ValidateToken" {
				log.Trace("Skipping auth for login/init")
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
			claims, err := authService.ValidateToken(token)
			if err != nil {
				log.Tracef("Token validation failed: %v", err)
				return nil, connect.NewError(connect.CodeUnauthenticated, nil)
			}

			ctx = context.WithValue(ctx, "user", claims)
			return next(ctx, req)
		}
	}
}
