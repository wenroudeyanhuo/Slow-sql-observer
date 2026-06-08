package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDotEnvSetsMissingVariablesOnly(t *testing.T) {
	tempDir := t.TempDir()
	envPath := filepath.Join(tempDir, ".env")
	content := "SSO_SERVER_ADDR=:9090\nSSO_DB_SCHEMA=from-dotenv\n"
	if err := os.WriteFile(envPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write env file: %v", err)
	}

	t.Setenv("SSO_DB_SCHEMA", "from-env")

	if err := loadDotEnv(envPath); err != nil {
		t.Fatalf("loadDotEnv returned error: %v", err)
	}

	if got := os.Getenv("SSO_SERVER_ADDR"); got != ":9090" {
		t.Fatalf("expected SSO_SERVER_ADDR from .env, got %q", got)
	}
	if got := os.Getenv("SSO_DB_SCHEMA"); got != "from-env" {
		t.Fatalf("expected existing env var to win, got %q", got)
	}
}
