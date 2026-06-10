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

func TestLoadAcceptsSSHPullConfiguration(t *testing.T) {
	t.Setenv("SSO_SOURCE_INSTANCE_NAME", "remote-mysql")
	t.Setenv("SSO_SOURCE_LOG_MODE", "ssh_pull")
	t.Setenv("SSO_SOURCE_REMOTE_HOST", "db-prod")
	t.Setenv("SSO_SOURCE_REMOTE_PORT", "22")
	t.Setenv("SSO_SOURCE_REMOTE_USER", "observer")
	t.Setenv("SSO_SOURCE_REMOTE_SLOW_LOG_PATH", "/var/log/mysql/slow.log")
	t.Setenv("SSO_SOURCE_SSH_PRIVATE_KEY_PATH", "C:/keys/id_rsa")
	t.Setenv("SSO_SOURCE_SSH_KNOWN_HOSTS_PATH", "C:/keys/known_hosts")
	t.Setenv("SSO_SOURCE_LOCAL_SPOOL_DIR", "./var/spool")
	t.Setenv("SSO_SOURCE_INITIAL_POSITION", "start")
	t.Setenv("SSO_SOURCE_LOCAL_SPOOL_MAX_BYTES", "2048")
	t.Setenv("SSO_ANALYSIS_DB_DSN", "root:root@tcp(127.0.0.1:3306)/")
	t.Setenv("SSO_ANALYSIS_DB_SCHEMA", "slow_sql_observer")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.Source.LogMode != "ssh_pull" {
		t.Fatalf("expected ssh_pull mode, got %q", cfg.Source.LogMode)
	}
	if cfg.Source.EffectiveParsePath() == "" {
		t.Fatalf("expected spool parse path to be derived")
	}
	if cfg.Source.IdentityPath() != "/var/log/mysql/slow.log" {
		t.Fatalf("expected remote slow log path as identity path, got %q", cfg.Source.IdentityPath())
	}
}

func TestLoadRejectsIncompleteSSHPullConfiguration(t *testing.T) {
	t.Setenv("SSO_SOURCE_INSTANCE_NAME", "remote-mysql")
	t.Setenv("SSO_SOURCE_LOG_MODE", "ssh_pull")
	t.Setenv("SSO_ANALYSIS_DB_DSN", "root:root@tcp(127.0.0.1:3306)/")
	t.Setenv("SSO_ANALYSIS_DB_SCHEMA", "slow_sql_observer")

	if _, err := Load(); err == nil {
		t.Fatalf("expected ssh_pull validation error")
	}
}
