package model

import "time"

// SessionToken represents a persisted refresh token / active user session.
type SessionToken struct {
	ID        int64      `json:"id" db:"id"`
	UserID    int64      `json:"user_id" db:"user_id"`
	TokenHash string     `json:"-" db:"token_hash"`
	UserAgent string     `json:"user_agent" db:"user_agent"`
	IPAddress string     `json:"ip_address" db:"ip_address"`
	ExpiresAt time.Time  `json:"expires_at" db:"expires_at"`
	RevokedAt *time.Time `json:"revoked_at,omitempty" db:"revoked_at"`
	CreatedAt time.Time  `json:"created_at" db:"created_at"`
}

// IsValid checks if session is neither expired nor revoked.
func (st *SessionToken) IsValid() bool {
	if st.RevokedAt != nil {
		return false
	}
	return time.Now().Before(st.ExpiresAt)
}

// RefreshTokenRequest is the payload to refresh an access token.
type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required" doc:"Opaque session refresh token"`
}

// TokenPair contains newly issued access and refresh tokens.
type TokenPair struct {
	AccessToken  string `json:"access_token" doc:"New short-lived JWT access token"`
	RefreshToken string `json:"refresh_token" doc:"New or rotated session refresh token"`
	ExpiresIn    int64  `json:"expires_in" example:"900" doc:"Access token lifetime in seconds"`
}
