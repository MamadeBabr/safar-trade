package model

import "time"

// User represents a user account in the system.
type User struct {
	ID           int64     `json:"id" db:"id"`
	Email        string    `json:"email" db:"email"`
	Name         string    `json:"name" db:"name"`
	PasswordHash string    `json:"-" db:"password_hash"`
	Role         string    `json:"role" db:"role"`
	IsActive     bool      `json:"is_active" db:"is_active"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`
}

// UserResponse is the safe public representation of a user without sensitive fields.
type UserResponse struct {
	ID        int64     `json:"id" example:"1" doc:"User unique identifier"`
	Email     string    `json:"email" example:"trader@example.com" doc:"User email address"`
	Name      string    `json:"name" example:"Jane Doe" doc:"User display name"`
	Role      string    `json:"role" example:"user" doc:"Access role (user, admin)"`
	CreatedAt time.Time `json:"created_at" doc:"Account registration timestamp"`
}

// ToResponse converts User to UserResponse.
func (u *User) ToResponse() UserResponse {
	return UserResponse{
		ID:        u.ID,
		Email:     u.Email,
		Name:      u.Name,
		Role:      u.Role,
		CreatedAt: u.CreatedAt,
	}
}

// RegisterRequest contains payload for creating a new user account.
type RegisterRequest struct {
	Email    string `json:"email" validate:"required,email" minLength:"5" maxLength:"255" example:"trader@example.com" doc:"User email"`
	Password string `json:"password" validate:"required,min=6,max=72" minLength:"6" maxLength:"72" example:"secret123" doc:"Password (min 6 chars)"`
	Name     string `json:"name" validate:"required,min=2,max=100" minLength:"2" maxLength:"100" example:"Jane Doe" doc:"Display name"`
}

// LoginRequest contains credentials for user authentication.
type LoginRequest struct {
	Email    string `json:"email" validate:"required,email" example:"trader@example.com" doc:"Registered email"`
	Password string `json:"password" validate:"required" example:"secret123" doc:"Password"`
}

// AuthResponse returns issued tokens and user info after successful authentication.
type AuthResponse struct {
	AccessToken  string                `json:"access_token" doc:"Short-lived JWT access token"`
	RefreshToken string                `json:"refresh_token" doc:"Opaque session refresh token"`
	ExpiresIn    int64                 `json:"expires_in" example:"900" doc:"Access token lifetime in seconds"`
	User         UserResponse          `json:"user" doc:"Authenticated user details"`
	Subscription *SubscriptionResponse `json:"subscription,omitempty" doc:"Current active subscription details"`
}
