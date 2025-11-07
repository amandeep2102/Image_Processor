package models

import "time"

type Image struct {
	ID           string    `json:"id"`
	Filename     string    `json:"filename"`
	OriginalPath string    `json:"original_path"`
	ContentType  string    `json:"content_type"`
	SizeBytes    int64     `json:"size_bytes"`
	Width        int       `json:"width"`
	Height       int       `json:"height"`
	UploadedBy   string    `json:"uploaded_by"`
	UploadedAt   time.Time `json:"uploaded_at"`
	Status       string    `json:"status"`
}

type ProcessedImage struct {
	ID               string                 `json:"id"`
	OriginalImageID  string                 `json:"original_image_id"`
	OperationType    string                 `json:"operation_type"`
	ProcessedPath    string                 `json:"processed_path"`
	Parameters       map[string]interface{} `json:"parameters"`
	ProcessingTimeMs int                    `json:"processing_time_ms"`
	CreatedAt        time.Time              `json:"created_at"`
}

type ProcessRequest struct {
	ImageID    string                 `json:"image_id"`
	Operation  string                 `json:"operation"`
	Parameters map[string]interface{} `json:"parameters"`
}

type ProcessResponse struct {
	Success          bool   `json:"success"`
	ProcessedID      string `json:"processed_id,omitempty"`
	Message          string `json:"message,omitempty"`
	ProcessingTimeMs int    `json:"processing_time_ms,omitempty"`
}
