package config

import (
	"os"
	"testing"
)

func TestLoadHappyPath(t *testing.T) {
	// Set up environment variables
	t.Setenv("JENNGATE_DB_HOST", "db.example.com")
	t.Setenv("JENNGATE_DB_PORT", "5433")
	t.Setenv("JENNGATE_DB_NAME", "jenn_prod")
	t.Setenv("JENNGATE_DB_USER", "admin")
	t.Setenv("JENNGATE_DB_PASSWORD", "supersecret")
	t.Setenv("JENNGATE_HTTP_PORT", "8082")
	t.Setenv("JENNGATE_LOG_LEVEL", "debug")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	if cfg.DBHost != "db.example.com" {
		t.Errorf("Expected DBHost=db.example.com, got %s", cfg.DBHost)
	}
	if cfg.DBPort != 5433 {
		t.Errorf("Expected DBPort=5433, got %d", cfg.DBPort)
	}
	if cfg.DBName != "jenn_prod" {
		t.Errorf("Expected DBName=jenn_prod, got %s", cfg.DBName)
	}
	if cfg.DBUser != "admin" {
		t.Errorf("Expected DBUser=admin, got %s", cfg.DBUser)
	}
	if cfg.DBPassword != "supersecret" {
		t.Errorf("Expected DBPassword=supersecret, got %s", cfg.DBPassword)
	}
	if cfg.HTTPPort != 8082 {
		t.Errorf("Expected HTTPPort=8082, got %d", cfg.HTTPPort)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("Expected LogLevel=debug, got %s", cfg.LogLevel)
	}
}

func TestLoadMissingRequiredPassword(t *testing.T) {
	// Ensure JENNGATE_DB_PASSWORD is not set
	t.Setenv("JENNGATE_DB_PASSWORD", "")
	os.Unsetenv("JENNGATE_DB_PASSWORD")

	cfg, err := Load()
	if err == nil {
		t.Fatalf("Load() expected error for missing JENNGATE_DB_PASSWORD, got cfg=%v", cfg)
	}
	if err.Error() != "JENNGATE_DB_PASSWORD environment variable is required" {
		t.Errorf("Expected error about missing DB_PASSWORD, got: %v", err)
	}
}

func TestLoadInvalidDBPort(t *testing.T) {
	t.Setenv("JENNGATE_DB_PASSWORD", "secret")
	t.Setenv("JENNGATE_DB_PORT", "invalid")

	cfg, err := Load()
	if err == nil {
		t.Fatalf("Load() expected error for invalid DBPort, got cfg=%v", cfg)
	}
	// Error should mention JENNGATE_DB_PORT
	errMsg := err.Error()
	if errMsg != "invalid JENNGATE_DB_PORT: strconv.Atoi: parsing \"invalid\": invalid syntax" {
		t.Errorf("Expected error about invalid DB_PORT, got: %v", err)
	}
}

func TestLoadDBPortOutOfRange(t *testing.T) {
	t.Setenv("JENNGATE_DB_PASSWORD", "secret")
	t.Setenv("JENNGATE_DB_PORT", "99999")

	cfg, err := Load()
	if err == nil {
		t.Fatalf("Load() expected error for out-of-range DBPort, got cfg=%v", cfg)
	}
	errMsg := err.Error()
	if errMsg != "JENNGATE_DB_PORT must be between 1 and 65535, got 99999" {
		t.Errorf("Expected error about DBPort out of range, got: %v", err)
	}
}

func TestLoadInvalidHTTPPort(t *testing.T) {
	t.Setenv("JENNGATE_DB_PASSWORD", "secret")
	t.Setenv("JENNGATE_HTTP_PORT", "not_a_number")

	cfg, err := Load()
	if err == nil {
		t.Fatalf("Load() expected error for invalid HTTPPort, got cfg=%v", cfg)
	}
	errMsg := err.Error()
	if errMsg != "invalid JENNGATE_HTTP_PORT: strconv.Atoi: parsing \"not_a_number\": invalid syntax" {
		t.Errorf("Expected error about invalid HTTP_PORT, got: %v", err)
	}
}

func TestLoadHTTPPortOutOfRange(t *testing.T) {
	t.Setenv("JENNGATE_DB_PASSWORD", "secret")
	t.Setenv("JENNGATE_HTTP_PORT", "0")

	cfg, err := Load()
	if err == nil {
		t.Fatalf("Load() expected error for out-of-range HTTPPort, got cfg=%v", cfg)
	}
	errMsg := err.Error()
	if errMsg != "JENNGATE_HTTP_PORT must be between 1 and 65535, got 0" {
		t.Errorf("Expected error about HTTPPort out of range, got: %v", err)
	}
}

func TestLoadDefaultValues(t *testing.T) {
	// Only set required password; let all others default
	t.Setenv("JENNGATE_DB_PASSWORD", "secret")
	os.Unsetenv("JENNGATE_DB_HOST")
	os.Unsetenv("JENNGATE_DB_PORT")
	os.Unsetenv("JENNGATE_DB_NAME")
	os.Unsetenv("JENNGATE_DB_USER")
	os.Unsetenv("JENNGATE_HTTP_PORT")
	os.Unsetenv("JENNGATE_LOG_LEVEL")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	if cfg.DBHost != "localhost" {
		t.Errorf("Expected DBHost=localhost, got %s", cfg.DBHost)
	}
	if cfg.DBPort != 5432 {
		t.Errorf("Expected DBPort=5432, got %d", cfg.DBPort)
	}
	if cfg.DBName != "jenngate" {
		t.Errorf("Expected DBName=jenngate, got %s", cfg.DBName)
	}
	if cfg.DBUser != "jenngate" {
		t.Errorf("Expected DBUser=jenngate, got %s", cfg.DBUser)
	}
	if cfg.HTTPPort != 8081 {
		t.Errorf("Expected HTTPPort=8081, got %d", cfg.HTTPPort)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("Expected LogLevel=info, got %s", cfg.LogLevel)
	}
}

func TestConfigString(t *testing.T) {
	cfg := &Config{
		DBHost:     "localhost",
		DBPort:     5432,
		DBName:     "jenngate",
		DBUser:     "jenngate",
		DBPassword: "supersecret",
		HTTPPort:   8081,
		LogLevel:   "info",
	}

	str := cfg.String()
	expected := "Config{DBHost:localhost DBPort:5432 DBName:jenngate DBUser:jenngate DBPassword:*** HTTPPort:8081 LogLevel:info}"
	if str != expected {
		t.Errorf("Expected String() to return %q, got %q", expected, str)
	}

	// Verify password is masked (should not contain the actual password)
	if contains(str, "supersecret") {
		t.Errorf("String() should mask the password, but found 'supersecret' in output: %s", str)
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	for i := 0; i < len(s)-len(substr)+1; i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
