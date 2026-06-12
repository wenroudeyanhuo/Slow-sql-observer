package discovery

import (
	"fmt"
	"testing"

	"slow-sql-observer/internal/model"
)

func TestResolveFileOutput(t *testing.T) {
	logOutput := "FILE"
	disc := model.SourceDiscovery{
		DiscoveryState:      model.DiscoveryStateHealthy,
		DiscoveredLogOutput: &logOutput,
	}
	result := Resolve(disc)
	if result.DiscoveryState != model.DiscoveryStateHealthy {
		t.Fatalf("expected healthy state, got %q", result.DiscoveryState)
	}
	if result.EffectiveAcqMode == nil || *result.EffectiveAcqMode != model.EffectiveModeMySQLFile {
		t.Fatalf("expected mysql_file effective mode, got %v", result.EffectiveAcqMode)
	}
}

func TestResolveTableOutput(t *testing.T) {
	logOutput := "TABLE"
	disc := model.SourceDiscovery{
		DiscoveryState:      model.DiscoveryStateHealthy,
		DiscoveredLogOutput: &logOutput,
	}
	result := Resolve(disc)
	if result.DiscoveryState != model.DiscoveryStateHealthy {
		t.Fatalf("expected healthy state, got %q", result.DiscoveryState)
	}
	if result.EffectiveAcqMode == nil || *result.EffectiveAcqMode != model.EffectiveModeMySQLTable {
		t.Fatalf("expected mysql_table effective mode, got %v", result.EffectiveAcqMode)
	}
}

func TestResolveMixedFileAndTableDefaultsToFile(t *testing.T) {
	logOutput := "FILE,TABLE"
	disc := model.SourceDiscovery{
		DiscoveryState:      model.DiscoveryStateHealthy,
		DiscoveredLogOutput: &logOutput,
	}
	result := Resolve(disc)
	if result.DiscoveryState != model.DiscoveryStateHealthy {
		t.Fatalf("expected healthy state, got %q", result.DiscoveryState)
	}
	if result.EffectiveAcqMode == nil || *result.EffectiveAcqMode != model.EffectiveModeMySQLFile {
		t.Fatalf("expected mysql_file for mixed FILE,TABLE, got %v", result.EffectiveAcqMode)
	}
}

func TestResolveMixedTableAndFileDefaultsToFile(t *testing.T) {
	logOutput := "TABLE,FILE"
	disc := model.SourceDiscovery{
		DiscoveryState:      model.DiscoveryStateHealthy,
		DiscoveredLogOutput: &logOutput,
	}
	result := Resolve(disc)
	if result.EffectiveAcqMode == nil || *result.EffectiveAcqMode != model.EffectiveModeMySQLFile {
		t.Fatalf("expected mysql_file for mixed TABLE,FILE, got %v", result.EffectiveAcqMode)
	}
}

func TestResolveUnsupportedLogOutput(t *testing.T) {
	logOutput := "NONE"
	disc := model.SourceDiscovery{
		DiscoveryState:      model.DiscoveryStateHealthy,
		DiscoveredLogOutput: &logOutput,
	}
	result := Resolve(disc)
	if result.DiscoveryState != model.DiscoveryStateBlocked {
		t.Fatalf("expected blocked state for unsupported log_output, got %q", result.DiscoveryState)
	}
	if result.DiagnosticMessage == nil {
		t.Fatal("expected diagnostic message for blocked state")
	}
}

func TestResolveEmptyLogOutput(t *testing.T) {
	logOutput := ""
	disc := model.SourceDiscovery{
		DiscoveryState:      model.DiscoveryStateHealthy,
		DiscoveredLogOutput: &logOutput,
	}
	result := Resolve(disc)
	if result.DiscoveryState != model.DiscoveryStateBlocked {
		t.Fatalf("expected blocked state for empty log_output, got %q", result.DiscoveryState)
	}
}

func TestResolveNilLogOutput(t *testing.T) {
	disc := model.SourceDiscovery{
		DiscoveryState: model.DiscoveryStateHealthy,
	}
	result := Resolve(disc)
	if result.DiscoveryState != model.DiscoveryStateBlocked {
		t.Fatalf("expected blocked state for nil log_output, got %q", result.DiscoveryState)
	}
}

func TestResolveSkipsUnhealthyDiscovery(t *testing.T) {
	disc := model.SourceDiscovery{
		DiscoveryState: model.DiscoveryStateError,
	}
	result := Resolve(disc)
	if result.DiscoveryState != model.DiscoveryStateError {
		t.Fatalf("expected error state to pass through, got %q", result.DiscoveryState)
	}
	if result.EffectiveAcqMode != nil {
		t.Fatalf("expected no effective mode for unhealthy discovery, got %v", result.EffectiveAcqMode)
	}
}

func TestResolveBlockedDiscoveryPassesThrough(t *testing.T) {
	msg := "slow query log is disabled"
	disc := model.SourceDiscovery{
		DiscoveryState:    model.DiscoveryStateBlocked,
		DiagnosticMessage: &msg,
	}
	result := Resolve(disc)
	if result.DiscoveryState != model.DiscoveryStateBlocked {
		t.Fatalf("expected blocked state to pass through, got %q", result.DiscoveryState)
	}
	if result.DiagnosticMessage == nil || *result.DiagnosticMessage != msg {
		t.Fatal("expected diagnostic message to be preserved")
	}
}

func TestErrorDiscoverySetsErrorState(t *testing.T) {
	disc := errorDiscovery(fmt.Errorf("connection refused"))
	if disc.DiscoveryState != model.DiscoveryStateError {
		t.Fatalf("expected error state, got %q", disc.DiscoveryState)
	}
	if disc.DiagnosticMessage == nil || *disc.DiagnosticMessage != "connection refused" {
		t.Fatalf("expected error message, got %v", disc.DiagnosticMessage)
	}
}
