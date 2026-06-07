package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"pixez-sync/db"
	"pixez-sync/model"

	"gorm.io/gorm"
)

const (
	// How many unmirrored bookmarks to enqueue per cycle.
	mirrorBatchSize = 6
)

// BookmarkMirrorScheduler periodically scans bookmark_illusts for items
// that need mirroring and enqueues them into mirror_tasks.
//
// Three scheduled loops:
//   - newLoop:       every 1 minute  — picks mirror_status=0 (unmirrored)
//   - retryLoop:     every 3 minutes — picks mirror_status=-1 (failed) with retry < 3
//   - retryImgLoop:  every 10 minutes — retries partially failed downloads (failed_count > 0)
type BookmarkMirrorScheduler struct {
	NewInterval      time.Duration
	RetryInterval    time.Duration
	RetryImgInterval time.Duration
	mirrorWorker     *MirrorWorker
	stopCh           chan struct{}
	newRunMu         sync.Mutex
	retryRunMu       sync.Mutex
	retryImgRunMu    sync.Mutex
}

func NewBookmarkMirrorScheduler(worker *MirrorWorker) *BookmarkMirrorScheduler {
	return &BookmarkMirrorScheduler{
		NewInterval:      1 * time.Minute,
		RetryInterval:    3 * time.Minute,
		RetryImgInterval: 10 * time.Minute,
		mirrorWorker:     worker,
		stopCh:           make(chan struct{}),
	}
}

func (s *BookmarkMirrorScheduler) Start() {
	go s.newLoop()
	go s.retryLoop()
	go s.retryImgLoop()
}

func (s *BookmarkMirrorScheduler) Stop() {
	close(s.stopCh)
}

// ---------------------------------------------------------------------------
// New mirror loop — every 1 minute
// ---------------------------------------------------------------------------

func (s *BookmarkMirrorScheduler) newLoop() {
	s.ensureScheduledTask(model.ScheduledTaskBookmarkMirrorNew, s.NewInterval)

	for {
		wait := s.timeUntilNextRun(model.ScheduledTaskBookmarkMirrorNew, s.NewInterval)
		slog.Debug("Bookmark mirror (new) scheduled", "wait", wait.Round(time.Second))
		timer := time.NewTimer(wait)
		select {
		case <-s.stopCh:
			timer.Stop()
			return
		case <-timer.C:
		}
		s.runNewCycle()
	}
}

func (s *BookmarkMirrorScheduler) runNewCycle() {
	if !s.newRunMu.TryLock() {
		return
	}
	defer s.newRunMu.Unlock()

	name := model.ScheduledTaskBookmarkMirrorNew
	s.updateScheduledStart(name)

	start := time.Now()
	enqueued, err := s.enqueueNewMirrors()
	elapsed := time.Since(start)

	s.updateScheduledEnd(name, elapsed, enqueued, err)
	if err != nil {
		slog.Error("Bookmark mirror (new) cycle failed", "error", err, "elapsed", elapsed.Round(time.Millisecond))
	} else if enqueued > 0 {
		slog.Info("Bookmark mirror (new) cycle completed", "enqueued", enqueued, "elapsed", elapsed.Round(time.Millisecond))
	}
}

