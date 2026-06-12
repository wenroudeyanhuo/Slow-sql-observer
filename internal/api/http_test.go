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

func (stubQueryService) GetOverview(context.Context, model.OverviewParams) (model.Overview, error) {
	now := time.Now().UTC()
	return model.Overview{TotalRecords: 1, TotalFingerprints: 1, LastIngestedAt: &now}, nil
}

func (stubQueryService) ListFingerprints(context.Context, model.ListFingerprintsParams) (model.PaginatedFingerprints, error) {
	return model.PaginatedFingerprints{Items: []model.FingerprintRecordView{{Fingerprint: model.Fingerprint{ID: 1, Hash: "abc"}}}}, nil
}

func (stubQueryService) GetFingerprint(context.Context, int64, model.GetFingerprintParams) (*model.FingerprintRecordView, error) {
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

func (stubQueryService) GetDiscovery(_ context.Context, _ int64) (*model.SourceDiscovery, error) {
	return nil, nil
}

func (stubQueryService) GetSourceID(context.Context) (int64, error) {
	return 1, nil
}

func TestOverviewEndpoint(t *testing.T) {
	server := NewServer(stubQueryService{}, "../../web", 1)
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

	server := NewServer(stubQueryService{}, webDir, 1)
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
	server := NewServer(stubQueryService{}, "../../web", 1)

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

type captureQueryService struct {
	stubQueryService
	overviewParams           model.OverviewParams
	fingerprintListParams    model.ListFingerprintsParams
	fingerprintDetailParams  model.GetFingerprintParams
	fingerprintRecordsParams model.ListFingerprintRecordsParams
}

func (c *captureQueryService) GetOverview(ctx context.Context, params model.OverviewParams) (model.Overview, error) {
	c.overviewParams = params
	return c.stubQueryService.GetOverview(ctx, params)
}

func (c *captureQueryService) ListFingerprints(ctx context.Context, params model.ListFingerprintsParams) (model.PaginatedFingerprints, error) {
	c.fingerprintListParams = params
	return c.stubQueryService.ListFingerprints(ctx, params)
}

func (c *captureQueryService) GetFingerprint(ctx context.Context, id int64, params model.GetFingerprintParams) (*model.FingerprintRecordView, error) {
	c.fingerprintDetailParams = params
	return c.stubQueryService.GetFingerprint(ctx, id, params)
}

func (c *captureQueryService) ListFingerprintRecords(ctx context.Context, id int64, params model.ListFingerprintRecordsParams) (model.PaginatedRecords, error) {
	c.fingerprintRecordsParams = params
	return c.stubQueryService.ListFingerprintRecords(ctx, id, params)
}

func TestThresholdQueryParamPropagation(t *testing.T) {
	service := &captureQueryService{}
	server := NewServer(service, "../../web", 1)

	cases := []struct {
		path  string
		check func(t *testing.T)
	}{
		{
			path: "/api/dashboard/overview?minQueryTimeSec=2.5",
			check: func(t *testing.T) {
				if service.overviewParams.MinQueryTimeSec != 2.5 {
					t.Fatalf("expected overview threshold 2.5, got %v", service.overviewParams.MinQueryTimeSec)
				}
			},
		},
		{
			path: "/api/slow-sql/fingerprints?minQueryTimeSec=3",
			check: func(t *testing.T) {
				if service.fingerprintListParams.MinQueryTimeSec != 3 {
					t.Fatalf("expected fingerprint list threshold 3, got %v", service.fingerprintListParams.MinQueryTimeSec)
				}
			},
		},
		{
			path: "/api/slow-sql/fingerprints/1?minQueryTimeSec=4",
			check: func(t *testing.T) {
				if service.fingerprintDetailParams.MinQueryTimeSec != 4 {
					t.Fatalf("expected fingerprint detail threshold 4, got %v", service.fingerprintDetailParams.MinQueryTimeSec)
				}
			},
		},
		{
			path: "/api/slow-sql/fingerprints/1/records?minQueryTimeSec=5",
			check: func(t *testing.T) {
				if service.fingerprintRecordsParams.MinQueryTimeSec != 5 {
					t.Fatalf("expected fingerprint records threshold 5, got %v", service.fingerprintRecordsParams.MinQueryTimeSec)
				}
			},
		},
	}

	for _, tc := range cases {
		req := httptest.NewRequest(http.MethodGet, tc.path, nil)
		recorder := httptest.NewRecorder()
		server.Handler().ServeHTTP(recorder, req)
		if recorder.Code != http.StatusOK {
			t.Fatalf("expected status 200 for %s, got %d", tc.path, recorder.Code)
		}
		tc.check(t)
	}
}

func TestThresholdDefaultsToServerConfig(t *testing.T) {
	service := &captureQueryService{}
	server := NewServer(service, "../../web", 1.25)
	req := httptest.NewRequest(http.MethodGet, "/api/dashboard/overview", nil)
	recorder := httptest.NewRecorder()

	server.Handler().ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
	if service.overviewParams.MinQueryTimeSec != 1.25 {
		t.Fatalf("expected default threshold 1.25, got %v", service.overviewParams.MinQueryTimeSec)
	}
}
