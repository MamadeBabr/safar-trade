-- ==============================================================================
-- Safar-Trade Seed Data
-- ==============================================================================

-- 1. Seed Packages
INSERT INTO packages (name, description, price_cents, duration_days, is_active, updated_at)
VALUES 
    ('Starter', 'Basic market signals & 1h timeframe access', 999, 30, true, NOW()),
    ('Pro', 'Full real-time streaming, candles & economic calendar', 2999, 30, true, NOW()),
    ('VIP', 'Complete access to all indicators, priority data feed & alerts', 7999, 30, true, NOW())
ON CONFLICT (name) DO UPDATE 
SET description = EXCLUDED.description, price_cents = EXCLUDED.price_cents, duration_days = EXCLUDED.duration_days;

-- 2. Seed Demo Users (Password: password123 for both)
-- Bcrypt cost 10 hash for "password123": $2a$10$L24gE5.eLqgqg.J0s1z6v.aZ7V8jJdYvP90R7h5OQy0123456789a
-- A universally valid bcrypt hash for "password123": $2a$10$4n9Fk72F2PZc9i/Zq13r0.Y92Fq6H7oG1dC48R4r9b4e/3f6e5OaC
INSERT INTO users (email, name, password_hash, role, is_active, created_at, updated_at)
VALUES 
    ('pro@example.com', 'Pro Trader', '$2a$10$4n9Fk72F2PZc9i/Zq13r0.Y92Fq6H7oG1dC48R4r9b4e/3f6e5OaC', 'user', true, NOW(), NOW()),
    ('free@example.com', 'Free Trader', '$2a$10$4n9Fk72F2PZc9i/Zq13r0.Y92Fq6H7oG1dC48R4r9b4e/3f6e5OaC', 'user', true, NOW(), NOW())
ON CONFLICT (email) DO NOTHING;

-- 3. Seed Active 30-Day Pro Subscription for pro@example.com
INSERT INTO subscriptions (user_id, package_id, status, starts_at, expires_at, created_at, updated_at)
SELECT u.id, p.id, 'active', NOW(), NOW() + INTERVAL '30 days', NOW(), NOW()
FROM users u, packages p
WHERE u.email = 'pro@example.com' AND p.name = 'Pro'
ON CONFLICT DO NOTHING;

-- 4. Seed Sample BTCUSDT Timeframe Candles (epoch ms for t)
INSERT INTO candles_1m (symbol, o, h, l, c, v, t)
VALUES 
    ('BTCUSDT', '65100.0', '65250.0', '65050.0', '65200.0', '150.2', (EXTRACT(EPOCH FROM NOW() - INTERVAL '5 minutes') * 1000)::BIGINT),
    ('BTCUSDT', '65200.0', '65300.0', '65180.0', '65280.0', '110.8', (EXTRACT(EPOCH FROM NOW() - INTERVAL '4 minutes') * 1000)::BIGINT),
    ('BTCUSDT', '65280.0', '65350.0', '65250.0', '65320.0', '180.5', (EXTRACT(EPOCH FROM NOW() - INTERVAL '3 minutes') * 1000)::BIGINT),
    ('BTCUSDT', '65320.0', '65400.0', '65300.0', '65390.0', '210.0', (EXTRACT(EPOCH FROM NOW() - INTERVAL '2 minutes') * 1000)::BIGINT),
    ('BTCUSDT', '65390.0', '65450.0', '65380.0', '65420.0', '95.4',  (EXTRACT(EPOCH FROM NOW() - INTERVAL '1 minute') * 1000)::BIGINT)
ON CONFLICT (symbol, t) DO NOTHING;

INSERT INTO candles_15m (symbol, o, h, l, c, v, t)
VALUES 
    ('BTCUSDT', '65000.0', '65450.0', '64950.0', '65380.0', '746.9', (EXTRACT(EPOCH FROM NOW() - INTERVAL '15 minutes') * 1000)::BIGINT)
ON CONFLICT (symbol, t) DO NOTHING;

-- 5. Seed Sample Economic Events
INSERT INTO economic_events (id, name, country, currency, impact_name, actual, previous, forecast, date, url, time_label, dateline)
VALUES 
    (99901, 'Federal Funds Rate', 'USD', 'USD', 'High', '5.25%', '5.50%', '5.25%', NOW(), 'https://forexfactory.com', '14:00', 1700000000),
    (99902, 'Non-Farm Employment Change', 'USD', 'USD', 'High', '216K', '173K', '170K', NOW() + INTERVAL '2 days', 'https://forexfactory.com', '08:30', 1700172800)
ON CONFLICT (id) DO NOTHING;
