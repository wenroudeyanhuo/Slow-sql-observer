package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"slow-sql-observer/internal/model"
)

type stubQueryService struct{}

func (stubQueryService) GetOverview(context.Context) (model.Overview, error) {
	now := time.Now().UTC()
	return model.Overview{TotalRecords: 1, TotalFingerprints: 1, LastIngestedAt: &now}, nil
}

func (stubQueryService) ListFingerprints(context.Context, model.ListFingerprintsParams) (model.PaginatedFingerprints, error) {
	return model.PaginatedFingerprints{Items: []model.FingerprintRecordView{{Fingerprint: model.Fingerprint{ID: 1, Hash: "abc"}}}}, nil
}

func (stubQueryService) GetFingerprint(context.Context, int64) (*model.FingerprintRecordView, error) {
	return &model.FingerprintRecordView{Fingerprint: model.Fingerprint{ID: 1, Hash: "abc"}}, nil
}

func (stubQueryService) ListFingerprintRecords(context.Context, int64, model.ListFingerprintRecordsParams) (model.PaginatedRecords, error) {
	return model.PaginatedRecords{Items: []model.SlowQueryRecord{{ID: 1, RawSQL: "SELECT 1"}}}, nil
}

func (stubQueryService) GetSource(context.Context) (*model.Source, error) {
	return &model.Source{ID: 1, InstanceName: "source-a", SlowLogPath: "/tmp/slow.log"}, nil
}

func (stubQueryService) GetCollectorStatus(context.Context) (*model.CollectorStatus, error) {
	return &model.CollectorStatus{SourceID: 1, CollectorState: model.CollectorStateHealthy, SourceAccessState: model.SourceAccessAccessible}, nil
}

func (stubQueryService) GetAcquisitionStatus(context.Context) (*model.AcquisitionStatus, error) {
	return &model.AcquisitionStatus{SourceID: 1, AcquisitionState: model.AcquisitionStateHealthy, RemoteAccessState: model.SourceAccessAccessible, TransportMode: model.LogModeLocalFile}, nil
}

func TestOverviewEndpoint(t *testing.T) {
	server := NewServer(stubQueryService{}, "../../web")
	req := httptest.NewRequest(http.MethodGet, "/api/dashboard/overview", nil)
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
}

func TestRootServesIndexWithoutRedirectLoop(t *testing.T) {
	webDir := t.TempDir()
	indexPath := filepath.Join(webDir, "index.html")
	if err := os.WriteFile(indexPath, []byte("<html><body>ok</body></html>"), 0o644); err != nil {
		t.Fatalf("write index file: %v", err)
	}

	server := NewServer(stubQueryService{}, webDir)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
	if location := recorder.Header().Get("Location"); location != "" {
		t.Fatalf("expected no redirect, got location %q", location)
	}
}

func TestSourceStatusEndpoints(t *testing.T) {
	server := NewServer(stubQueryService{}, "../../web")

	sourceReq := httptest.NewRequest(http.MethodGet, "/api/source", nil)
	sourceRecorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(sourceRecorder, sourceReq)
	if sourceRecorder.Code != http.StatusOK {
		t.Fatalf("expected source endpoint status 200, got %d", sourceRecorder.Code)
	}

	statusReq := httptest.NewRequest(http.MethodGet, "/api/collector/status", nil)
	statusRecorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(statusRecorder, statusReq)
	if statusRecorder.Code != http.StatusOK {
		t.Fatalf("expected collector status endpoint status 200, got %d", statusRecorder.Code)
	}

	acquisitionReq := httptest.NewRequest(http.MethodGet, "/api/acquisition/status", nil)
	acquisitionRecorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(acquisitionRecorder, acquisitionReq)
	if acquisitionRecorder.Code != http.StatusOK {
		t.Fatalf("expected acquisition status endpoint status 200, got %d", acquisitionRecorder.Code)
	}
}
