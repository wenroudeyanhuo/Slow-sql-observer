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
	Server    ServerConfig
	Collector CollectorConfig
	Database  DatabaseConfig
}

type ServerConfig struct {
	Addr   string
	WebDir string
}

type CollectorConfig struct {
	InstanceName string
	SlowLogPath  string
	PollInterval time.Duration
}

type DatabaseConfig struct {
	DSN    string
	Schema string
}

func Load() (Config, error) {
	if err := loadDotEnv(".env"); err != nil {
		return Config{}, err
	}

	cfg := Config{
		Server: ServerConfig{
			Addr:   getEnv("SSO_SERVER_ADDR", ":8080"),
			WebDir: getEnv("SSO_WEB_DIR", "./web"),
		},
		Collector: CollectorConfig{
			InstanceName: getEnv("SSO_INSTANCE_NAME", "local-mysql"),
			SlowLogPath:  getEnv("SSO_SLOW_LOG_PATH", "./scripts/sample-slow.log"),
			PollInterval: getDurationEnv("SSO_COLLECTOR_POLL_INTERVAL", 15*time.Second),
		},
		Database: DatabaseConfig{
			DSN:    getEnv("SSO_DB_DSN", "root:root@tcp(127.0.0.1:3306)/"),
			Schema: getEnv("SSO_DB_SCHEMA", "slow_sql_observer"),
		},
	}

	if strings.TrimSpace(cfg.Collector.InstanceName) == "" {
		return Config{}, fmt.Errorf("SSO_INSTANCE_NAME must not be empty")
	}
	if strings.TrimSpace(cfg.Collector.SlowLogPath) == "" {
		return Config{}, fmt.Errorf("SSO_SLOW_LOG_PATH must not be empty")
	}
	if strings.TrimSpace(cfg.Database.DSN) == "" {
		return Config{}, fmt.Errorf("SSO_DB_DSN must not be empty")
	}
	if strings.TrimSpace(cfg.Database.Schema) == "" {
		return Config{}, fmt.Errorf("SSO_DB_SCHEMA must not be empty")
	}

	return cfg, nil
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

func getEnv(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func getDurationEnv(key string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	duration, err := time.ParseDuration(value)
	if err == nil {
		return duration
	}
	if seconds, err := strconv.Atoi(value); err == nil {
		return time.Duration(seconds) * time.Second
	}
	return fallback
}
