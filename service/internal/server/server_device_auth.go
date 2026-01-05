package server

import (
	"context"
	"fmt"
	"os"
	"time"

	"connectrpc.com/connect"
	"github.com/golang-jwt/jwt/v5"

	sickrockpb "github.com/jamesread/SickRock/gen/proto"
	"github.com/jamesread/SickRock/internal/auth"
)

func (s *SickRockServer) GenerateDeviceCode(ctx context.Context, req *connect.Request[sickrockpb.GenerateDeviceCodeRequest]) (*connect.Response[sickrockpb.GenerateDeviceCodeResponse], error) {
	// Generate a 4-digit device code
	code, err := s.repo.GenerateDeviceCode()
	if err != nil {
		return connect.NewResponse(&sickrockpb.GenerateDeviceCodeResponse{
			Code:      "",
			ExpiresAt: 0,
		}), fmt.Errorf("failed to generate device code: %w", err)
	}

	// Set expiration to 10 minutes from now
	expiresAt := time.Now().Add(10 * time.Minute)

	// Store the device code in the database
	err = s.repo.CreateDeviceCode(ctx, code, expiresAt)
	if err != nil {
		return connect.NewResponse(&sickrockpb.GenerateDeviceCodeResponse{
			Code:      "",
			ExpiresAt: 0,
		}), fmt.Errorf("failed to store device code: %w", err)
	}

	return connect.NewResponse(&sickrockpb.GenerateDeviceCodeResponse{
		Code:      code,
		ExpiresAt: expiresAt.Unix(),
	}), nil
}

func (s *SickRockServer) ClaimDeviceCode(ctx context.Context, req *connect.Request[sickrockpb.ClaimDeviceCodeRequest]) (*connect.Response[sickrockpb.ClaimDeviceCodeResponse], error) {
	code := req.Msg.GetCode()
	if code == "" {
		return connect.NewResponse(&sickrockpb.ClaimDeviceCodeResponse{
			Success:   false,
			Message:   "Device code is required",
			Token:     "",
			ExpiresAt: 0,
		}), nil
	}

	// Get the current user from context (must be authenticated to claim a device code)
	username, err := s.authService.GetUserFromContext(ctx)
	if err != nil {
		return connect.NewResponse(&sickrockpb.ClaimDeviceCodeResponse{
			Success:   false,
			Message:   "Authentication required to claim device code",
			Token:     "",
			ExpiresAt: 0,
		}), nil
	}

	// Claim the device code
	err = s.repo.ClaimDeviceCode(ctx, code, username)
	if err != nil {
		return connect.NewResponse(&sickrockpb.ClaimDeviceCodeResponse{
			Success:   false,
			Message:   "Device code not found, expired, or already claimed",
			Token:     "",
			ExpiresAt: 0,
		}), nil
	}

	// Get the device code to check its expiration
	deviceCode, err := s.repo.GetDeviceCode(ctx, code)
	if err != nil {
		return connect.NewResponse(&sickrockpb.ClaimDeviceCodeResponse{
			Success:   false,
			Message:   "Failed to retrieve device code",
			Token:     "",
			ExpiresAt: 0,
		}), nil
	}

	if deviceCode == nil {
		return connect.NewResponse(&sickrockpb.ClaimDeviceCodeResponse{
			Success:   false,
			Message:   "Device code not found",
			Token:     "",
			ExpiresAt: 0,
		}), nil
	}

	// Create a new session for the device code claimer
	// Extract client information from the request
	userAgent := req.Header().Get("User-Agent")
	ipAddress := getClientIP(req)

	// Create session with 10 year expiration
	expirationTime := time.Now().Add(10 * 365 * 24 * time.Hour) // 10 years

	// Generate session ID using the repository method
	sessionID, err := s.repo.GenerateDeviceCode() // Reuse the same random generation logic
	if err != nil {
		return connect.NewResponse(&sickrockpb.ClaimDeviceCodeResponse{
			Success:   false,
			Message:   "Failed to generate session",
			Token:     "",
			ExpiresAt: 0,
		}), nil
	}

	// Create session in database
	err = s.repo.CreateSession(ctx, sessionID, username, expirationTime, userAgent, ipAddress)
	if err != nil {
		return connect.NewResponse(&sickrockpb.ClaimDeviceCodeResponse{
			Success:   false,
			Message:   "Failed to create session",
			Token:     "",
			ExpiresAt: 0,
		}), nil
	}

	// Create JWT token manually
	// Get JWT secret from environment or use default
	jwtSecret := "supersecretkey" // This should match the auth service default
	if secret := os.Getenv("JWT_SECRET"); secret != "" {
		jwtSecret = secret
	}

	claims := &auth.Claims{
		Username:  username,
		SessionID: sessionID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(jwtSecret))
	if err != nil {
		// Clean up session if token creation fails
		s.repo.DeleteSession(ctx, sessionID)
		return connect.NewResponse(&sickrockpb.ClaimDeviceCodeResponse{
			Success:   false,
			Message:   "Failed to create token",
			Token:     "",
			ExpiresAt: 0,
		}), nil
	}

	return connect.NewResponse(&sickrockpb.ClaimDeviceCodeResponse{
		Success:   true,
		Message:   "Device code claimed successfully",
		Token:     tokenString,
		ExpiresAt: expirationTime.Unix(),
	}), nil
}

