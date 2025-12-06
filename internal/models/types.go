package models

import "time"

type CheckRequest struct {
	Links []string `json:"links"`
}

type CheckResponse struct {
	Links    map[string]string `json:"links"`
	LinksNum int               `json:"links_num"`
}

type ReportRequest struct {
	LinksList []int `json:"links_list"`
}

type LinkStatus string

const (
	StatusAvailable    LinkStatus = "available"
	StatusNotAvailable LinkStatus = "not available"
	StatusProcessing   LinkStatus = "processing"
)

type BatchStatus string

const (
	BatchStatusProcessing BatchStatus = "processing"
	BatchStatusCompleted  BatchStatus = "completed"
	BatchStatusFailed     BatchStatus = "failed"
)

type Link struct {
	ID       int        `json:"id"`
	URL      string     `json:"url"`
	Status   LinkStatus `json:"status"`
	BatchNum int        `json:"batch_num"`
	Time     *time.Time `json:"time"`
}

type Batch struct {
	LinksNum  int         `json:"links_num"`
	Status    BatchStatus `json:"status"`
	CreatedAt time.Time   `json:"created_at"`
}
