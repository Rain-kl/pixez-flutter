package model

import "time"

const (
	ScheduledTaskBookmarkExport         = "bookmark_export"
	ScheduledTaskBookmarkMirrorNew      = "bookmark_mirror_new"
	ScheduledTaskBookmarkMirrorRetry    = "bookmark_mirror_retry"
	ScheduledTaskBookmarkMirrorRetryImg = "bookmark_mirror_retry_img"
	ScheduledTaskNovelBookmarkExport    = "novel_bookmark_export"

	ScheduledTaskStatusRunning = "running"
	ScheduledTaskStatusSuccess = "success"
	ScheduledTaskStatusFailed  = "failed"
)

type ScheduledTask struct {
	Name            string     `gorm:"primaryKey;column:name" json:"name"`
	Enabled         bool       `gorm:"column:enabled;not null;default:true" json:"enabled"`
	IntervalSeconds int        `gorm:"column:interval_seconds;not null" json:"interval_seconds"`
	LastRunAt       *time.Time `gorm:"column:last_run_at" json:"last_run_at"`
	NextRunAt       *time.Time `gorm:"column:next_run_at" json:"next_run_at"`
	LastDurationMs  int64      `gorm:"column:last_duration_ms;not null;default:0" json:"last_duration_ms"`
	LastStatus      string     `gorm:"column:last_status" json:"last_status"`
	LastError       string     `gorm:"column:last_error" json:"last_error"`
	CreatedAt       time.Time  `gorm:"column:created_at" json:"created_at"`
	UpdatedAt       time.Time  `gorm:"column:updated_at" json:"updated_at"`
}

func (ScheduledTask) TableName() string { return "scheduled_tasks" }
