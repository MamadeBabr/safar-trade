package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"github.com/hossen/safar-trade/internal/model"
	"github.com/hossen/safar-trade/internal/repository"
)

// Common auth sentinel errors.
var (
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrInvalidToken       = errors.New("invalid or expired token")
	ErrUserDisabled       = errors.New("user account is inactive")
)

// JWTClaims represents the payload stored inside a signed JWT access token.
type JWTClaims struct {
	UserID int64  `json:"uid"`
	Email  string `json:"email"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

// AuthService defines operations for authentication and token lifecycle.
type AuthService interface {
	Register(ctx context.Context, req model.RegisterRequest, userAgent, ip string) (*model.AuthResponse, error)
	Login(ctx context.Context, req model.LoginRequest, userAgent, ip string) (*model.AuthResponse, error)
	RefreshToken(ctx context.Context, req model.RefreshTokenRequest, userAgent, ip string) (*model.TokenPair, error)
	Logout(ctx context.Context, refreshToken string, userID int64) error
	ValidateAccessToken(tokenString string) (*JWTClaims, error)
}

type authService struct {
	userRepo        repository.UserRepository
	sessionRepo     repository.SessionRepository
	subService      SubscriptionService
	jwtSecret       []byte
	accessTTL       time.Duration
	refreshTTL      time.Duration
}

var _ AuthService = (*authService)(nil)

// NewAuthService constructs a new AuthService.
func NewAuthService(
	userRepo repository.UserRepository,
	sessionRepo repository.SessionRepository,
	subService SubscriptionService,
	jwtSecret string,
	accessTTL time.Duration,
	refreshTTL time.Duration,
) AuthService {
	return &authService{
		userRepo:    userRepo,
		sessionRepo: sessionRepo,
		subService:  subService,
		jwtSecret:   []byte(jwtSecret),
		accessTTL:   accessTTL,
		refreshTTL:  refreshTTL,
	}
}

func (s *authService) Register(ctx context.Context, req model.RegisterRequest, userAgent, ip string) (*model.AuthResponse, error) {
	email := strings.ToLower(strings.TrimSpace(req.Email))
	name := strings.TrimSpace(req.Name)

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	user := &model.User{
		Email:        email,
		Name:         name,
		PasswordHash: string(hashedPassword),
		Role:         "user",
		IsActive:     true,
	}

	createdUser, err := s.userRepo.Create(ctx, user)
	if err != nil {
		return nil, err
	}

	accessToken, rawRefresh, err := s.issueTokens(ctx, createdUser, userAgent, ip)
	if err != nil {
		return nil, err
	}

	userResp := createdUser.ToResponse()
	return &model.AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: rawRefresh,
		ExpiresIn:    int64(s.accessTTL.Seconds()),
		User:         userResp,
		Subscription: nil,
	}, nil
}

func (s *authService) Login(ctx context.Context, req model.LoginRequest, userAgent, ip string) (*model.AuthResponse, error) {
	email := strings.ToLower(strings.TrimSpace(req.Email))

	user, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}

	if !user.IsActive {
		return nil, ErrUserDisabled
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	accessToken, rawRefresh, err := s.issueTokens(ctx, user, userAgent, ip)
	if err != nil {
		return nil, err
	}

	userResp := user.ToResponse()

	// Optionally include current subscription status
	var subResp *model.SubscriptionResponse
	if s.subService != nil {
		subResp, _ = s.subService.GetActiveSubscription(ctx, user.ID)
	}

	return &model.AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: rawRefresh,
		ExpiresIn:    int64(s.accessTTL.Seconds()),
		User:         userResp,
		Subscription: subResp,
	}, nil
}

func (s *authService) RefreshToken(ctx context.Context, req model.RefreshTokenRequest, userAgent, ip string) (*model.TokenPair, error) {
	rawToken := strings.TrimSpace(req.RefreshToken)
	if rawToken == "" {
		return nil, ErrInvalidToken
	}

	tokenHash := hashToken(rawToken)
	session, err := s.sessionRepo.GetByTokenHash(ctx, tokenHash)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrInvalidToken
		}
		return nil, err
	}

	if !session.IsValid() {
		return nil, ErrInvalidToken
	}

	user, err := s.userRepo.GetByID(ctx, session.UserID)
	if err != nil || !user.IsActive {
		return nil, ErrInvalidToken
	}

	// Revoke the old refresh token (refresh token rotation)
	_ = s.sessionRepo.Revoke(ctx, tokenHash)

	// Issue new token pair
	accessToken, newRawRefresh, err := s.issueTokens(ctx, user, userAgent, ip)
	if err != nil {
		return nil, err
	}

	return &model.TokenPair{
		AccessToken:  accessToken,
		RefreshToken: newRawRefresh,
		ExpiresIn:    int64(s.accessTTL.Seconds()),
	}, nil
}

func (s *authService) Logout(ctx context.Context, refreshToken string, userID int64) error {
	if refreshToken != "" {
		tokenHash := hashToken(refreshToken)
		_ = s.sessionRepo.Revoke(ctx, tokenHash)
	}

	if userID > 0 {
		_ = s.sessionRepo.RevokeAllForUser(ctx, userID)
	}

	return nil
}

func (s *authService) ValidateAccessToken(tokenString string) (*JWTClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return s.jwtSecret, nil
	})

	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidToken, err)
	}

	claims, ok := token.Claims.(*JWTClaims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}

	return claims, nil
}

func (s *authService) issueTokens(ctx context.Context, user *model.User, userAgent, ip string) (string, string, error) {
	// 1. Generate JWT Access Token
	now := time.Now()
	claims := JWTClaims{
		UserID: user.ID,
		Email:  user.Email,
		Role:   user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   fmt.Sprintf("%d", user.ID),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.accessTTL)),
			Issuer:    "safar-trade",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	accessToken, err := token.SignedString(s.jwtSecret)
	if err != nil {
		return "", "", fmt.Errorf("failed to sign access token: %w", err)
	}

	// 2. Generate cryptographically secure random Refresh Token
	randomBytes := make([]byte, 32)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", "", fmt.Errorf("failed to generate random token: %w", err)
	}
	rawRefresh := hex.EncodeToString(randomBytes)
	tokenHash := hashToken(rawRefresh)

	// 3. Persist session token in DB
	session := &model.SessionToken{
		UserID:    user.ID,
		TokenHash: tokenHash,
		UserAgent: userAgent,
		IPAddress: ip,
		ExpiresAt: now.Add(s.refreshTTL),
	}

	if err := s.sessionRepo.Create(ctx, session); err != nil {
		return "", "", fmt.Errorf("failed to persist session token: %w", err)
	}

	return accessToken, rawRefresh, nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
