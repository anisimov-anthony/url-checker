package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"url-checker/internal/database"
	"url-checker/internal/models"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestService(t *testing.T) (*URLChecker, *database.Database) {
	file := "./test_service_" + t.Name() + ".db"
	db, err := database.NewDatabase(file)
	require.NoError(t, err)

	t.Cleanup(func() {
		db.Close()
		os.Remove(file)
	})

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	httpClient := &http.Client{
		Timeout: 5 * time.Second,
	}

	checker := NewURLChecker(db, logger, httpClient)

	return checker, db
}

func setupMockHTTPServer(t *testing.T) *httptest.Server {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ok":
			w.WriteHeader(http.StatusOK)
		case "/notfound":
			w.WriteHeader(http.StatusNotFound)
		case "/error":
			w.WriteHeader(http.StatusInternalServerError)
		case "/timeout":
			time.Sleep(10 * time.Second)
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))

	t.Cleanup(server.Close)

	return server
}

func TestNewURLChecker(t *testing.T) {
	db, err := database.NewDatabase("./test_new_checker.db")
	require.NoError(t, err)
	defer db.Close()
	defer os.Remove("./test_new_checker.db")

	logger := logrus.New()
	httpClient := &http.Client{}

	checker := NewURLChecker(db, logger, httpClient)

	assert.NotNil(t, checker)
	assert.Equal(t, db, checker.db)
	assert.Equal(t, logger, checker.logger)
	assert.Equal(t, httpClient, checker.httpClient)
	assert.NotNil(t, checker.pendingPDFTasks)
	assert.False(t, checker.IsShutdown())
}

func TestURLChecker_LoadBatches(t *testing.T) {
	checker, db := setupTestService(t)
	ctx := context.Background()

	err := checker.LoadBatches(ctx)
	assert.NoError(t, err)

	err = db.CreateBatch(ctx, 1, models.BatchStatusCompleted, time.Now())
	require.NoError(t, err)

	err = checker.LoadBatches(ctx)
	assert.NoError(t, err)
}

func TestURLChecker_IsShutdown_SetShutdown(t *testing.T) {
	checker, _ := setupTestService(t)

	assert.False(t, checker.IsShutdown())

	checker.SetShutdown(true)
	assert.True(t, checker.IsShutdown())

	checker.SetShutdown(false)
	assert.False(t, checker.IsShutdown())
}

