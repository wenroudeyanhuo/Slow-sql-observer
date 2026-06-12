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

func (stubQueryService) GetDashboardTrends(context.Context, model.TrendParams) (model.DashboardTrends, error) {
	now := time.Now().UTC()
	return model.DashboardTrends{
		Bucket:      model.TrendBucketDay,
		Days:        7,
		WindowStart: now.Add(-6 * 24 * time.Hour),
		WindowEnd:   now,
		Series:      []model.DashboardTrendBucket{{BucketStart: now.Add(-24 * time.Hour), TotalRecords: 1}},
	}, nil
}

func (stubQueryService) ListFingerprints(context.Context, model.ListFingerprintsParams) (model.PaginatedFingerprints, error) {
	return model.PaginatedFingerprints{Items: []model.FingerprintRecordView{{Fingerprint: model.Fingerprint{ID: 1, Hash: "abc"}}}}, nil
}

func (stubQueryService) GetFingerprint(context.Context, int64, model.GetFingerprintParams) (*model.FingerprintRecordView, error) {
	return &model.FingerprintRecordView{Fingerprint: model.Fingerprint{ID: 1, Hash: "abc"}}, nil
}

func (stubQueryService) GetFingerprintTrends(_ context.Context, id int64, _ model.TrendParams) (model.FingerprintTrends, error) {
	now := time.Now().UTC()
	return model.FingerprintTrends{
		FingerprintID: id,
		Bucket:        model.TrendBucketDay,
		Days:          7,
		WindowStart:   now.Add(-6 * 24 * time.Hour),
		WindowEnd:     now,
		Series:        []model.FingerprintTrendBucket{{BucketStart: now.Add(-24 * time.Hour), TotalCount: 1}},
	}, nil
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
	dashboardTrendParams     model.TrendParams
	fingerprintListParams    model.ListFingerprintsParams
	fingerprintDetailParams  model.GetFingerprintParams
	fingerprintTrendParams   model.TrendParams
	fingerprintRecordsParams model.ListFingerprintRecordsParams
}

func (c *captureQueryService) GetOverview(ctx context.Context, params model.OverviewParams) (model.Overview, error) {
	c.overviewParams = params
	return c.stubQueryService.GetOverview(ctx, params)
}

func (c *captureQueryService) GetDashboardTrends(ctx context.Context, params model.TrendParams) (model.DashboardTrends, error) {
	c.dashboardTrendParams = params
	return c.stubQueryService.GetDashboardTrends(ctx, params)
}

func (c *captureQueryService) ListFingerprints(ctx context.Context, params model.ListFingerprintsParams) (model.PaginatedFingerprints, error) {
	c.fingerprintListParams = params
	return c.stubQueryService.ListFingerprints(ctx, params)
}

func (c *captureQueryService) GetFingerprint(ctx context.Context, id int64, params model.GetFingerprintParams) (*model.FingerprintRecordView, error) {
	c.fingerprintDetailParams = params
	return c.stubQueryService.GetFingerprint(ctx, id, params)
}

func (c *captureQueryService) GetFingerprintTrends(ctx context.Context, id int64, params model.TrendParams) (model.FingerprintTrends, error) {
	c.fingerprintTrendParams = params
	return c.stubQueryService.GetFingerprintTrends(ctx, id, params)
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
		{
			path: "/api/dashboard/trends?bucket=day&days=7&dbName=app&minQueryTimeSec=2",
			check: func(t *testing.T) {
				if service.dashboardTrendParams.Bucket != model.TrendBucketDay {
					t.Fatalf("expected dashboard trend bucket day, got %q", service.dashboardTrendParams.Bucket)
				}
				if service.dashboardTrendParams.Days != 7 {
					t.Fatalf("expected dashboard trend days 7, got %d", service.dashboardTrendParams.Days)
				}
				if service.dashboardTrendParams.DBName != "app" {
					t.Fatalf("expected dashboard trend dbName app, got %q", service.dashboardTrendParams.DBName)
				}
				if service.dashboardTrendParams.MinQueryTimeSec != 2 {
					t.Fatalf("expected dashboard trend threshold 2, got %v", service.dashboardTrendParams.MinQueryTimeSec)
				}
			},
		},
		{
			path: "/api/slow-sql/fingerprints/1/trends?bucket=hour&days=2&minQueryTimeSec=6",
			check: func(t *testing.T) {
				if service.fingerprintTrendParams.Bucket != model.TrendBucketHour {
					t.Fatalf("expected fingerprint trend bucket hour, got %q", service.fingerprintTrendParams.Bucket)
				}
				if service.fingerprintTrendParams.Days != 2 {
					t.Fatalf("expected fingerprint trend days 2, got %d", service.fingerprintTrendParams.Days)
				}
				if service.fingerprintTrendParams.MinQueryTimeSec != 6 {
					t.Fatalf("expected fingerprint trend threshold 6, got %v", service.fingerprintTrendParams.MinQueryTimeSec)
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

func TestTrendValidationReturnsBadRequest(t *testing.T) {
	service := &captureQueryService{}
	server := NewServer(service, "../../web", 1)

	cases := []string{
		"/api/dashboard/trends?bucket=week",
		"/api/dashboard/trends?bucket=hour&days=30",
		"/api/dashboard/trends?days=0",
		"/api/slow-sql/fingerprints/1/trends?days=oops",
	}

	for _, path := range cases {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		recorder := httptest.NewRecorder()
		server.Handler().ServeHTTP(recorder, req)
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400 for %s, got %d", path, recorder.Code)
		}
	}
}
