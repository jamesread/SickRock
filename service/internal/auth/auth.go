package auth

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type AuthService struct {
	jwtSecret []byte
	users     map[string]User
}

type User struct {
	Username string
	Password string // hashed
}

type Claims struct {
	Username string `json:"username"`
	jwt.RegisteredClaims
}

func NewAuthService() *AuthService {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		// Use a fixed secret for development
		secret = "supersecretkey"
	}

	// Create default admin user
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("admin"), bcrypt.DefaultCost)
	users := map[string]User{
		"admin": {
			Username: "admin",
			Password: string(hashedPassword),
		},
	}

	return &AuthService{
		jwtSecret: []byte(secret),
		users:     users,
	}
}

func (a *AuthService) Login(username, password string) (string, time.Time, error) {
	user, exists := a.users[username]
	if !exists {
		return "", time.Time{}, fmt.Errorf("invalid credentials")
	}

	err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password))
	if err != nil {
		return "", time.Time{}, fmt.Errorf("invalid credentials")
	}

	expirationTime := time.Now().Add(24 * time.Hour)
	claims := &Claims{
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(a.jwtSecret)
	if err != nil {
		return "", time.Time{}, err
	}

	return tokenString, expirationTime, nil
}

func (a *AuthService) ValidateToken(tokenString string) (*Claims, error) {
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

	return claims, nil
}

func (a *AuthService) GetUserFromContext(ctx context.Context) (string, error) {
	claims, ok := ctx.Value("user").(*Claims)
	if !ok {
		return "", fmt.Errorf("no user in context")
	}
	return claims.Username, nil
}
