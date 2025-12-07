package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"url-checker/internal/database"
	"url-checker/internal/models"
	"url-checker/internal/service"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupSimpleTestHandler(t *testing.T) (*Handler, *service.URLChecker, *database.Database) {
	file := "./test_simple_" + t.Name() + ".db"
	db, err := database.NewDatabase(file)
	require.NoError(t, err)

	t.Cleanup(func() {
		db.Close()
	})

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	httpClient := &http.Client{
		Timeout: 5 * time.Second,
	}

	checker := service.NewURLChecker(db, logger, httpClient)
	handler := NewHandler(checker, logger)

	return handler, checker, db
}

func TestHandler_Simple_CheckLinksHandler(t *testing.T) {
	handler, checker, _ := setupSimpleTestHandler(t)

	ctx := context.Background()
	err := checker.LoadBatches(ctx)
	require.NoError(t, err)

	requestBody := models.CheckRequest{
		Links: []string{"http://example.com"},
	}

	jsonData, err := json.Marshal(requestBody)
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/api/check", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.CheckLinksHandler(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response models.CheckResponse
	err = json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.NotEmpty(t, response.Links)
}

func TestHandler_Simple_CheckLinksHandler_EmptyLinks(t *testing.T) {
	handler, _, _ := setupSimpleTestHandler(t)

	requestBody := models.CheckRequest{
		Links: []string{},
	}

	jsonData, err := json.Marshal(requestBody)
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/api/check", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.CheckLinksHandler(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandler_Simple_HealthHandler(t *testing.T) {
	handler, _, _ := setupSimpleTestHandler(t)

	req := httptest.NewRequest("GET", "/api/health", nil)
	w := httptest.NewRecorder()

	handler.HealthHandler(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var response map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "healthy", response["status"])
}

func TestHandler_Simple_SetupRoutes(t *testing.T) {
	handler, _, _ := setupSimpleTestHandler(t)

	router := handler.SetupRoutes()
	assert.NotNil(t, router)

	req := httptest.NewRequest("GET", "/api/health", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	req = httptest.NewRequest("GET", "/api/invalid", nil)
	w = httptest.NewRecorder()

	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestHandler_Simple_ReportHandler(t *testing.T) {
	handler, _, _ := setupSimpleTestHandler(t)

	requestBody := models.ReportRequest{
		LinksList: []int{},
	}

	jsonData, err := json.Marshal(requestBody)
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/api/report", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ReportHandler(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandler_Simple_ReportHandler_NilBatches(t *testing.T) {
	handler, _, _ := setupSimpleTestHandler(t)

	requestBody := models.ReportRequest{
		LinksList: nil,
	}

	jsonData, err := json.Marshal(requestBody)
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/api/report", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ReportHandler(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandler_Simple_ReportHandler_InvalidJSON(t *testing.T) {
	handler, _, _ := setupSimpleTestHandler(t)

	req := httptest.NewRequest("POST", "/api/report", bytes.NewBufferString("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ReportHandler(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandler_Simple_ReportHandler_EmptyBody(t *testing.T) {
	handler, _, _ := setupSimpleTestHandler(t)

	req := httptest.NewRequest("POST", "/api/report", nil)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ReportHandler(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandler_Simple_CheckLinksHandler_InvalidJSON(t *testing.T) {
	handler, _, _ := setupSimpleTestHandler(t)

	req := httptest.NewRequest("POST", "/api/check", bytes.NewBufferString("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.CheckLinksHandler(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandler_Simple_CheckLinksHandler_EmptyBody(t *testing.T) {
	handler, _, _ := setupSimpleTestHandler(t)

	req := httptest.NewRequest("POST", "/api/check", nil)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.CheckLinksHandler(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandler_Simple_CheckLinksHandler_NilLinks(t *testing.T) {
	handler, _, _ := setupSimpleTestHandler(t)

	requestBody := models.CheckRequest{
		Links: nil,
	}

	jsonData, err := json.Marshal(requestBody)
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/api/check", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.CheckLinksHandler(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandler_Simple_CheckLinksHandler_Shutdown(t *testing.T) {
	handler, checker, _ := setupSimpleTestHandler(t)

	checker.SetShutdown(true)

	requestBody := models.CheckRequest{
		Links: []string{"http://example.com"},
	}

	jsonData, err := json.Marshal(requestBody)
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/api/check", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.CheckLinksHandler(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestHandler_Simple_ReportHandler_Shutdown(t *testing.T) {
	handler, checker, _ := setupSimpleTestHandler(t)

	checker.SetShutdown(true)

	requestBody := models.ReportRequest{
		LinksList: []int{1},
	}

	jsonData, err := json.Marshal(requestBody)
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/api/report", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ReportHandler(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestHandler_Simple_SetupRoutes_Methods(t *testing.T) {
	handler, _, _ := setupSimpleTestHandler(t)

	router := handler.SetupRoutes()
	assert.NotNil(t, router)

	req := httptest.NewRequest("POST", "/api/health", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)

	req = httptest.NewRequest("GET", "/api/check", nil)
	w = httptest.NewRecorder()

	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)

	req = httptest.NewRequest("GET", "/api/report", nil)
	w = httptest.NewRecorder()

	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestHandler_Simple_ReportHandler_NonExistentBatches(t *testing.T) {
	handler, _, _ := setupSimpleTestHandler(t)

	requestBody := models.ReportRequest{
		LinksList: []int{999},
	}

	jsonData, err := json.Marshal(requestBody)
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/api/report", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ReportHandler(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandler_Simple_ReportHandler_WithContextCancellation(t *testing.T) {
	handler, _, _ := setupSimpleTestHandler(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	requestBody := models.ReportRequest{
		LinksList: []int{1},
	}

	jsonData, err := json.Marshal(requestBody)
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/api/report", bytes.NewBuffer(jsonData))
	req = req.WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ReportHandler(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
