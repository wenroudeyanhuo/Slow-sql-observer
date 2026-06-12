package discovery

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	mysql "github.com/go-sql-driver/mysql"

	"slow-sql-observer/internal/model"
)

// Service inspects a source MySQL instance to discover slow-query logging state.
type Service struct{}

func NewService() *Service {
	return &Service{}
}

// Discover connects to the source MySQL and inspects slow-query logging configuration.
func (s *Service) Discover(ctx context.Context, dsn string) (model.SourceDiscovery, error) {
	parsed, err := mysql.ParseDSN(dsn)
	if err != nil {
		return errorDiscovery(fmt.Errorf("parse source DSN: %w", err)), nil
	}
	parsed.ParseTime = true

	db, err := sql.Open("mysql", parsed.FormatDSN())
	if err != nil {
		return errorDiscovery(fmt.Errorf("open source DB: %w", err)), nil
	}
	defer db.Close()

	if err := db.PingContext(ctx); err != nil {
		return errorDiscovery(fmt.Errorf("ping source DB: %w", err)), nil
	}

	disc := model.SourceDiscovery{
		DiscoveryState: model.DiscoveryStateHealthy,
		DiscoveredAt:   time.Now().UTC(),
	}

	// Discover source version
	var version string
	if err := db.QueryRowContext(ctx, `SELECT VERSION()`).Scan(&version); err == nil {
		disc.SourceVersion = &version
	}

	// Discover source host identity
	host := parsed.Addr
	if host != "" {
		disc.SourceHost = &host
	}

	// Check if slow query logging is enabled
	var slowLogName, slowLogEnabled string
	if err := db.QueryRowContext(ctx, `SHOW VARIABLES LIKE 'slow_query_log'`).Scan(&slowLogName, &slowLogEnabled); err != nil {
		if err == sql.ErrNoRows {
			disc.DiscoveryState = model.DiscoveryStateBlocked
			msg := "source MySQL does not expose slow_query_log variable (managed/restricted instance)"
			disc.DiagnosticMessage = &msg
			return disc, nil
		}
		return errorDiscovery(fmt.Errorf("check slow_query_log: %w", err)), nil
	}
	enabled := strings.EqualFold(slowLogEnabled, "ON")
	disc.SlowLogEnabled = &enabled

	if !enabled {
		disc.DiscoveryState = model.DiscoveryStateBlocked
		msg := "source MySQL slow query log is not enabled (slow_query_log = OFF)"
		disc.DiagnosticMessage = &msg
		return disc, nil
	}

	// Discover log_output
	var logOutputName, logOutput string
	if err := db.QueryRowContext(ctx, `SHOW VARIABLES LIKE 'log_output'`).Scan(&logOutputName, &logOutput); err != nil {
		if err == sql.ErrNoRows {
			disc.DiscoveryState = model.DiscoveryStateBlocked
			msg := "source MySQL does not expose log_output variable (managed/restricted instance)"
			disc.DiagnosticMessage = &msg
			return disc, nil
		}
		return errorDiscovery(fmt.Errorf("check log_output: %w", err)), nil
	}
	logOutput = strings.TrimSpace(logOutput)
	disc.DiscoveredLogOutput = &logOutput

	// Discover slow-log file path (best-effort, not all managed instances expose this)
	var filePathName, slowLogFile string
	if err := db.QueryRowContext(ctx, `SHOW VARIABLES LIKE 'slow_query_log_file'`).Scan(&filePathName, &slowLogFile); err == nil {
		slowLogFile = strings.TrimSpace(slowLogFile)
		if slowLogFile != "" {
			disc.DiscoveredFilePath = &slowLogFile
		}
	}
	// If slow_query_log_file is not visible (ErrNoRows on managed DB), that's OK — continue.

	return disc, nil
}

// Resolve determines the effective acquisition mode from discovery results.
func Resolve(disc model.SourceDiscovery) model.SourceDiscovery {
	if disc.DiscoveryState != model.DiscoveryStateHealthy {
		return disc
	}

	logOutput := ""
	if disc.DiscoveredLogOutput != nil {
		logOutput = strings.ToUpper(*disc.DiscoveredLogOutput)
	}

	var effectiveMode string
	switch {
	case strings.Contains(logOutput, "FILE") && strings.Contains(logOutput, "TABLE"):
		effectiveMode = model.EffectiveModeMySQLFile
	case strings.Contains(logOutput, "FILE"):
		effectiveMode = model.EffectiveModeMySQLFile
	case strings.Contains(logOutput, "TABLE"):
		effectiveMode = model.EffectiveModeMySQLTable
	default:
		disc.DiscoveryState = model.DiscoveryStateBlocked
		msg := fmt.Sprintf("unsupported log_output value: %q; expected FILE, TABLE, or FILE,TABLE", logOutput)
		disc.DiagnosticMessage = &msg
		return disc
	}

	disc.EffectiveAcqMode = &effectiveMode
	return disc
}

func errorDiscovery(err error) model.SourceDiscovery {
	msg := err.Error()
	return model.SourceDiscovery{
		DiscoveryState:    model.DiscoveryStateError,
		DiagnosticMessage: &msg,
		DiscoveredAt:      time.Now().UTC(),
	}
}
