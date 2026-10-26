package model

import "time"

const (
	SubscriptionStatusActive   = "active"
	SubscriptionStatusExpired  = "expired"
	SubscriptionStatusCanceled = "canceled"
)

// Subscription represents a user's subscription to a package.
type Subscription struct {
	ID        int64     `json:"id" db:"id"`
	UserID    int64     `json:"user_id" db:"user_id"`
	PackageID int64     `json:"package_id" db:"package_id"`
	Status    string    `json:"status" db:"status"`
	StartsAt  time.Time `json:"starts_at" db:"starts_at"`
	ExpiresAt time.Time `json:"expires_at" db:"expires_at"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`

	// Joined package details if fetched together
	PackageName string `json:"package_name,omitempty" db:"package_name"`
}

// IsActive checks if the subscription is active and has not expired.
func (s *Subscription) IsActive() bool {
	return s.Status == SubscriptionStatusActive && time.Now().Before(s.ExpiresAt)
}

// SubscriptionResponse is the client-facing representation of a subscription.
type SubscriptionResponse struct {
	ID          int64     `json:"id" example:"10" doc:"Subscription ID"`
	UserID      int64     `json:"user_id" example:"1" doc:"Subscriber user ID"`
	PackageID   int64     `json:"package_id" example:"2" doc:"Subscribed package ID"`
	PackageName string    `json:"package_name,omitempty" example:"Pro" doc:"Subscribed package name"`
	Status      string    `json:"status" example:"active" doc:"Subscription status (active, expired, canceled)"`
	IsActive    bool      `json:"is_active" example:"true" doc:"Whether subscription is currently active"`
	StartsAt    time.Time `json:"starts_at" doc:"Subscription start time"`
	ExpiresAt   time.Time `json:"expires_at" doc:"Subscription expiration time"`
}

// ToResponse converts Subscription to SubscriptionResponse.
func (s *Subscription) ToResponse() SubscriptionResponse {
	return SubscriptionResponse{
		ID:          s.ID,
		UserID:      s.UserID,
		PackageID:   s.PackageID,
		PackageName: s.PackageName,
		Status:      s.Status,
		IsActive:    s.IsActive(),
		StartsAt:    s.StartsAt,
		ExpiresAt:   s.ExpiresAt,
	}
}

// SubscribeRequest is the payload to subscribe to a package.
type SubscribeRequest struct {
	PackageID int64 `json:"package_id" validate:"required,min=1" example:"1" doc:"ID of the package to subscribe to"`
}

// CachedSubscription stores the lightweight subscription status in Redis.
type CachedSubscription struct {
	UserID      int64     `json:"user_id"`
	IsActive    bool      `json:"is_active"`
	PackageID   int64     `json:"package_id"`
	PackageName string    `json:"package_name"`
	ExpiresAt   time.Time `json:"expires_at"`
}
