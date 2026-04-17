package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds the application configuration.
type Config struct {
	DBHost     string
	DBPort     int
	DBName     string
	DBUser     string
	DBPassword string
	SSLMode    string
	HTTPPort   int
	LogLevel   string
}

// Load reads environment variables with prefix JENNGATE_ and returns a Config.
// JENNGATE_DB_PASSWORD is required; all others have sensible defaults.
func Load() (*Config, error) {
	cfg := &Config{
		DBHost:   getEnvOrDefault("JENNGATE_DB_HOST", "localhost"),
		DBName:   getEnvOrDefault("JENNGATE_DB_NAME", "jenngate"),
		DBUser:   getEnvOrDefault("JENNGATE_DB_USER", "jenngate"),
		SSLMode:  getEnvOrDefault("JENNGATE_DB_SSLMODE", "require"),
		HTTPPort: 8081,
		LogLevel: getEnvOrDefault("JENNGATE_LOG_LEVEL", "info"),
	}

	// DBPort with validation
	portStr := getEnvOrDefault("JENNGATE_DB_PORT", "5432")
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return nil, fmt.Errorf("invalid JENNGATE_DB_PORT: %w", err)
	}
	if port < 1 || port > 65535 {
		return nil, fmt.Errorf("JENNGATE_DB_PORT must be between 1 and 65535, got %d", port)
	}
	cfg.DBPort = port

	// HTTPPort with validation
	httpPortStr := getEnvOrDefault("JENNGATE_HTTP_PORT", "8081")
	httpPort, err := strconv.Atoi(httpPortStr)
	if err != nil {
		return nil, fmt.Errorf("invalid JENNGATE_HTTP_PORT: %w", err)
	}
	if httpPort < 1 || httpPort > 65535 {
		return nil, fmt.Errorf("JENNGATE_HTTP_PORT must be between 1 and 65535, got %d", httpPort)
	}
	cfg.HTTPPort = httpPort

	// DBPassword is required
	dbPassword, ok := os.LookupEnv("JENNGATE_DB_PASSWORD")
	if !ok {
		return nil, fmt.Errorf("JENNGATE_DB_PASSWORD environment variable is required")
	}
	cfg.DBPassword = dbPassword

	return cfg, nil
}

// getEnvOrDefault retrieves an environment variable or returns the default.
func getEnvOrDefault(key, defaultValue string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	return defaultValue
}

// String returns a readable summary of the Config with the password masked.
func (c *Config) String() string {
	return fmt.Sprintf(
		"Config{DBHost:%s DBPort:%d DBName:%s DBUser:%s SSLMode:%s DBPassword:*** HTTPPort:%d LogLevel:%s}",
		c.DBHost,
		c.DBPort,
		c.DBName,
		c.DBUser,
		c.SSLMode,
		c.HTTPPort,
		c.LogLevel,
	)
}
