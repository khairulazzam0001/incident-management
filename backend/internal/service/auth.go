package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"github.com/khairulazzam0001/incident-management/backend/internal/model"
	"github.com/khairulazzam0001/incident-management/backend/internal/repository"
)

// TokenTTL is the JWT lifetime.
const TokenTTL = 8 * time.Hour

// AuthService issues and verifies JWTs (HS256) for Phase 1 authentication.
type AuthService struct {
	repo   *repository.Repository
	secret []byte
}

// NewAuthService creates the auth service. Empty secret is rejected (fail fast).
func NewAuthService(repo *repository.Repository, secret string) (*AuthService, error) {
	if strings.TrimSpace(secret) == "" {
		return nil, fmt.Errorf("jwt secret must not be empty")
	}
	return &AuthService{repo: repo, secret: []byte(secret)}, nil
}

// Login verifies credentials and returns a token plus the public user profile.
func (s *AuthService) Login(ctx context.Context, email, password string) (string, *model.User, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" || password == "" {
		return "", nil, BadRequest("VALIDATION_ERROR", "Email dan password wajib diisi.", nil)
	}
	u, hash, err := s.repo.FindUserByEmail(ctx, email)
	if err != nil {
		return "", nil, Unauthorized("INVALID_CREDENTIALS", "Email atau password salah.")
	}
	if !u.IsActive {
		return "", nil, Unauthorized("INACTIVE_USER", "Akun tidak aktif.")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return "", nil, Unauthorized("INVALID_CREDENTIALS", "Email atau password salah.")
	}
	now := time.Now()
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":   u.ID,
		"email": u.Email,
		"name":  u.Name,
		"role":  u.Role,
		"iat":   now.Unix(),
		"exp":   now.Add(TokenTTL).Unix(),
	}).SignedString(s.secret)
	if err != nil {
		return "", nil, fmt.Errorf("sign token: %w", err)
	}
	return token, u, nil
}

// Parse verifies the token and returns the principal.
func (s *AuthService) Parse(tokenString string) (*model.AuthUser, error) {
	token, err := jwt.Parse(tokenString, func(t *jwt.Token) (any, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return s.secret, nil
	})
	if err != nil || !token.Valid {
		return nil, fmt.Errorf("invalid token: %w", err)
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("invalid claims")
	}
	sub, _ := claims["sub"].(string)
	role, _ := claims["role"].(string)
	if sub == "" || role == "" {
		return nil, fmt.Errorf("incomplete claims")
	}
	email, _ := claims["email"].(string)
	name, _ := claims["name"].(string)
	return &model.AuthUser{ID: sub, Email: email, Name: name, Role: role}, nil
}
