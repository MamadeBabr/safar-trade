package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"github.com/hossen/safar-trade/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer pool.Close()

	fmt.Println("Seeding database...")

	// 1. Seed Packages
	packages := []struct {
		name        string
		description string
		priceCents  int
		days        int
	}{
		{"Starter", "Basic market signals & 1h timeframe access", 999, 30},
		{"Pro", "Full real-time streaming, candles & economic calendar", 2999, 30},
		{"VIP", "Complete access to all indicators, priority data feed & alerts", 7999, 30},
	}

	for _, p := range packages {
		_, err := pool.Exec(ctx, `
			INSERT INTO packages (name, description, price_cents, duration_days, is_active, updated_at)
			VALUES ($1, $2, $3, $4, true, NOW())
			ON CONFLICT (name) DO UPDATE 
			SET description = EXCLUDED.description, price_cents = EXCLUDED.price_cents, duration_days = EXCLUDED.duration_days;
		`, p.name, p.description, p.priceCents, p.days)
		if err != nil {
			log.Fatalf("failed to seed package %s: %v", p.name, err)
		}
	}
	fmt.Println("  [OK] Packages seeded")

	// 2. Hash default password
	password := "password123"
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		log.Fatalf("failed to hash password: %v", err)
	}

	// 3. Seed Users
	var proUserID, freeUserID int64

	// User with active subscription: pro@example.com
	err = pool.QueryRow(ctx, `
		INSERT INTO users (email, name, password_hash, role, is_active, created_at, updated_at)
		VALUES ('pro@example.com', 'Pro Trader', $1, 'user', true, NOW(), NOW())
		ON CONFLICT (email) DO UPDATE 
		SET password_hash = EXCLUDED.password_hash, name = EXCLUDED.name
		RETURNING id;
	`, string(hashedPassword)).Scan(&proUserID)
	if err != nil {
		log.Fatalf("failed to seed pro user: %v", err)
	}

	// User without subscription: free@example.com
	err = pool.QueryRow(ctx, `
		INSERT INTO users (email, name, password_hash, role, is_active, created_at, updated_at)
		VALUES ('free@example.com', 'Free Trader', $1, 'user', true, NOW(), NOW())
		ON CONFLICT (email) DO UPDATE 
		SET password_hash = EXCLUDED.password_hash, name = EXCLUDED.name
		RETURNING id;
	`, string(hashedPassword)).Scan(&freeUserID)
	if err != nil {
		log.Fatalf("failed to seed free user: %v", err)
	}
	fmt.Printf("  [OK] Users seeded (Password: %s)\n", password)
	fmt.Printf("       - pro@example.com  (ID: %d, Has Active Subscription)\n", proUserID)
	fmt.Printf("       - free@example.com (ID: %d, No Subscription)\n", freeUserID)

	// 4. Seed Active Subscription for pro@example.com (Pro package)
	var proPackageID int64
	err = pool.QueryRow(ctx, `SELECT id FROM packages WHERE name = 'Pro'`).Scan(&proPackageID)
	if err != nil {
		log.Fatalf("failed to find Pro package: %v", err)
	}

	_, err = pool.Exec(ctx, `UPDATE subscriptions SET status = 'canceled' WHERE user_id = $1`, proUserID)
	if err != nil {
		log.Fatalf("failed to cancel previous subscriptions: %v", err)
	}

	_, err = pool.Exec(ctx, `
		INSERT INTO subscriptions (user_id, package_id, status, starts_at, expires_at, created_at, updated_at)
		VALUES ($1, $2, 'active', NOW(), NOW() + INTERVAL '30 days', NOW(), NOW())
	`, proUserID, proPackageID)
	if err != nil {
		log.Fatalf("failed to seed subscription: %v", err)
	}
	fmt.Println("  [OK] Active 30-day Pro subscription seeded for pro@example.com")

	// 5. Seed Sample Timeframe Candles (for BTCUSDT in candles_1m and candles_15m)
	now := time.Now().Truncate(time.Minute)
	for i := 20; i >= 0; i-- {
		t := now.Add(-time.Duration(i) * time.Minute).UnixMilli()
		_, _ = pool.Exec(ctx, `
			INSERT INTO candles_1m (symbol, o, h, l, c, v, t)
			VALUES ('BTCUSDT', '65000.00', '65200.00', '64950.00', '65150.00', '125.5', $1)
			ON CONFLICT (symbol, t) DO NOTHING;
		`, t)
	}
	for i := 10; i >= 0; i-- {
		t := now.Add(-time.Duration(i*15) * time.Minute).UnixMilli()
		_, _ = pool.Exec(ctx, `
			INSERT INTO candles_15m (symbol, o, h, l, c, v, t)
			VALUES ('BTCUSDT', '64800.00', '65350.00', '64750.00', '65280.00', '520.4', $1)
			ON CONFLICT (symbol, t) DO NOTHING;
		`, t)
	}
	fmt.Println("  [OK] Sample BTCUSDT candles seeded (1m & 15m)")

	// 6. Seed Sample Economic Events
	_, _ = pool.Exec(ctx, `
		INSERT INTO economic_events (id, name, country, currency, impact_name, actual, previous, forecast, date, url, time_label, dateline)
		VALUES 
			(99901, 'Federal Funds Rate', 'USD', 'USD', 'High', '5.25%', '5.50%', '5.25%', NOW(), 'https://forexfactory.com', '14:00', 1700000000),
			(99902, 'CPI m/m', 'USD', 'USD', 'High', '0.2%', '0.1%', '0.2%', NOW() + INTERVAL '1 day', 'https://forexfactory.com', '08:30', 1700086400)
		ON CONFLICT (id) DO NOTHING;
	`)
	fmt.Println("  [OK] Sample Economic Events seeded")

	fmt.Println("\nDatabase seeding completed successfully!")
}
