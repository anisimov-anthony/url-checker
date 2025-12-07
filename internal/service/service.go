package service

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"url-checker/internal/database"
	"url-checker/internal/models"

	"github.com/jung-kurt/gofpdf"
	"github.com/sirupsen/logrus"
)

type URLChecker struct {
	db              *database.Database
	logger          *logrus.Logger
	pendingPDFTasks chan *PDFTask
	httpClient      *http.Client
	shutdown        bool
	shutdownMux     sync.RWMutex
}

type PDFTask struct {
	BatchIDs []int
	Result   chan []byte
	Error    chan error
}

func NewURLChecker(db *database.Database, logger *logrus.Logger, httpClient *http.Client) *URLChecker {
	return &URLChecker{
		db:              db,
		logger:          logger,
		pendingPDFTasks: make(chan *PDFTask, 10),
		httpClient:      httpClient,
	}
}

func (urlchecker *URLChecker) LoadBatches(ctx context.Context) error {
	maxID, err := urlchecker.db.GetMaxBatchNum(ctx)
	if err != nil {
		return fmt.Errorf("failed to get max batch num: %w", err)
	}

	urlchecker.logger.Infof("Database loaded, max batch num: %d", maxID)
	return nil
}

func (urlchecker *URLChecker) IsShutdown() bool {
	urlchecker.shutdownMux.RLock()
	defer urlchecker.shutdownMux.RUnlock()
	return urlchecker.shutdown
}

func (urlchecker *URLChecker) SetShutdown(shutdown bool) {
	urlchecker.shutdownMux.Lock()
	defer urlchecker.shutdownMux.Unlock()
	urlchecker.shutdown = shutdown
}

func (urlchecker *URLChecker) getNextID(ctx context.Context) (int, error) {
	maxID, err := urlchecker.db.GetMaxBatchNum(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to get max batch num: %w", err)
	}
	return maxID + 1, nil
}

func (urlchecker *URLChecker) checkURLAvailability(rawURL string) models.LinkStatus {
	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		rawURL = "http://" + rawURL
	}

	parsedURL, err := url.Parse(rawURL)
	if err != nil || parsedURL.Host == "" {
		urlchecker.logger.Warnf("Invalid URL %s: %v", rawURL, err)
		return models.StatusNotAvailable
	}

	req, err := http.NewRequest("GET", rawURL, nil)
	if err != nil {
		urlchecker.logger.Warnf("Failed to create request for %s: %v", rawURL, err)
		return models.StatusNotAvailable
	}

	req.Header.Set("User-Agent", "URL-Checker/1.0")

	resp, err := urlchecker.httpClient.Do(req)
	if err != nil {
		urlchecker.logger.Warnf("Failed to fetch %s: %v", rawURL, err)
		return models.StatusNotAvailable
	}
	defer resp.Body.Close()

	urlchecker.logger.Infof("URL %s returned status %d", rawURL, resp.StatusCode)
	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		return models.StatusAvailable
	}

	return models.StatusNotAvailable
}

func (urlchecker *URLChecker) processLinks(ctx context.Context, links []string, batchNum int) ([]*models.Link, error) {
	var linkIDs []int
	for _, link := range links {
		linkID, err := urlchecker.db.CreateLink(ctx, link, models.StatusProcessing, batchNum, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create link for %s: %w", link, err)
		}
		linkIDs = append(linkIDs, linkID)
	}

	results := make([]*models.Link, len(links))
	var wg sync.WaitGroup
	var resultsMux sync.Mutex

	for i, link := range links {
		wg.Add(1)
		go func(idx int, l string, linkID int) {
			defer wg.Done()

			select {
			case <-ctx.Done():
				return
			default:
			}

			status := urlchecker.checkURLAvailability(l)
			processedAt := time.Now()

			var time *time.Time
			if status == models.StatusAvailable || status == models.StatusNotAvailable {
				time = &processedAt
			}

			select {
			case <-ctx.Done():
				return
			default:
			}

			if err := urlchecker.db.UpdateLinkStatus(ctx, linkID, status, time); err != nil {
				urlchecker.logger.Errorf("Failed to update link status for %s: %v", l, err)
			}

			resultsMux.Lock()
			results[idx] = &models.Link{
				ID:       linkID,
				URL:      l,
				Status:   status,
				BatchNum: batchNum,
				Time:     time,
			}
			resultsMux.Unlock()
		}(i, link, linkIDs[i])
	}

	wg.Wait()

	if err := urlchecker.db.UpdateBatchStatus(ctx, batchNum, models.BatchStatusCompleted); err != nil {
		urlchecker.logger.Errorf("Failed to update batch status: %v", err)
	}

	return results, nil
}

func (urlchecker *URLChecker) StartWorker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			urlchecker.logger.Info("PDF worker shutting down...")
			return
		case task := <-urlchecker.pendingPDFTasks:
			if task != nil {
				urlchecker.processPDFTask(ctx, task)
			}
		}
	}
}

func (urlchecker *URLChecker) processPDFTask(ctx context.Context, task *PDFTask) {
	pdfData, err := urlchecker.GeneratePDFReport(ctx, task.BatchIDs)
	if err != nil {
		task.Error <- err
	} else {
		task.Result <- pdfData
	}
}

