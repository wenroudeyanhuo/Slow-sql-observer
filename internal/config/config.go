package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Server   ServerConfig
	Source   SourceConfig
	Analysis AnalysisConfig
	Runtime  RuntimeConfig
	Warnings []string
}

type ServerConfig struct {
	Addr   string
	WebDir string
}

type SourceConfig struct {
	InstanceName string
	SlowLogPath  string
	DatabaseDSN  string
	Timezone     string
	Description  string
}

type AnalysisConfig struct {
	DSN    string
	Schema string
}

type RuntimeConfig struct {
	CollectorPollInterval  time.Duration
	RawRecordRetentionDays int
	LogLevel               string
}

func Load() (Config, error) {
	if err := loadDotEnv(".env"); err != nil {
		return Config{}, err
	}

	resolver := newEnvResolver()
	cfg := Config{
		Server: ServerConfig{
			Addr:   resolver.stringValue("SSO_SERVER_ADDR", nil, ":8080"),
			WebDir: resolver.stringValue("SSO_WEB_DIR", nil, "./web"),
		},
		Source: SourceConfig{
			InstanceName: resolver.stringValue("SSO_SOURCE_INSTANCE_NAME", []string{"SSO_INSTANCE_NAME"}, "local-mysql"),
			SlowLogPath:  resolver.stringValue("SSO_SOURCE_SLOW_LOG_PATH", []string{"SSO_SLOW_LOG_PATH"}, "./scripts/sample-slow.log"),
			DatabaseDSN:  resolver.stringValue("SSO_SOURCE_DB_DSN", nil, ""),
			Timezone:     resolver.stringValue("SSO_SOURCE_TIMEZONE", nil, ""),
			Description:  resolver.stringValue("SSO_SOURCE_DESCRIPTION", nil, ""),
		},
		Analysis: AnalysisConfig{
			DSN:    resolver.stringValue("SSO_ANALYSIS_DB_DSN", []string{"SSO_DB_DSN"}, "root:root@tcp(127.0.0.1:3306)/"),
			Schema: resolver.stringValue("SSO_ANALYSIS_DB_SCHEMA", []string{"SSO_DB_SCHEMA"}, "slow_sql_observer"),
		},
		Runtime: RuntimeConfig{
			CollectorPollInterval:  resolver.durationValue("SSO_COLLECTOR_POLL_INTERVAL", nil, 15*time.Second),
			RawRecordRetentionDays: resolver.intValue("SSO_RAW_RECORD_RETENTION_DAYS", nil, 0),
			LogLevel:               resolver.stringValue("SSO_LOG_LEVEL", nil, "info"),
		},
		Warnings: resolver.warnings,
	}

	if strings.TrimSpace(cfg.Source.InstanceName) == "" {
		return Config{}, fmt.Errorf("SSO_SOURCE_INSTANCE_NAME must not be empty")
	}
	if strings.TrimSpace(cfg.Source.SlowLogPath) == "" {
		return Config{}, fmt.Errorf("SSO_SOURCE_SLOW_LOG_PATH must not be empty")
	}
	if strings.TrimSpace(cfg.Analysis.DSN) == "" {
		return Config{}, fmt.Errorf("SSO_ANALYSIS_DB_DSN must not be empty")
	}
	if strings.TrimSpace(cfg.Analysis.Schema) == "" {
		return Config{}, fmt.Errorf("SSO_ANALYSIS_DB_SCHEMA must not be empty")
	}

	return cfg, nil
}

type envResolver struct {
	warnings []string
}

func newEnvResolver() *envResolver {
	return &envResolver{}
}

func (r *envResolver) stringValue(preferred string, legacy []string, fallback string) string {
	if value, ok := lookupTrimmedEnv(preferred); ok {
		r.warnIgnoredLegacy(preferred, legacy)
		return value
	}
	for _, oldKey := range legacy {
		if value, ok := lookupTrimmedEnv(oldKey); ok {
			r.warnings = append(r.warnings, fmt.Sprintf("%s is deprecated; use %s instead", oldKey, preferred))
			return value
		}
	}
	return fallback
}

func (r *envResolver) durationValue(preferred string, legacy []string, fallback time.Duration) time.Duration {
	raw := r.stringValue(preferred, legacy, "")
	if strings.TrimSpace(raw) == "" {
		return fallback
	}
	duration, err := time.ParseDuration(raw)
	if err == nil {
		return duration
	}
	if seconds, err := strconv.Atoi(raw); err == nil {
		return time.Duration(seconds) * time.Second
	}
	return fallback
}

func (r *envResolver) intValue(preferred string, legacy []string, fallback int) int {
	raw := r.stringValue(preferred, legacy, "")
	if strings.TrimSpace(raw) == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return value
}

func (r *envResolver) warnIgnoredLegacy(preferred string, legacy []string) {
	for _, oldKey := range legacy {
		if _, ok := lookupTrimmedEnv(oldKey); ok {
			r.warnings = append(r.warnings, fmt.Sprintf("%s is deprecated and ignored because %s is set", oldKey, preferred))
		}
	}
}

func lookupTrimmedEnv(key string) (string, bool) {
	value, ok := os.LookupEnv(key)
	if !ok {
		return "", false
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return "", false
	}
	return value, true
}

func loadDotEnv(path string) error {
	file, err := os.Open(filepath.Clean(path))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		name, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		name = strings.TrimSpace(name)
		value = strings.TrimSpace(value)
		if name == "" {
			continue
		}
		if _, exists := os.LookupEnv(name); exists {
			continue
		}

		value = strings.Trim(value, `"'`)
		if err := os.Setenv(name, value); err != nil {
			return fmt.Errorf("set env %s from %s: %w", name, path, err)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	return nil
}
