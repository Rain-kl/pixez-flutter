package model

import "time"

const (
	BookmarkExportStatusRunning = "running"
	BookmarkExportStatusSuccess = "success"
	BookmarkExportStatusFailed  = "failed"
)

// BookmarkExportRun records one full bookmark export pass for a Pixiv user.
type BookmarkExportRun struct {
	ID             string     `gorm:"primaryKey;column:id" json:"id"`
	PixivUserID    string     `gorm:"column:pixiv_user_id;not null;index" json:"pixiv_user_id"`
	Restrict       string     `gorm:"column:restrict;not null;index" json:"restrict"`
	Status         string     `gorm:"column:status;not null;index" json:"status"`
	TotalCount     int        `gorm:"column:total_count;not null;default:0" json:"total_count"`
	NewCount       int        `gorm:"column:new_count;not null;default:0" json:"new_count"`
	UpdatedCount   int        `gorm:"column:updated_count;not null;default:0" json:"updated_count"`
	RemovedCount   int        `gorm:"column:removed_count;not null;default:0" json:"removed_count"`
	ErrorMessage   string     `gorm:"column:error_message" json:"error_message"`
	StartedAt      time.Time  `gorm:"column:started_at;not null" json:"started_at"`
	FinishedAt     *time.Time `gorm:"column:finished_at" json:"finished_at"`
	NextURL        string     `gorm:"column:next_url" json:"next_url"`
	LastRequestURL string     `gorm:"column:last_request_url" json:"last_request_url"`
	CreatedAt      time.Time  `gorm:"column:created_at" json:"created_at"`
	UpdatedAt      time.Time  `gorm:"column:updated_at" json:"updated_at"`
}

func (BookmarkExportRun) TableName() string { return "bookmark_export_runs" }

// BookmarkIllust stores the latest known full Pixiv bookmark illustration payload.
type BookmarkIllust struct {
	ID              uint       `gorm:"primaryKey;autoIncrement;column:id" json:"id"`
	PixivUserID     string     `gorm:"column:pixiv_user_id;not null;uniqueIndex:idx_bookmark_illust_user_restrict_illust" json:"pixiv_user_id"`
	Restrict        string     `gorm:"column:restrict;not null;uniqueIndex:idx_bookmark_illust_user_restrict_illust" json:"restrict"`
	IllustID        int64      `gorm:"column:illust_id;not null;uniqueIndex:idx_bookmark_illust_user_restrict_illust" json:"illust_id"`
	Title           string     `gorm:"column:title" json:"title"`
	Type            string     `gorm:"column:type" json:"type"`
	UserID          int64      `gorm:"column:user_id" json:"user_id"`
	UserName        string     `gorm:"column:user_name" json:"user_name"`
	PageCount       int        `gorm:"column:page_count" json:"page_count"`
	Width           int        `gorm:"column:width" json:"width"`
	Height          int        `gorm:"column:height" json:"height"`
	SanityLevel     int        `gorm:"column:sanity_level" json:"sanity_level"`
	XRestrict       int        `gorm:"column:x_restrict" json:"x_restrict"`
	TotalView       int        `gorm:"column:total_view" json:"total_view"`
	TotalBookmarks  int        `gorm:"column:total_bookmarks" json:"total_bookmarks"`
	Visible         bool       `gorm:"column:visible" json:"visible"`
	IsMuted         bool       `gorm:"column:is_muted" json:"is_muted"`
	IllustAIType    int        `gorm:"column:illust_ai_type" json:"illust_ai_type"`
	IllustJSON      string     `gorm:"column:illust_json;not null" json:"illust_json"`
	LastExportRunID string     `gorm:"column:last_export_run_id;not null;index" json:"last_export_run_id"`
	LastSeenAt      time.Time  `gorm:"column:last_seen_at;not null" json:"last_seen_at"`
	Removed         bool       `gorm:"column:removed;not null;default:false;index" json:"removed"`
	RemovedAt       *time.Time `gorm:"column:removed_at" json:"removed_at"`
	CreatedAt       time.Time  `gorm:"column:created_at" json:"created_at"`
	UpdatedAt       time.Time  `gorm:"column:updated_at" json:"updated_at"`
}

func (BookmarkIllust) TableName() string { return "bookmark_illusts" }
