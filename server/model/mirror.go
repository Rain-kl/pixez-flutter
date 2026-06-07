package model

import "time"

const (
	MirrorTaskTypeIllust = "illust_mirror"
	MirrorTargetIllust   = "illust"

	MirrorTaskTypeNovel = "novel_mirror"
	MirrorTargetNovel   = "novel"

	MirrorTaskStatusQueued     = "queued"
	MirrorTaskStatusProcessing = "processing"
	MirrorTaskStatusSuccess    = "success"
	MirrorTaskStatusFailed     = "failed"
)

type MirrorTask struct {
	ID                 string     `gorm:"primaryKey;column:id" json:"id"`
	TaskType           string     `gorm:"column:task_type;not null" json:"task_type"`
	TargetType         string     `gorm:"column:target_type;not null;uniqueIndex:idx_mirror_task_target_id" json:"target_type"`
	TargetID           int64      `gorm:"column:target_id;not null;uniqueIndex:idx_mirror_task_target_id" json:"target_id"`
	Status             string     `gorm:"column:status;not null;index" json:"status"`
	RequestPayloadJSON string     `gorm:"column:request_payload_json" json:"request_payload_json"`
	RequestURLsJSON    string     `gorm:"column:request_urls_json" json:"request_urls_json"`
	RetryURLsJSON      string     `gorm:"column:retry_urls_json" json:"retry_urls_json"`
	ErrorMessage       string     `gorm:"column:error_message" json:"error_message"`
	TotalCount         int        `gorm:"column:total_count;not null;default:0" json:"total_count"`
	SuccessCount       int        `gorm:"column:success_count;not null;default:0" json:"success_count"`
	FailedCount        int        `gorm:"column:failed_count;not null;default:0" json:"failed_count"`
	AttemptCount       int        `gorm:"column:attempt_count;not null;default:0" json:"attempt_count"`
	LockedAt           *time.Time `gorm:"column:locked_at" json:"locked_at"`
	StartedAt          *time.Time `gorm:"column:started_at" json:"started_at"`
	FinishedAt         *time.Time `gorm:"column:finished_at" json:"finished_at"`
	CreatedAt          time.Time  `gorm:"column:created_at" json:"created_at"`
	UpdatedAt          time.Time  `gorm:"column:updated_at" json:"updated_at"`
}

func (MirrorTask) TableName() string { return "mirror_tasks" }

type MirrorIllust struct {
	IllustID       int64     `gorm:"primaryKey;column:illust_id" json:"illust_id"`
	DetailJSON     string    `gorm:"column:detail_json;not null" json:"detail_json"`
	ImageFilesJSON string    `gorm:"column:image_files_json;not null" json:"image_files_json"`
	CreatedAt      time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt      time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (MirrorIllust) TableName() string { return "mirror_illust" }

type MirrorNovel struct {
	NovelID    int64     `gorm:"primaryKey;column:novel_id" json:"novel_id"`
	DetailJSON string    `gorm:"column:detail_json;not null" json:"detail_json"`
	TextJSON   string    `gorm:"column:text_json;not null" json:"text_json"`
	CreatedAt  time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt  time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (MirrorNovel) TableName() string { return "mirror_novel" }