func (s *SickRockServer) CheckDeviceCode(ctx context.Context, req *connect.Request[sickrockpb.CheckDeviceCodeRequest]) (*connect.Response[sickrockpb.CheckDeviceCodeResponse], error) {
	code := req.Msg.GetCode()
	if code == "" {
		return connect.NewResponse(&sickrockpb.CheckDeviceCodeResponse{
			Valid:     false,
			Claimed:   false,
			ExpiresAt: 0,
		}), nil
	}

	deviceCode, err := s.repo.GetDeviceCode(ctx, code)
	if err != nil {
		return connect.NewResponse(&sickrockpb.CheckDeviceCodeResponse{
			Valid:     false,
			Claimed:   false,
			ExpiresAt: 0,
		}), nil
	}

	if deviceCode == nil {
		return connect.NewResponse(&sickrockpb.CheckDeviceCodeResponse{
			Valid:     false,
			Claimed:   false,
			ExpiresAt: 0,
		}), nil
	}

	claimed := deviceCode.ClaimedBy.Valid && deviceCode.ClaimedBy.String != ""

	response := &sickrockpb.CheckDeviceCodeResponse{
		Valid:     true,
		Claimed:   claimed,
		ExpiresAt: deviceCode.ExpiresAt.Unix(),
		Token:     "",
		Username:  "",
	}

	// If claimed, get the session information
	if claimed {
		username := deviceCode.ClaimedBy.String
		session, err := s.repo.GetSessionByUsername(ctx, username)
		if err == nil && session != nil {
			// Create JWT token for the session
			jwtSecret := "supersecretkey" // This should match the auth service default
			if secret := os.Getenv("JWT_SECRET"); secret != "" {
				jwtSecret = secret
			}

			claims := &auth.Claims{
				Username:  username,
				SessionID: session.SessionID,
				RegisteredClaims: jwt.RegisteredClaims{
					ExpiresAt: jwt.NewNumericDate(session.ExpiresAt),
					IssuedAt:  jwt.NewNumericDate(time.Now()),
				},
			}

			token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
			if tokenString, err := token.SignedString([]byte(jwtSecret)); err == nil {
				response.Token = tokenString
				response.Username = username
			}
		}
	}

	return connect.NewResponse(response), nil
}

func (s *SickRockServer) GetDeviceCodeSession(ctx context.Context, req *connect.Request[sickrockpb.GetDeviceCodeSessionRequest]) (*connect.Response[sickrockpb.GetDeviceCodeSessionResponse], error) {
	code := req.Msg.GetCode()
	if code == "" {
		return connect.NewResponse(&sickrockpb.GetDeviceCodeSessionResponse{
			Success:   false,
			Message:   "Device code is required",
			Token:     "",
			ExpiresAt: 0,
			Username:  "",
		}), nil
	}

	// Get the device code
	deviceCode, err := s.repo.GetDeviceCode(ctx, code)
	if err != nil {
		return connect.NewResponse(&sickrockpb.GetDeviceCodeSessionResponse{
			Success:   false,
			Message:   "Failed to retrieve device code",
			Token:     "",
			ExpiresAt: 0,
			Username:  "",
		}), nil
	}

	if deviceCode == nil {
		return connect.NewResponse(&sickrockpb.GetDeviceCodeSessionResponse{
			Success:   false,
			Message:   "Device code not found or expired",
			Token:     "",
			ExpiresAt: 0,
			Username:  "",
		}), nil
	}

	// Check if the device code is claimed
	if !deviceCode.ClaimedBy.Valid || deviceCode.ClaimedBy.String == "" {
		return connect.NewResponse(&sickrockpb.GetDeviceCodeSessionResponse{
			Success:   false,
			Message:   "Device code not yet claimed",
			Token:     "",
			ExpiresAt: 0,
			Username:  "",
		}), nil
	}

	// Get the session for the user who claimed the device code
	username := deviceCode.ClaimedBy.String
	session, err := s.repo.GetSessionByUsername(ctx, username)
	if err != nil {
		return connect.NewResponse(&sickrockpb.GetDeviceCodeSessionResponse{
			Success:   false,
			Message:   "Failed to retrieve session",
			Token:     "",
			ExpiresAt: 0,
			Username:  "",
		}), nil
	}

	if session == nil {
		return connect.NewResponse(&sickrockpb.GetDeviceCodeSessionResponse{
			Success:   false,
			Message:   "Session not found",
			Token:     "",
			ExpiresAt: 0,
			Username:  "",
		}), nil
	}

	// Create JWT token for the session
	jwtSecret := "supersecretkey" // This should match the auth service default
	if secret := os.Getenv("JWT_SECRET"); secret != "" {
		jwtSecret = secret
	}

	claims := &auth.Claims{
		Username:  username,
		SessionID: session.SessionID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(session.ExpiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(jwtSecret))
	if err != nil {
		return connect.NewResponse(&sickrockpb.GetDeviceCodeSessionResponse{
			Success:   false,
			Message:   "Failed to create token",
			Token:     "",
			ExpiresAt: 0,
			Username:  "",
		}), nil
	}

	return connect.NewResponse(&sickrockpb.GetDeviceCodeSessionResponse{
		Success:   true,
		Message:   "Session retrieved successfully",
		Token:     tokenString,
		ExpiresAt: session.ExpiresAt.Unix(),
		Username:  username,
	}), nil
}
