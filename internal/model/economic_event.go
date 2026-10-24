package model

import (
	"encoding/json"
	"time"
)

// EconomicEvent represents an economic calendar event stored in our domain.
type EconomicEvent struct {
	ID         int64     `json:"id"`
	Name       string    `json:"name"`
	Country    string    `json:"country"`
	Currency   string    `json:"currency"`
	ImpactName string    `json:"impact_name"`
	Actual     string    `json:"actual"`
	Previous   string    `json:"previous"`
	Forecast   string    `json:"forecast"`
	Revision   string    `json:"revision"`
	Date       string    `json:"date"`
	URL        string    `json:"url"`
	TimeLabel  string    `json:"time_label"`
	Dateline   int64     `json:"dateline"` // Epoch timestamp in milliseconds (e.g. 1729830000000)
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// ForexFactoryResponse mirrors the JSON response structure returned by ForexFactory's apply-settings API.
type ForexFactoryResponse struct {
	Days []RawDay `json:"days"`
}

// RawDay contains the events scheduled for a single day.
type RawDay struct {
	Date   string     `json:"date"`
	Events []RawEvent `json:"events"`
}

// RawEvent maps the exact JSON fields from the Forex Factory calendar endpoint.
// json.Number handles both integer and string representations safely without runtime panics.
type RawEvent struct {
	ID         json.Number `json:"id"`
	Name       string      `json:"name"`
	Country    string      `json:"country"`
	Dateline   json.Number `json:"dateline"`
	Currency   string      `json:"currency"`
	ImpactName string      `json:"impactName"`
	Actual     string      `json:"actual"`
	Previous   string      `json:"previous"`
	Forecast   string      `json:"forecast"`
	Revision   string      `json:"revision"`
	Date       string      `json:"date"`
	URL        string      `json:"url"`
	TimeLabel  string      `json:"timeLabel"`
}

// IngestionResult summarizes the outcome of an ingestion run.
type IngestionResult struct {
	Status      string `json:"status"`
	TotalDays   int    `json:"total_days"`
	TotalEvents int    `json:"total_events"`
	Upserted    int    `json:"upserted"`
	DurationMs  int64  `json:"duration_ms"`
}
