package model

// APIResponse describes the standard PixEz Sync API response envelope.
type APIResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// MessageResponse describes a standard response without a data payload.
type MessageResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// ErrorResponse describes a standard error response.
type ErrorResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// BasicErrorResponse describes raw mirror endpoint errors.
type BasicErrorResponse struct {
	Error string `json:"error"`
}

// PingData is returned by the health check endpoint.
type PingData struct {
	Status string `json:"status"`
}

// UserDataHashes maps sync table names to their checksum values.
type UserDataHashes map[string]string

// MirrorStatusResponse describes an illustration mirror task status.
type MirrorStatusResponse struct {
	TaskID          string `json:"task_id"`
	IllustID        int64  `json:"illust_id"`
	Status          string `json:"status"`
	Mirrored        bool   `json:"mirrored"`
	TotalCount      int    `json:"total_count"`
	SuccessCount    int    `json:"success_count"`
	FailedCount     int    `json:"failed_count"`
	RequestURLsJSON string `json:"request_urls_json"`
	RetryURLsJSON   string `json:"retry_urls_json"`
}

// BookmarkExportRunResponse describes a bookmark export pass.
type BookmarkExportRunResponse BookmarkExportRun

// BookmarkIllustResponse describes one exported bookmark illustration record.
type BookmarkIllustResponse BookmarkIllust
