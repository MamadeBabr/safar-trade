package migrations

import (
	"database/sql"
	"embed"
	"fmt"
	"log/slog"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

//go:embed *.sql
var fs embed.FS

// Up automatically applies all pending SQL migrations embedded in the binary.
func Up(databaseURL string, logger *slog.Logger) error {
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return fmt.Errorf("failed to open database connection for migrations: %w", err)
	}
	defer db.Close()

	goose.SetBaseFS(fs)
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("failed to configure goose dialect: %w", err)
	}

	// Silence internal goose stdout output in favor of application structured logs
	goose.SetLogger(goose.NopLogger())

	if err := goose.Up(db, "."); err != nil {
		return fmt.Errorf("failed to run database migrations: %w", err)
	}

	if logger != nil {
		logger.Info("embedded database migrations applied successfully")
	}
	return nil
}

// Down rolls back the most recently applied migration.
func Down(databaseURL string) error {
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return fmt.Errorf("failed to open database connection: %w", err)
	}
	defer db.Close()

	goose.SetBaseFS(fs)
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("failed to configure goose dialect: %w", err)
	}

	return goose.Down(db, ".")
}

// Status prints the status of all migrations to stdout.
func Status(databaseURL string) error {
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return fmt.Errorf("failed to open database connection: %w", err)
	}
	defer db.Close()

	goose.SetBaseFS(fs)
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("failed to configure goose dialect: %w", err)
	}

	return goose.Status(db, ".")
}
