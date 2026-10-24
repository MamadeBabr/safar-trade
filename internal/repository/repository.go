// Package repository provides the data access layer abstraction and implementations.
//
// Node comparison:
// In Node.js, you might define custom Error classes (e.g. `class NotFoundError extends Error`).
// In Go, sentinel errors (`var ErrNotFound = errors.New(...)`) or custom error types are idiomatic.
// Callers check errors using `errors.Is(err, repository.ErrNotFound)` rather than `instanceof`.
package repository

import "errors"

// Common sentinel errors returned across repositories.
var (
	// ErrNotFound is returned when a requested record does not exist in the database.
	ErrNotFound = errors.New("resource not found")

	// ErrConflict is returned when an insert/update violates a unique constraint.
	ErrConflict = errors.New("resource conflict or unique violation")
)
