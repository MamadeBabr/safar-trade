package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/hossen/safar-trade/internal/model"
	"github.com/hossen/safar-trade/internal/repository"
	"github.com/hossen/safar-trade/internal/service"
)

type mockUserRepository struct {
	createFn     func(ctx context.Context, user *model.User) (*model.User, error)
	getByIDFn    func(ctx context.Context, id int64) (*model.User, error)
	getByEmailFn func(ctx context.Context, email string) (*model.User, error)
	updateFn     func(ctx context.Context, user *model.User) (*model.User, error)
}

func (m *mockUserRepository) Create(ctx context.Context, user *model.User) (*model.User, error) {
	if m.createFn != nil {
		return m.createFn(ctx, user)
	}
	return nil, errors.New("createFn not implemented")
}

func (m *mockUserRepository) GetByID(ctx context.Context, id int64) (*model.User, error) {
	if m.getByIDFn != nil {
		return m.getByIDFn(ctx, id)
	}
	return nil, errors.New("getByIDFn not implemented")
}

func (m *mockUserRepository) GetByEmail(ctx context.Context, email string) (*model.User, error) {
	if m.getByEmailFn != nil {
		return m.getByEmailFn(ctx, email)
	}
	return nil, errors.New("getByEmailFn not implemented")
}

func (m *mockUserRepository) Update(ctx context.Context, user *model.User) (*model.User, error) {
	if m.updateFn != nil {
		return m.updateFn(ctx, user)
	}
	return nil, errors.New("updateFn not implemented")
}

type mockSessionRepository struct {
	createFn           func(ctx context.Context, session *model.SessionToken) error
	getByTokenHashFn   func(ctx context.Context, tokenHash string) (*model.SessionToken, error)
	revokeFn           func(ctx context.Context, tokenHash string) error
	revokeAllForUserFn func(ctx context.Context, userID int64) error
}

func (m *mockSessionRepository) Create(ctx context.Context, session *model.SessionToken) error {
	if m.createFn != nil {
		return m.createFn(ctx, session)
	}
	return nil
}

func (m *mockSessionRepository) GetByTokenHash(ctx context.Context, tokenHash string) (*model.SessionToken, error) {
	if m.getByTokenHashFn != nil {
		return m.getByTokenHashFn(ctx, tokenHash)
	}
	return nil, errors.New("getByTokenHashFn not implemented")
}

func (m *mockSessionRepository) Revoke(ctx context.Context, tokenHash string) error {
	if m.revokeFn != nil {
		return m.revokeFn(ctx, tokenHash)
	}
	return nil
}

func (m *mockSessionRepository) RevokeAllForUser(ctx context.Context, userID int64) error {
	if m.revokeAllForUserFn != nil {
		return m.revokeAllForUserFn(ctx, userID)
	}
	return nil
}

func TestAuthService_Register_Success(t *testing.T) {
	userRepo := &mockUserRepository{
		createFn: func(ctx context.Context, user *model.User) (*model.User, error) {
			user.ID = 1
			user.CreatedAt = time.Now()
			return user, nil
		},
	}
	sessionRepo := &mockSessionRepository{}

	svc := service.NewAuthService(userRepo, sessionRepo, nil, "super-secret-test-key", 15*time.Minute, 7*24*time.Hour)

	resp, err := svc.Register(context.Background(), model.RegisterRequest{
		Email:    "trader@example.com",
		Password: "password123",
		Name:     "Test Trader",
	}, "test-agent", "127.0.0.1")

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if resp.AccessToken == "" {
		t.Errorf("expected access token to be populated")
	}
	if resp.RefreshToken == "" {
		t.Errorf("expected refresh token to be populated")
	}
	if resp.User.Email != "trader@example.com" {
		t.Errorf("expected email 'trader@example.com', got '%s'", resp.User.Email)
	}

	// Validate access token
	claims, err := svc.ValidateAccessToken(resp.AccessToken)
	if err != nil {
		t.Fatalf("failed to validate generated access token: %v", err)
	}
	if claims.UserID != 1 {
		t.Errorf("expected user id 1, got %d", claims.UserID)
	}
}

func TestAuthService_Login_Success(t *testing.T) {
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("secret123"), bcrypt.DefaultCost)
	userRepo := &mockUserRepository{
		getByEmailFn: func(ctx context.Context, email string) (*model.User, error) {
			return &model.User{
				ID:           42,
				Email:        "trader@example.com",
				Name:         "Trader 42",
				PasswordHash: string(hashedPassword),
				Role:         "user",
				IsActive:     true,
			}, nil
		},
	}
	sessionRepo := &mockSessionRepository{}

	svc := service.NewAuthService(userRepo, sessionRepo, nil, "super-secret-test-key", 15*time.Minute, 7*24*time.Hour)

	resp, err := svc.Login(context.Background(), model.LoginRequest{
		Email:    "trader@example.com",
		Password: "secret123",
	}, "agent", "127.0.0.1")

	if err != nil {
		t.Fatalf("expected login success, got %v", err)
	}

	if resp.User.ID != 42 {
		t.Errorf("expected user ID 42, got %d", resp.User.ID)
	}
}

func TestAuthService_Login_InvalidPassword(t *testing.T) {
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("secret123"), bcrypt.DefaultCost)
	userRepo := &mockUserRepository{
		getByEmailFn: func(ctx context.Context, email string) (*model.User, error) {
			return &model.User{
				ID:           42,
				Email:        "trader@example.com",
				PasswordHash: string(hashedPassword),
				IsActive:     true,
			}, nil
		},
	}
	sessionRepo := &mockSessionRepository{}

	svc := service.NewAuthService(userRepo, sessionRepo, nil, "super-secret-test-key", 15*time.Minute, 7*24*time.Hour)

	_, err := svc.Login(context.Background(), model.LoginRequest{
		Email:    "trader@example.com",
		Password: "wrongpassword",
	}, "agent", "127.0.0.1")

	if !errors.Is(err, service.ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials, got %v", err)
	}
}

func TestAuthService_Login_UserNotFound(t *testing.T) {
	userRepo := &mockUserRepository{
		getByEmailFn: func(ctx context.Context, email string) (*model.User, error) {
			return nil, repository.ErrNotFound
		},
	}
	sessionRepo := &mockSessionRepository{}

	svc := service.NewAuthService(userRepo, sessionRepo, nil, "super-secret-test-key", 15*time.Minute, 7*24*time.Hour)

	_, err := svc.Login(context.Background(), model.LoginRequest{
		Email:    "nonexistent@example.com",
		Password: "secret",
	}, "agent", "127.0.0.1")

	if !errors.Is(err, service.ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials, got %v", err)
	}
}