func (urlchecker *URLChecker) CheckLinks(ctx context.Context, links []string) (models.CheckResponse, error) {
	if len(links) == 0 {
		return models.CheckResponse{}, fmt.Errorf("no links provided")
	}

	batchNum, err := urlchecker.getNextID(ctx)
	if err != nil {
		return models.CheckResponse{}, fmt.Errorf("failed to get next batch ID: %w", err)
	}

	if err := urlchecker.db.CreateBatch(ctx, batchNum, models.BatchStatusProcessing, time.Now()); err != nil {
		return models.CheckResponse{}, fmt.Errorf("failed to create batch: %w", err)
	}

	processedLinks, err := urlchecker.processLinks(ctx, links, batchNum)
	if err != nil {
		urlchecker.db.UpdateBatchStatus(ctx, batchNum, models.BatchStatusFailed)
		return models.CheckResponse{}, fmt.Errorf("failed to process links: %w", err)
	}

	resultLinks := make(map[string]string)
	for _, link := range processedLinks {
		resultLinks[link.URL] = string(link.Status)
	}

	response := models.CheckResponse{
		Links:    resultLinks,
		LinksNum: batchNum,
	}

	return response, nil
}

func (urlchecker *URLChecker) GetBatchStatus(ctx context.Context, id int) (models.CheckResponse, error) {
	links, err := urlchecker.db.GetLinksByBatchNum(ctx, id)
	if err != nil {
		return models.CheckResponse{}, fmt.Errorf("batch not found")
	}

	resultLinks := make(map[string]string)
	for _, link := range links {
		resultLinks[link.URL] = string(link.Status)
	}

	response := models.CheckResponse{
		Links:    resultLinks,
		LinksNum: id,
	}

	return response, nil
}

func (urlchecker *URLChecker) GeneratePDFReportAsync(ctx context.Context, batchIDs []int) ([]byte, error) {
	if urlchecker.IsShutdown() {
		return nil, fmt.Errorf("service is shutting down")
	}

	task := &PDFTask{
		BatchIDs: batchIDs,
		Result:   make(chan []byte, 1),
		Error:    make(chan error, 1),
	}

	select {
	case urlchecker.pendingPDFTasks <- task:
		urlchecker.logger.Infof("Queued PDF task for batches %v", batchIDs)

		select {
		case pdfData := <-task.Result:
			return pdfData, nil
		case err := <-task.Error:
			return nil, err
		case <-time.After(30 * time.Second):
			return nil, fmt.Errorf("PDF generation timeout")
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	default:
		urlchecker.logger.Warnf("PDF queue full, generating report synchronously for batches %v", batchIDs)
		return urlchecker.GeneratePDFReport(ctx, batchIDs)
	}
}

func (urlchecker *URLChecker) GeneratePDFReport(ctx context.Context, batchIDs []int) ([]byte, error) {
	batches, links, err := urlchecker.db.GetBatchesByIDs(ctx, batchIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to get batches data: %w", err)
	}

	if len(batches) == 0 {
		return nil, fmt.Errorf("no valid batches found")
	}

	batchLinks := make(map[int][]*models.Link)
	for _, link := range links {
		batchLinks[link.BatchNum] = append(batchLinks[link.BatchNum], link)
	}

	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()
	pdf.SetFont("Arial", "B", 16)
	pdf.Cell(40, 10, "URL Availability Report")
	pdf.Ln(15)

	pdf.SetFont("Arial", "", 12)
	pdf.Cell(40, 10, fmt.Sprintf("Generated: %s", time.Now().Format("2006-01-02 15:04:05")))
	pdf.Ln(15)

	for _, batch := range batches {
		pdf.SetFont("Arial", "B", 14)
		pdf.Cell(40, 10, fmt.Sprintf("link_num #%d (%s)", batch.LinksNum, batch.Status))
		pdf.Ln(10)

		pdf.SetFont("Arial", "", 10)
		pdf.Cell(40, 10, fmt.Sprintf("Created: %s", batch.CreatedAt.Format("2006-01-02 15:04:05")))
		pdf.Ln(8)

		if batchLinks, exists := batchLinks[batch.LinksNum]; exists {
			for _, link := range batchLinks {
				statusText := string(link.Status)
				if link.Status == models.StatusAvailable {
					statusText = "Available"
				} else {
					statusText = "Not Available"
				}

				pdf.Cell(40, 8, fmt.Sprintf("- %s: %s", link.URL, statusText))
				pdf.Ln(6)
			}
		}
		pdf.Ln(10)
	}

	var buf bytes.Buffer
	err = pdf.Output(&buf)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func (urlchecker *URLChecker) GetHealthStatus(ctx context.Context) map[string]any {
	batches, err := urlchecker.db.GetAllBatches(ctx)
	batchCount := 0
	if err == nil {
		batchCount = len(batches)
	}

	return map[string]any{
		"status":    "healthy",
		"shutdown":  urlchecker.IsShutdown(),
		"batches":   batchCount,
		"timestamp": time.Now().Unix(),
	}
}

func (urlchecker *URLChecker) GetCurrentTimestamp() int64 {
	return time.Now().Unix()
}