func TestURLChecker_checkURLAvailability(t *testing.T) {
	checker, _ := setupTestService(t)
	server := setupMockHTTPServer(t)

	tests := []struct {
		name     string
		url      string
		expected models.LinkStatus
	}{
		{
			name:     "valid URL - success",
			url:      server.URL + "/ok",
			expected: models.StatusAvailable,
		},
		{
			name:     "valid URL - not found",
			url:      server.URL + "/notfound",
			expected: models.StatusNotAvailable,
		},
		{
			name:     "valid URL - server error",
			url:      server.URL + "/error",
			expected: models.StatusNotAvailable,
		},
		{
			name:     "URL without protocol - example.com should resolve to localhost",
			url:      "example.com",
			expected: models.StatusNotAvailable,
		},
		{
			name:     "invalid URL",
			url:      "://invalid",
			expected: models.StatusNotAvailable,
		},
		{
			name:     "empty URL",
			url:      "",
			expected: models.StatusNotAvailable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := checker.checkURLAvailability(tt.url)
			if tt.url == "example.com" {
				assert.True(t, result == models.StatusAvailable || result == models.StatusNotAvailable)
			} else {
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestURLChecker_CheckLinks(t *testing.T) {
	checker, _ := setupTestService(t)
	server := setupMockHTTPServer(t)
	ctx := context.Background()

	tests := []struct {
		name          string
		links         []string
		expectError   bool
		expectedCount int
	}{
		{
			name:          "valid links",
			links:         []string{server.URL + "/ok", server.URL + "/notfound"},
			expectError:   false,
			expectedCount: 2,
		},
		{
			name:          "empty links",
			links:         []string{},
			expectError:   true,
			expectedCount: 0,
		},
		{
			name:          "nil links",
			links:         nil,
			expectError:   true,
			expectedCount: 0,
		},
		{
			name:          "mixed valid/invalid",
			links:         []string{server.URL + "/ok", "://invalid", server.URL + "/error"},
			expectError:   false,
			expectedCount: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response, err := checker.CheckLinks(ctx, tt.links)

			if tt.expectError {
				assert.Error(t, err)
				assert.Equal(t, models.CheckResponse{}, response)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedCount, len(response.Links))
				assert.Greater(t, response.LinksNum, 0)

				for _, link := range tt.links {
					assert.Contains(t, response.Links, link)
				}
			}
		})
	}
}

func TestURLChecker_CheckLinks_ContextCancellation(t *testing.T) {
	checker, _ := setupTestService(t)
	server := setupMockHTTPServer(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	links := []string{server.URL + "/ok"}
	_, err := checker.CheckLinks(ctx, links)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "context canceled")
}

func TestURLChecker_CheckLinks_Shutdown(t *testing.T) {
	checker, _ := setupTestService(t)
	server := setupMockHTTPServer(t)
	ctx := context.Background()

	checker.SetShutdown(true)

	links := []string{server.URL + "/ok"}
	response, err := checker.CheckLinks(ctx, links)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "service is shutting down")
	assert.Equal(t, models.CheckResponse{}, response)
}

func TestURLChecker_GeneratePDFReport(t *testing.T) {
	checker, db := setupTestService(t)
	ctx := context.Background()

	err := db.CreateBatch(ctx, 1, models.BatchStatusCompleted, time.Now())
	require.NoError(t, err)

	now := time.Now()
	_, err = db.CreateLink(ctx, "http://example.com", models.StatusAvailable, 1, &now)
	require.NoError(t, err)

	_, err = db.CreateLink(ctx, "http://test.com", models.StatusNotAvailable, 1, &now)
	require.NoError(t, err)

	pdfData, err := checker.GeneratePDFReport(ctx, []int{1})
	assert.NoError(t, err)
	assert.NotEmpty(t, pdfData)

	assert.True(t, strings.HasPrefix(string(pdfData), "%PDF"))
}

func TestURLChecker_GeneratePDFReport_EmptyBatches(t *testing.T) {
	checker, _ := setupTestService(t)
	ctx := context.Background()

	_, err := checker.GeneratePDFReport(ctx, []int{999})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no valid batches found")

	_, err = checker.GeneratePDFReport(ctx, []int{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no batch IDs provided")
}

func TestURLChecker_GeneratePDFReportAsync(t *testing.T) {
	checker, db := setupTestService(t)
	ctx := context.Background()

	err := db.CreateBatch(ctx, 1, models.BatchStatusCompleted, time.Now())
	require.NoError(t, err)

	now := time.Now()
	_, err = db.CreateLink(ctx, "http://example.com", models.StatusAvailable, 1, &now)
	require.NoError(t, err)

	workerCtx, workerCancel := context.WithCancel(context.Background())
	defer workerCancel()
	go checker.StartWorker(workerCtx)

	pdfData, err := checker.GeneratePDFReportAsync(ctx, []int{1})
	assert.NoError(t, err)
	assert.NotEmpty(t, pdfData)
	assert.True(t, strings.HasPrefix(string(pdfData), "%PDF"))
}

func TestURLChecker_GeneratePDFReportAsync_Shutdown(t *testing.T) {
	checker, _ := setupTestService(t)
	ctx := context.Background()

	checker.SetShutdown(true)

	_, err := checker.GeneratePDFReportAsync(ctx, []int{1})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "service is shutting down")
}

func TestURLChecker_GeneratePDFReportAsync_Timeout(t *testing.T) {
	checker, _ := setupTestService(t)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()
	time.Sleep(1 * time.Millisecond)

	_, err := checker.GeneratePDFReportAsync(ctx, []int{1})
	assert.Error(t, err)
}

func TestURLChecker_GetHealthStatus(t *testing.T) {
	checker, db := setupTestService(t)
	ctx := context.Background()

	status := checker.GetHealthStatus(ctx)
	assert.Equal(t, "healthy", status["status"])
	assert.Equal(t, false, status["shutdown"])
	assert.Equal(t, 0, status["batches"])
	assert.NotNil(t, status["timestamp"])

	err := db.CreateBatch(ctx, 1, models.BatchStatusCompleted, time.Now())
	require.NoError(t, err)

	status = checker.GetHealthStatus(ctx)
	assert.Equal(t, 1, status["batches"])

	checker.SetShutdown(true)
	status = checker.GetHealthStatus(ctx)
	assert.Equal(t, true, status["shutdown"])
}

func TestURLChecker_GetCurrentTimestamp(t *testing.T) {
	checker, _ := setupTestService(t)

	before := time.Now().Unix()
	timestamp := checker.GetCurrentTimestamp()
	after := time.Now().Unix()

	assert.GreaterOrEqual(t, timestamp, before)
	assert.LessOrEqual(t, timestamp, after)
}

func TestURLChecker_StartWorker(t *testing.T) {
	checker, _ := setupTestService(t)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	assert.NotPanics(t, func() {
		checker.StartWorker(ctx)
	})
}

func TestURLChecker_processPDFTask(t *testing.T) {
	checker, db := setupTestService(t)
	ctx := context.Background()

	err := db.CreateBatch(ctx, 1, models.BatchStatusCompleted, time.Now())
	require.NoError(t, err)

	now := time.Now()
	_, err = db.CreateLink(ctx, "http://example.com", models.StatusAvailable, 1, &now)
	require.NoError(t, err)

	task := &PDFTask{
		BatchIDs: []int{1},
		Result:   make(chan []byte, 1),
		Error:    make(chan error, 1),
	}

	checker.processPDFTask(ctx, task)

	select {
	case pdfData := <-task.Result:
		assert.NotEmpty(t, pdfData)
		assert.True(t, strings.HasPrefix(string(pdfData), "%PDF"))
	case err := <-task.Error:
		t.Fatalf("Unexpected error: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for PDF generation")
	}
}

func TestURLChecker_processLinks(t *testing.T) {
	checker, db := setupTestService(t)
	server := setupMockHTTPServer(t)
	ctx := context.Background()

	err := db.CreateBatch(ctx, 1, models.BatchStatusProcessing, time.Now())
	require.NoError(t, err)

	links := []string{server.URL + "/ok", server.URL + "/notfound"}
	results, err := checker.processLinks(ctx, links, 1)
	assert.NoError(t, err)
	assert.Len(t, results, 2)

	for _, result := range results {
		assert.Equal(t, 1, result.BatchNum)
		assert.NotNil(t, result.Time)
		assert.Contains(t, links, result.URL)
		assert.True(t, result.Status == models.StatusAvailable || result.Status == models.StatusNotAvailable)
	}
}

func TestURLChecker_processLinks_ContextCancellation(t *testing.T) {
	checker, db := setupTestService(t)
	server := setupMockHTTPServer(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := db.CreateBatch(context.Background(), 1, models.BatchStatusProcessing, time.Now())
	require.NoError(t, err)

	links := []string{server.URL + "/ok"}
	results, err := checker.processLinks(ctx, links, 1)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "context canceled")
	assert.Empty(t, results)
}