// enqueueNewMirrors selects up to mirrorBatchSize bookmark_illusts with
// mirror_status=0 and mirror_status=2 (mirroring but not yet in queue),
// sets them to mirror_status=2, and enqueues mirror_tasks.
func (s *BookmarkMirrorScheduler) enqueueNewMirrors() (int, error) {
	var bookmarks []model.BookmarkIllust
	if err := db.DB.Where(
		"removed = ? AND mirror_status = ?",
		false,
		model.BookmarkMirrorNone,
	).Order("last_seen_at asc").
		Limit(mirrorBatchSize).
		Find(&bookmarks).Error; err != nil {
		return 0, fmt.Errorf("query unmirrored bookmarks: %w", err)
	}
	if len(bookmarks) == 0 {
		return 0, nil
	}

	enqueued := 0
	now := time.Now()
	for _, bk := range bookmarks {
		task, err := enqueueMirrorTaskForIllust(bk.IllustID, false)
		if err != nil {
			if errors.Is(err, gorm.ErrDuplicatedKey) {
				slog.Debug("Mirror task already queued, skipping", "illustID", bk.IllustID)
			} else {
				slog.Error("Failed to enqueue mirror task", "illustID", bk.IllustID, "error", err)
				continue
			}
		}

		// If the task already succeeded, mark the bookmark as done immediately
		// instead of setting it to mirroring — the worker won't re-process a
		// successful task, so leaving it at 2 would be permanent.
		status := model.BookmarkMirrorMirroring
		if task.SuccessCount > 0 {
			status = model.BookmarkMirrorDone
		}

		if err := db.DB.Model(&model.BookmarkIllust{}).
			Where("id = ?", bk.ID).
			Updates(map[string]any{
				"mirror_status": status,
				"updated_at":    now,
			}).Error; err != nil {
			slog.Error("Failed to update bookmark mirror_status", "illustID", bk.IllustID, "error", err)
			continue
		}
		enqueued++
	}
	return enqueued, nil
}

// ---------------------------------------------------------------------------
// Retry loop — every 3 minutes
// ---------------------------------------------------------------------------

func (s *BookmarkMirrorScheduler) retryLoop() {
	s.ensureScheduledTask(model.ScheduledTaskBookmarkMirrorRetry, s.RetryInterval)

	for {
		wait := s.timeUntilNextRun(model.ScheduledTaskBookmarkMirrorRetry, s.RetryInterval)
		slog.Debug("Bookmark mirror (retry) scheduled", "wait", wait.Round(time.Second))
		timer := time.NewTimer(wait)
		select {
		case <-s.stopCh:
			timer.Stop()
			return
		case <-timer.C:
		}
		s.runRetryCycle()
	}
}

func (s *BookmarkMirrorScheduler) runRetryCycle() {
	if !s.retryRunMu.TryLock() {
		return
	}
	defer s.retryRunMu.Unlock()

	name := model.ScheduledTaskBookmarkMirrorRetry
	s.updateScheduledStart(name)

	start := time.Now()
	retried, err := s.retryFailedMirrors()
	elapsed := time.Since(start)

	s.updateScheduledEnd(name, elapsed, retried, err)
	if err != nil {
		slog.Error("Bookmark mirror (retry) cycle failed", "error", err, "elapsed", elapsed.Round(time.Millisecond))
	} else if retried > 0 {
		slog.Info("Bookmark mirror (retry) cycle completed", "retried", retried, "elapsed", elapsed.Round(time.Millisecond))
	}
}

// retryFailedMirrors selects bookmark_illusts with mirror_status=-1 and
// mirror_retry_count < 3, resets them to mirror_status=2, increments retry
// count, and re-enqueues mirror_tasks (retry path).
func (s *BookmarkMirrorScheduler) retryFailedMirrors() (int, error) {
	var bookmarks []model.BookmarkIllust
	if err := db.DB.Where(
		"removed = ? AND mirror_status = ? AND mirror_retry_count < ?",
		false,
		model.BookmarkMirrorFailed,
		model.BookmarkMirrorMaxRetry,
	).Order("updated_at asc").
		Find(&bookmarks).Error; err != nil {
		return 0, fmt.Errorf("query failed bookmarks for retry: %w", err)
	}
	if len(bookmarks) == 0 {
		return 0, nil
	}

	retried := 0
	now := time.Now()
	for _, bk := range bookmarks {
		// Re-enqueue using retry path (resets task status).
		_, err := enqueueMirrorTaskForIllust(bk.IllustID, true)
		if err != nil {
			slog.Error("Failed to re-enqueue mirror task for retry", "illustID", bk.IllustID, "error", err)
			continue
		}

		// Increment retry count and set back to mirroring.
		if err := db.DB.Model(&model.BookmarkIllust{}).
			Where("id = ?", bk.ID).
			Updates(map[string]any{
				"mirror_status":      model.BookmarkMirrorMirroring,
				"mirror_retry_count": gorm.Expr("mirror_retry_count + 1"),
				"updated_at":         now,
			}).Error; err != nil {
			slog.Error("Failed to update bookmark retry state", "illustID", bk.IllustID, "error", err)
			continue
		}
		retried++
	}
	return retried, nil
}

