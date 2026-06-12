package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"slow-sql-observer/internal/model"
)

type QueryService interface {
	GetOverview(ctx context.Context, params model.OverviewParams) (model.Overview, error)
	GetDashboardTrends(ctx context.Context, params model.TrendParams) (model.DashboardTrends, error)
	ListFingerprints(ctx context.Context, params model.ListFingerprintsParams) (model.PaginatedFingerprints, error)
	GetFingerprint(ctx context.Context, id int64, params model.GetFingerprintParams) (*model.FingerprintRecordView, error)
	GetFingerprintTrends(ctx context.Context, id int64, params model.TrendParams) (model.FingerprintTrends, error)
	ListFingerprintRecords(ctx context.Context, fingerprintID int64, params model.ListFingerprintRecordsParams) (model.PaginatedRecords, error)
	GetSource(ctx context.Context) (*model.Source, error)
	GetCollectorStatus(ctx context.Context) (*model.CollectorStatus, error)
	GetAcquisitionStatus(ctx context.Context) (*model.AcquisitionStatus, error)
	GetDiscovery(ctx context.Context, sourceID int64) (*model.SourceDiscovery, error)
	GetSourceID(ctx context.Context) (int64, error)
}

type Server struct {
	store                  QueryService
	webDir                 string
	defaultMinQueryTimeSec float64
}

func NewServer(store QueryService, webDir string, defaultMinQueryTimeSec float64) *Server {
	return &Server{
		store:                  store,
		webDir:                 webDir,
		defaultMinQueryTimeSec: normalizeMinQueryTimeSec(defaultMinQueryTimeSec),
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/dashboard/overview", s.handleOverview)
	mux.HandleFunc("/api/dashboard/trends", s.handleDashboardTrends)
	mux.HandleFunc("/api/source", s.handleSource)
	mux.HandleFunc("/api/collector/status", s.handleCollectorStatus)
	mux.HandleFunc("/api/acquisition/status", s.handleAcquisitionStatus)
	mux.HandleFunc("/api/discovery/status", s.handleDiscoveryStatus)
	mux.HandleFunc("/api/slow-sql/fingerprints/", s.handleFingerprintSubroutes)
	mux.HandleFunc("/api/slow-sql/fingerprints", s.handleFingerprintList)
	fileServer := http.FileServer(http.Dir(s.webDir))
	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}
		if r.URL.Path == "/" {
			indexPath := filepath.Join(s.webDir, "index.html")
			if _, err := os.Stat(indexPath); err != nil {
				writeError(w, http.StatusInternalServerError, err)
				return
			}
			http.ServeFile(w, r, indexPath)
			return
		}
		fileServer.ServeHTTP(w, r)
	}))
	return mux
}

func (s *Server) handleOverview(w http.ResponseWriter, r *http.Request) {
	overview, err := s.store.GetOverview(r.Context(), model.OverviewParams{
		MinQueryTimeSec: s.resolveMinQueryTimeSec(r),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, overview)
}

func (s *Server) handleDashboardTrends(w http.ResponseWriter, r *http.Request) {
	params, err := s.resolveTrendParams(r, true)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	trends, err := s.store.GetDashboardTrends(r.Context(), params)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, trends)
}

func (s *Server) handleSource(w http.ResponseWriter, r *http.Request) {
	source, err := s.store.GetSource(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, source)
}

