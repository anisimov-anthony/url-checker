package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"url-checker/internal/models"
	"url-checker/internal/service"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

type Handler struct {
	service *service.URLChecker
	logger  *logrus.Logger
}

func NewHandler(service *service.URLChecker, logger *logrus.Logger) *Handler {
	return &Handler{
		service: service,
		logger:  logger,
	}
}

func (h *Handler) CheckLinksHandler(w http.ResponseWriter, r *http.Request) {
	if h.service.IsShutdown() {
		http.Error(w, "Service is shutting down", http.StatusServiceUnavailable)
		return
	}

	var req models.CheckRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if len(req.Links) == 0 {
		http.Error(w, "No links provided", http.StatusBadRequest)
		return
	}

	response, err := h.service.CheckLinks(req.Links)
	if err != nil {
		if err.Error() == "no links provided" {
			http.Error(w, "No links provided", http.StatusBadRequest)
		} else {
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (h *Handler) GetBatchStatusHandler(w http.ResponseWriter, r *http.Request) {
	if h.service.IsShutdown() {
		http.Error(w, "Service is shutting down", http.StatusServiceUnavailable)
		return
	}

	vars := mux.Vars(r)
	idStr := vars["id"]

	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid batch ID", http.StatusBadRequest)
		return
	}

	response, err := h.service.GetBatchStatus(id)
	if err != nil {
		if err.Error() == "batch not found" {
			http.Error(w, "Batch not found", http.StatusNotFound)
		} else {
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (h *Handler) ReportHandler(w http.ResponseWriter, r *http.Request) {
	if h.service.IsShutdown() {
		http.Error(w, "Service is shutting down", http.StatusServiceUnavailable)
		return
	}

	var req models.ReportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if len(req.LinksList) == 0 {
		http.Error(w, "No batch IDs provided", http.StatusBadRequest)
		return
	}

	pdfData, err := h.service.GeneratePDFReportAsync(req.LinksList)
	if err != nil {
		h.logger.Errorf("Failed to generate PDF: %v", err)
		http.Error(w, "Failed to generate report", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=url_report_%d.pdf", h.service.GetCurrentTimestamp()))
	w.Write(pdfData)
}

func (h *Handler) HealthHandler(w http.ResponseWriter, r *http.Request) {
	status := h.service.GetHealthStatus()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

func (h *Handler) SetupRoutes() *mux.Router {
	router := mux.NewRouter()

	api := router.PathPrefix("/api").Subrouter()
	api.HandleFunc("/check", h.CheckLinksHandler).Methods("POST")
	api.HandleFunc("/report", h.ReportHandler).Methods("POST")
	api.HandleFunc("/batch/{id}", h.GetBatchStatusHandler).Methods("GET")
	api.HandleFunc("/health", h.HealthHandler).Methods("GET")

	return router
}
