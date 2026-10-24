// Package config provides typed application configuration loaded from environment variables.
//
// Node comparison:
// In Node.js, you typically use `process.env` combined with libraries like `dotenv` and `zod`
// (e.g. `z.object({ PORT: z.coerce.number().default(8080) }).parse(process.env)`).
// In idiomatic Go without adding heavy dependencies, we use a strongly-typed struct,
// read from `os.Getenv`, and apply defaults and type parsing via the standard library.
package config

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all configuration parameters for the application.
// Struct fields are exported (capitalized) so other packages can read them.
type Config struct {
	AppEnv               string        // "development", "production", "test"
	Port                 int           // HTTP server listening port
	DatabaseURL          string        // PostgreSQL DSN connection string
	LogLevel             string        // "debug", "info", "warn", "error"
	CORSOrigin           string        // Allowed CORS origins (e.g. "*" or specific domain)
	ServerTimeout        time.Duration // HTTP read/write timeout
	ShutdownPeriod       time.Duration // Graceful shutdown deadline
	EnableScheduler      bool          // Background ingestion worker toggle
	IngestionInterval    time.Duration // Periodic ingestion interval (e.g. "1h")
	ForexFactoryJSONPath string        // Path to output forexfactory.json (empty to disable)
}

// Load reads configuration from environment variables, falling back to sensible defaults.
// It also checks for an optional .env file in the working directory (standard library implementation).
func Load() (*Config, error) {
	// Attempt to load .env file if it exists (does not overwrite existing environment variables).
	_ = loadDotEnv(".env")

	cfg := &Config{
		AppEnv:               getEnv("APP_ENV", "development"),
		Port:                 getEnvAsInt("PORT", 8080),
		DatabaseURL:          getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/safar_trade?sslmode=disable"),
		LogLevel:             getEnv("LOG_LEVEL", "info"),
		CORSOrigin:           getEnv("CORS_ORIGIN", "*"),
		ServerTimeout:        getEnvAsDuration("SERVER_TIMEOUT", 15*time.Second),
		ShutdownPeriod:       getEnvAsDuration("SHUTDOWN_PERIOD", 10*time.Second),
		EnableScheduler:      getEnvAsBool("ENABLE_INGESTION_SCHEDULER", true),
		IngestionInterval:    getEnvAsDuration("INGESTION_INTERVAL", 1*time.Hour),
		ForexFactoryJSONPath: getEnv("FOREX_FACTORY_JSON_PATH", "forexfactory.json"),
	}

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return cfg, nil
}

// validate ensures critical settings are provided and within valid ranges.
func (c *Config) validate() error {
	if c.Port <= 0 || c.Port > 65535 {
		return fmt.Errorf("invalid PORT: %d (must be 1-65535)", c.Port)
	}
	if strings.TrimSpace(c.DatabaseURL) == "" {
		return fmt.Errorf("DATABASE_URL must not be empty")
	}
	return nil
}

// Helper: read environment variable with fallback default.
func getEnv(key, defaultVal string) string {
	if val, ok := os.LookupEnv(key); ok && val != "" {
		return val
	}
	return defaultVal
}

// Helper: read environment variable parsed as integer with fallback default.
func getEnvAsInt(key string, defaultVal int) int {
	valStr := getEnv(key, "")
	if valStr == "" {
		return defaultVal
	}
	val, err := strconv.Atoi(valStr)
	if err != nil {
		return defaultVal
	}
	return val
}

// Helper: read environment variable parsed as boolean with fallback default.
func getEnvAsBool(key string, defaultVal bool) bool {
	valStr := getEnv(key, "")
	if valStr == "" {
		return defaultVal
	}
	val, err := strconv.ParseBool(valStr)
	if err != nil {
		return defaultVal
	}
	return val
}

// Helper: read environment variable parsed as time.Duration (e.g., "15s", "1m", "1h").
func getEnvAsDuration(key string, defaultVal time.Duration) time.Duration {
	valStr := getEnv(key, "")
	if valStr == "" {
		return defaultVal
	}
	val, err := time.ParseDuration(valStr)
	if err != nil {
		return defaultVal
	}
	return val
}

// loadDotEnv parses key=value pairs from a file into os.Setenv if not already set.
func loadDotEnv(filepath string) error {
	file, err := os.Open(filepath)
	if err != nil {
		return err // file does not exist, ignore
	}
	defer file.Close() // LIFO cleanup: closes the file descriptor when this function returns

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue // skip empty lines and comments
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			k := strings.TrimSpace(parts[0])
			v := strings.Trim(strings.TrimSpace(parts[1]), `"'`)
			// Only set if not already defined in the OS environment
			if _, exists := os.LookupEnv(k); !exists {
				_ = os.Setenv(k, v)
			}
		}
	}
	return scanner.Err()
}
