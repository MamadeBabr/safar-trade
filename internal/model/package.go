package model

import "time"

// Package represents a subscription plan.
type Package struct {
	ID           int64     `json:"id" db:"id"`
	Name         string    `json:"name" db:"name"`
	Description  string    `json:"description" db:"description"`
	PriceCents   int       `json:"price_cents" db:"price_cents"`
	DurationDays int       `json:"duration_days" db:"duration_days"`
	IsActive     bool      `json:"is_active" db:"is_active"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`
}

// PackageResponse is the public representation of a subscription package.
type PackageResponse struct {
	ID           int64  `json:"id" example:"1" doc:"Package ID"`
	Name         string `json:"name" example:"Pro" doc:"Plan name"`
	Description  string `json:"description" example:"Full real-time streaming" doc:"Plan description"`
	PriceCents   int    `json:"price_cents" example:"2999" doc:"Price in cents USD (e.g. 2999 = $29.99)"`
	DurationDays int    `json:"duration_days" example:"30" doc:"Duration in days"`
	IsActive     bool   `json:"is_active" example:"true" doc:"Whether this package is available for purchase"`
}

// ToResponse converts Package to PackageResponse.
func (p *Package) ToResponse() PackageResponse {
	return PackageResponse{
		ID:           p.ID,
		Name:         p.Name,
		Description:  p.Description,
		PriceCents:   p.PriceCents,
		DurationDays: p.DurationDays,
		IsActive:     p.IsActive,
	}
}
