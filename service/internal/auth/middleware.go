package auth

import (
	"context"
	"net/http"
	"net/url"
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

			// Convert Connect request to HTTP request for httpauthshim
			// httpauthshim requires req.URL to be set, so we construct a minimal URL
			// from the procedure path
			procedure := req.Spec().Procedure
			parsedURL, err := url.Parse("http://localhost" + procedure)
			if err != nil {
				log.WithError(err).Error("Failed to parse request URL")
				return nil, connect.NewError(connect.CodeInternal, err)
			}

			httpReq := &http.Request{
				Method: "POST", // Connect uses POST
				URL:    parsedURL,
				Header: make(http.Header),
			}

			// Copy headers from Connect request
			// Connect headers are accessed via Header().Get() or Header().Values()
			// We'll manually copy the Authorization header which is what we need
			if authHeader := req.Header().Get("Authorization"); authHeader != "" {
				httpReq.Header.Set("Authorization", authHeader)
			}
			if sessionToken := req.Header().Get("Session-Token"); sessionToken != "" {
				httpReq.Header.Set("Session-Token", sessionToken)
			}
			httpReq = httpReq.WithContext(ctx)

			// Authenticate using httpauthshim
			authUser := authService.AuthFromHttpReq(httpReq)
			if authUser == nil || authUser.IsGuest() {
				log.Trace("Authentication failed - guest user or nil")
				return nil, connect.NewError(connect.CodeUnauthenticated, nil)
			}

			// Add authenticated user to context
			ctx = context.WithValue(ctx, "authenticated_user", authUser)

			// Also add legacy Claims for backward compatibility
			claims := &Claims{
				Username:  authUser.Username,
				SessionID: authUser.SID,
				RegisteredClaims: jwt.RegisteredClaims{
					ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)), // Default expiration
				},
			}
			ctx = context.WithValue(ctx, "user", claims)

			return next(ctx, req)
		}
	}
}