func (s *Server) handleCollectorStatus(w http.ResponseWriter, r *http.Request) {
	status, err := s.store.GetCollectorStatus(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (s *Server) handleAcquisitionStatus(w http.ResponseWriter, r *http.Request) {
	status, err := s.store.GetAcquisitionStatus(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (s *Server) handleDiscoveryStatus(w http.ResponseWriter, r *http.Request) {
	sourceID, err := s.store.GetSourceID(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	disc, err := s.store.GetDiscovery(r.Context(), sourceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if disc == nil {
		writeJSON(w, http.StatusOK, map[string]string{
			"discoveryState": "unknown",
			"message":        "no discovery data available; source may not be in mysql_auto mode",
		})
		return
	}
	writeJSON(w, http.StatusOK, disc)
}

func (s *Server) handleFingerprintList(w http.ResponseWriter, r *http.Request) {
	params := model.ListFingerprintsParams{
		Page:            parseInt(r.URL.Query().Get("page"), 1),
		PageSize:        parseInt(r.URL.Query().Get("pageSize"), 20),
		SortBy:          r.URL.Query().Get("sortBy"),
		SortOrder:       r.URL.Query().Get("sortOrder"),
		DBName:          r.URL.Query().Get("dbName"),
		SQLType:         r.URL.Query().Get("sqlType"),
		Keyword:         r.URL.Query().Get("keyword"),
		MinQueryTimeSec: s.resolveMinQueryTimeSec(r),
	}
	response, err := s.store.ListFingerprints(r.Context(), params)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *Server) handleFingerprintSubroutes(w http.ResponseWriter, r *http.Request) {
	cleanPath := strings.TrimPrefix(path.Clean(r.URL.Path), "/api/slow-sql/fingerprints/")
	parts := strings.Split(cleanPath, "/")
	if len(parts) == 0 || parts[0] == "." || parts[0] == "" {
		http.NotFound(w, r)
		return
	}

	id, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if len(parts) > 1 && parts[1] == "records" {
		s.handleFingerprintRecords(w, r, id)
		return
	}
	if len(parts) > 1 && parts[1] == "trends" {
		s.handleFingerprintTrends(w, r, id)
		return
	}
	s.handleFingerprintDetail(w, r, id)
}

func (s *Server) handleFingerprintDetail(w http.ResponseWriter, r *http.Request, id int64) {
	view, err := s.store.GetFingerprint(r.Context(), id, model.GetFingerprintParams{
		MinQueryTimeSec: s.resolveMinQueryTimeSec(r),
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, err)
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func (s *Server) handleFingerprintTrends(w http.ResponseWriter, r *http.Request, id int64) {
	params, err := s.resolveTrendParams(r, false)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	trends, err := s.store.GetFingerprintTrends(r.Context(), id, params)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, err)
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, trends)
}

func (s *Server) handleFingerprintRecords(w http.ResponseWriter, r *http.Request, id int64) {
	params := model.ListFingerprintRecordsParams{
		Page:            parseInt(r.URL.Query().Get("page"), 1),
		PageSize:        parseInt(r.URL.Query().Get("pageSize"), 20),
		SortBy:          r.URL.Query().Get("sortBy"),
		SortOrder:       r.URL.Query().Get("sortOrder"),
		MinQueryTimeSec: s.resolveMinQueryTimeSec(r),
	}
	response, err := s.store.ListFingerprintRecords(r.Context(), id, params)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]any{
		"error": err.Error(),
	})
}

func parseInt(value string, fallback int) int {
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func (s *Server) resolveMinQueryTimeSec(r *http.Request) float64 {
	raw := strings.TrimSpace(r.URL.Query().Get("minQueryTimeSec"))
	if raw == "" {
		return s.defaultMinQueryTimeSec
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return s.defaultMinQueryTimeSec
	}
	return normalizeMinQueryTimeSec(value)
}

func (s *Server) resolveTrendParams(r *http.Request, allowDBName bool) (model.TrendParams, error) {
	bucket := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("bucket")))
	if bucket == "" {
		bucket = model.TrendBucketDay
	}
	if bucket != model.TrendBucketDay && bucket != model.TrendBucketHour {
		return model.TrendParams{}, fmt.Errorf("unsupported bucket %q; supported values are %q and %q", bucket, model.TrendBucketDay, model.TrendBucketHour)
	}

	daysRaw := strings.TrimSpace(r.URL.Query().Get("days"))
	days := 7
	if daysRaw != "" {
		parsed, err := strconv.Atoi(daysRaw)
		if err != nil {
			return model.TrendParams{}, fmt.Errorf("invalid days value %q", daysRaw)
		}
		days = parsed
	}
	if days <= 0 || days > 30 {
		return model.TrendParams{}, fmt.Errorf("unsupported days %d; supported range is 1-30", days)
	}
	if bucket == model.TrendBucketHour && days > 7 {
		return model.TrendParams{}, fmt.Errorf("unsupported days %d for hourly trends; supported range is 1-7", days)
	}

	params := model.TrendParams{
		Bucket:          bucket,
		Days:            days,
		MinQueryTimeSec: s.resolveMinQueryTimeSec(r),
	}
	if allowDBName {
		params.DBName = strings.TrimSpace(r.URL.Query().Get("dbName"))
	}
	return params, nil
}

func normalizeMinQueryTimeSec(value float64) float64 {
	if value <= 0 {
		return 0
	}
	return value
}