// ---------------------------------------------------------------------------
// Image retry loop — every 10 minutes
// ---------------------------------------------------------------------------

func (s *BookmarkMirrorScheduler) retryImgLoop() {
	s.ensureScheduledTask(model.ScheduledTaskBookmarkMirrorRetryImg, s.RetryImgInterval)

	for {
		wait := s.timeUntilNextRun(model.ScheduledTaskBookmarkMirrorRetryImg, s.RetryImgInterval)
		slog.Debug("Bookmark mirror (retry images) scheduled", "wait", wait.Round(time.Second))
		timer := time.NewTimer(wait)
		select {
		case <-s.stopCh:
			timer.Stop()
			return
		case <-timer.C:
		}
		s.runRetryImgCycle()
	}
}

func (s *BookmarkMirrorScheduler) runRetryImgCycle() {
	if !s.retryImgRunMu.TryLock() {
		return
	}
	defer s.retryImgRunMu.Unlock()

	name := model.ScheduledTaskBookmarkMirrorRetryImg
	s.updateScheduledStart(name)

	start := time.Now()
	retried, err := s.retryFailedImages()
	elapsed := time.Since(start)

	s.updateScheduledEnd(name, elapsed, retried, err)
	if err != nil {
		slog.Error("Bookmark mirror (retry images) cycle failed", "error", err, "elapsed", elapsed.Round(time.Millisecond))
	} else if retried > 0 {
		slog.Info("Bookmark mirror (retry images) cycle completed", "retried", retried, "elapsed", elapsed.Round(time.Millisecond))
	}
}

// retryFailedImages finds mirror_tasks with failed_count > 0 that already
// succeeded overall, and re-downloads the failed image URLs. This handles
// partial failures (e.g., 5/10 images downloaded) without resetting the
// entire task.
func (s *BookmarkMirrorScheduler) retryFailedImages() (int, error) {
	var tasks []model.MirrorTask
	if err := db.DB.Where(
		"target_type = ? AND status = ? AND failed_count > 0",
		model.MirrorTargetIllust,
		model.MirrorTaskStatusSuccess,
	).Order("updated_at asc").
		Find(&tasks).Error; err != nil {
		return 0, fmt.Errorf("query tasks with failed images: %w", err)
	}
	if len(tasks) == 0 {
		return 0, nil
	}

	retried := 0
	for _, task := range tasks {
		newOK, newFailed, err := s.mirrorWorker.RetryFailedDownloads(task)
		if err != nil {
			slog.Error("Image retry failed", "taskID", task.ID, "illustID", task.TargetID, "error", err)
			continue
		}
		if newOK > 0 {
			retried++
			slog.Info("Image retry succeeded",
				"taskID", task.ID, "illustID", task.TargetID,
				"newSuccess", newOK, "stillFailed", newFailed)
		}
	}
	return retried, nil
}

// ---------------------------------------------------------------------------
// enqueueMirrorTaskForIllust — shared enqueue logic
// ---------------------------------------------------------------------------

