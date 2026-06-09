package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadDotEnvSetsMissingVariablesOnly(t *testing.T) {
	tempDir := t.TempDir()
	envPath := filepath.Join(tempDir, ".env")
	content := "SSO_SERVER_ADDR=:9090\nSSO_ANALYSIS_DB_SCHEMA=from-dotenv\n"
	if err := os.WriteFile(envPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write env file: %v", err)
	}

	t.Setenv("SSO_ANALYSIS_DB_SCHEMA", "from-env")

	if err := loadDotEnv(envPath); err != nil {
		t.Fatalf("loadDotEnv returned error: %v", err)
	}

	if got := os.Getenv("SSO_SERVER_ADDR"); got != ":9090" {
		t.Fatalf("expected SSO_SERVER_ADDR from .env, got %q", got)
	}
	if got := os.Getenv("SSO_ANALYSIS_DB_SCHEMA"); got != "from-env" {
		t.Fatalf("expected existing env var to win, got %q", got)
	}
}

func TestLoadPrefersV2NamesOverLegacyFallback(t *testing.T) {
	t.Setenv("SSO_SOURCE_INSTANCE_NAME", "v2-instance")
	t.Setenv("SSO_SOURCE_SLOW_LOG_PATH", "/tmp/v2.log")
	t.Setenv("SSO_ANALYSIS_DB_DSN", "v2-analysis")
	t.Setenv("SSO_ANALYSIS_DB_SCHEMA", "v2_schema")
	t.Setenv("SSO_INSTANCE_NAME", "legacy-instance")
	t.Setenv("SSO_SLOW_LOG_PATH", "/tmp/legacy.log")
	t.Setenv("SSO_DB_DSN", "legacy-analysis")
	t.Setenv("SSO_DB_SCHEMA", "legacy_schema")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.Source.InstanceName != "v2-instance" {
		t.Fatalf("expected V2 instance name, got %q", cfg.Source.InstanceName)
	}
	if cfg.Analysis.Schema != "v2_schema" {
		t.Fatalf("expected V2 analysis schema, got %q", cfg.Analysis.Schema)
	}
	if len(cfg.Warnings) == 0 {
		t.Fatalf("expected deprecation warnings when legacy names are present")
	}
}

func TestLoadFallsBackToLegacyNamesForOneCycle(t *testing.T) {
	t.Setenv("SSO_SOURCE_INSTANCE_NAME", "")
	t.Setenv("SSO_SOURCE_SLOW_LOG_PATH", "")
	t.Setenv("SSO_ANALYSIS_DB_DSN", "")
	t.Setenv("SSO_ANALYSIS_DB_SCHEMA", "")
	t.Setenv("SSO_INSTANCE_NAME", "legacy-instance")
	t.Setenv("SSO_SLOW_LOG_PATH", "/tmp/legacy.log")
	t.Setenv("SSO_DB_DSN", "legacy-analysis")
	t.Setenv("SSO_DB_SCHEMA", "legacy_schema")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.Source.InstanceName != "legacy-instance" {
		t.Fatalf("expected legacy instance fallback, got %q", cfg.Source.InstanceName)
	}
	if cfg.Analysis.DSN != "legacy-analysis" {
		t.Fatalf("expected legacy analysis dsn fallback, got %q", cfg.Analysis.DSN)
	}
	if len(cfg.Warnings) < 4 {
		t.Fatalf("expected warnings for each legacy variable, got %d", len(cfg.Warnings))
	}
	if !strings.Contains(strings.Join(cfg.Warnings, " "), "deprecated") {
		t.Fatalf("expected warning text to mention deprecation")
	}
}