// enqueueMirrorTaskForIllust creates or resets a mirror_task for the given illust.
// If isRetry is true and an existing task is found, it resets the task to queued.
// If isRetry is false and an existing task is found, it returns the existing task.
// The unique index on (target_type, target_id) ensures at most one task per illust.
func enqueueMirrorTaskForIllust(illustID int64, isRetry bool) (model.MirrorTask, error) {
	var existing model.MirrorTask
	err := db.DB.Where(
		"target_type = ? AND target_id = ?",
		model.MirrorTargetIllust,
		illustID,
	).First(&existing).Error

	if err == nil {
		if !isRetry {
			return existing, nil
		}
		// Retry: reset existing task to queued for re-processing.
		now := time.Now()
		if err := db.DB.Model(&model.MirrorTask{}).
			Where("id = ?", existing.ID).
			Updates(map[string]any{
				"status":        model.MirrorTaskStatusQueued,
				"error_message": "",
				"success_count": 0,
				"failed_count":  0,
				"attempt_count": gorm.Expr("attempt_count + 1"),
				"finished_at":   nil,
				"updated_at":    now,
			}).Error; err != nil {
			return model.MirrorTask{}, fmt.Errorf("reset mirror task for retry: %w", err)
		}
		existing.Status = model.MirrorTaskStatusQueued
		return existing, nil
	}

	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return model.MirrorTask{}, err
	}

	// No existing task — create new.
	payload, _ := json.Marshal(map[string]any{"illust_id": illustID})
	now := time.Now()
	task := model.MirrorTask{
		ID:                 newMirrorSchedulerTaskID(),
		TaskType:           model.MirrorTaskTypeIllust,
		TargetType:         model.MirrorTargetIllust,
		TargetID:           illustID,
		Status:             model.MirrorTaskStatusQueued,
		RequestPayloadJSON: string(payload),
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	if err := db.DB.Create(&task).Error; err != nil {
		return model.MirrorTask{}, err
	}
	return task, nil
}

func newMirrorSchedulerTaskID() string {
	return newBookmarkExportRunID() // same random hex ID generator
}

// ---------------------------------------------------------------------------
// Scheduled task helpers (same pattern as BookmarkExportWorker)
// ---------------------------------------------------------------------------

func (s *BookmarkMirrorScheduler) ensureScheduledTask(name string, interval time.Duration) {
	var task model.ScheduledTask
	err := db.DB.First(&task, "name = ?", name).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		slog.Error("Failed to query scheduled task", "name", name, "error", err)
		return
	}
	if err == nil {
		db.DB.Model(&model.ScheduledTask{}).
			Where("name = ?", name).
			Update("interval_seconds", int(interval.Seconds()))
		return
	}

	now := nowShanghai()
	nextRun := now.Add(scheduledTaskInitialDelay)
	task = model.ScheduledTask{
		Name:            name,
		Enabled:         true,
		IntervalSeconds: int(interval.Seconds()),
		NextRunAt:       &nextRun,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := db.DB.Create(&task).Error; err != nil {
		slog.Error("Failed to create scheduled task", "name", name, "error", err)
	}
}

func (s *BookmarkMirrorScheduler) timeUntilNextRun(name string, fallback time.Duration) time.Duration {
	var task model.ScheduledTask
	if err := db.DB.First(&task, "name = ?", name).Error; err != nil {
		return fallback
	}
	if task.NextRunAt == nil {
		return fallback
	}
	d := task.NextRunAt.In(shanghaiLoc).Sub(nowShanghai())
	if d <= 0 {
		return 0
	}
	return d
}

func (s *BookmarkMirrorScheduler) updateScheduledStart(name string) {
	now := nowShanghai()
	db.DB.Model(&model.ScheduledTask{}).
		Where("name = ?", name).
		Updates(map[string]any{
			"last_run_at": now,
			"last_status": model.ScheduledTaskStatusRunning,
			"last_error":  "",
			"updated_at":  now,
		})
}

func (s *BookmarkMirrorScheduler) updateScheduledEnd(name string, elapsed time.Duration, count int, err error) {
	now := nowShanghai()
	var interval model.ScheduledTask
	intervalSec := 60
	if db.DB.First(&interval, "name = ?", name).Error == nil {
		intervalSec = interval.IntervalSeconds
	}
	nextRun := now.Add(time.Duration(intervalSec) * time.Second)

	updates := map[string]any{
		"last_duration_ms": elapsed.Milliseconds(),
		"next_run_at":      nextRun,
		"updated_at":       now,
	}
	if err != nil {
		updates["last_status"] = model.ScheduledTaskStatusFailed
		updates["last_error"] = err.Error()
	} else {
		updates["last_status"] = model.ScheduledTaskStatusSuccess
		updates["last_error"] = ""
	}
	db.DB.Model(&model.ScheduledTask{}).Where("name = ?", name).Updates(updates)
}
