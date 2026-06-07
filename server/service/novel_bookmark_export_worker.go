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

// NovelBookmarkExportWorker periodically exports Pixiv novel bookmarks for all synced users.
type NovelBookmarkExportWorker struct {
	Interval time.Duration
	Pixiv    *PixivUtils
	stopCh   chan struct{}
	runMu    sync.Mutex
}

func NewNovelBookmarkExportWorker(interval time.Duration) *NovelBookmarkExportWorker {
	if interval <= 0 {
		interval = 24 * time.Hour
	}
	return &NovelBookmarkExportWorker{
		Interval: interval,
		Pixiv:    NewPixivUtils(),
		stopCh:   make(chan struct{}),
	}
}

func (w *NovelBookmarkExportWorker) Start() { go w.loop() }
func (w *NovelBookmarkExportWorker) Stop()  { close(w.stopCh) }

func (w *NovelBookmarkExportWorker) loop() {
	w.ensureScheduledTask()
	for {
		wait := w.timeUntilNextRun()
		slog.Info("Novel bookmark export scheduled",
			"interval", w.Interval,
			"waitDuration", wait.Round(time.Second))

		timer := time.NewTimer(wait)
		select {
		case <-w.stopCh:
			timer.Stop()
			return
		case <-timer.C:
		}
		w.RunOnceAndUpdate()
	}
}

func (w *NovelBookmarkExportWorker) ensureScheduledTask() {
	var task model.ScheduledTask
	err := db.DB.First(&task, "name = ?", model.ScheduledTaskNovelBookmarkExport).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		slog.Error("Failed to query scheduled task", "error", err)
		return
	}
	if err == nil {
		db.DB.Model(&model.ScheduledTask{}).
			Where("name = ?", model.ScheduledTaskNovelBookmarkExport).
			Update("interval_seconds", int(w.Interval.Seconds()))
		return
	}
	now := nowShanghai()
	nextRun := now.Add(scheduledTaskInitialDelay)
	task = model.ScheduledTask{
		Name:            model.ScheduledTaskNovelBookmarkExport,
		Enabled:         true,
		IntervalSeconds: int(w.Interval.Seconds()),
		NextRunAt:       &nextRun,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := db.DB.Create(&task).Error; err != nil {
		slog.Error("Failed to create scheduled task", "error", err)
	}
	slog.Info("Scheduled task registered", "name", task.Name, "interval", w.Interval, "nextRunAt", nextRun)
}

func (w *NovelBookmarkExportWorker) timeUntilNextRun() time.Duration {
	var task model.ScheduledTask
	if err := db.DB.First(&task, "name = ?", model.ScheduledTaskNovelBookmarkExport).Error; err != nil {
		return w.Interval
	}
	if task.NextRunAt == nil {
		return w.Interval
	}
	d := task.NextRunAt.In(shanghaiLoc).Sub(nowShanghai())
	if d <= 0 {
		return 0
	}
	return d
}

func (w *NovelBookmarkExportWorker) RunOnceAsync() bool {
	if !w.runMu.TryLock() {
		return false
	}
	go func() {
		defer w.runMu.Unlock()
		w.runOnceAndUpdateLocked()
	}()
	return true
}

func (w *NovelBookmarkExportWorker) RunOnceAndUpdate() bool {
	if !w.runMu.TryLock() {
		return false
	}
	defer w.runMu.Unlock()
	w.runOnceAndUpdateLocked()
	return true
}

func (w *NovelBookmarkExportWorker) runOnceAndUpdateLocked() {
	w.ensureScheduledTask()

	now := nowShanghai()
	nextRun := now.Add(w.Interval)

	db.DB.Model(&model.ScheduledTask{}).
		Where("name = ?", model.ScheduledTaskNovelBookmarkExport).
		Updates(map[string]any{
			"last_run_at": now,
			"last_status": model.ScheduledTaskStatusRunning,
			"last_error":  "",
			"updated_at":  now,
		})

	start := time.Now()
	err := w.ExportAllUsersOnce()
	elapsed := time.Since(start)

	updates := map[string]any{
		"last_duration_ms": elapsed.Milliseconds(),
		"next_run_at":      nextRun,
		"updated_at":       nowShanghai(),
	}
	if err != nil {
		updates["last_status"] = model.ScheduledTaskStatusFailed
		updates["last_error"] = err.Error()
		slog.Error("Novel bookmark export pass failed", "error", err, "elapsed", elapsed.Round(time.Millisecond))
	} else {
		updates["last_status"] = model.ScheduledTaskStatusSuccess
		updates["last_error"] = ""
		slog.Info("Novel bookmark export pass finished", "elapsed", elapsed.Round(time.Millisecond), "nextRunAt", nextRun)
	}
	db.DB.Model(&model.ScheduledTask{}).
		Where("name = ?", model.ScheduledTaskNovelBookmarkExport).
		Updates(updates)
}

func (w *NovelBookmarkExportWorker) ExportAllUsersOnce() error {
	var users []model.PixivUser
	if err := db.DB.Order("updated_at desc").Find(&users).Error; err != nil {
		return fmt.Errorf("query Pixiv users failed: %w", err)
	}
	slog.Info("Novel bookmark export pass starting", "userCount", len(users))
	for _, user := range users {
		w.ExportUserOnce(user)
	}
	slog.Info("Novel bookmark export pass completed", "userCount", len(users))
	return nil
}

func (w *NovelBookmarkExportWorker) ExportUserOnce(user model.PixivUser) {
	for _, restrict := range bookmarkRestricts {
		result, err := w.exportUserForRestrict(user, restrict)
		if err != nil {
			slog.Error("Novel bookmark export failed for user",
				"pixivUserID", user.PixivUserID, "restrict", restrict, "error", err)
			continue
		}
		slog.Info("Novel bookmark export completed for user",
			"pixivUserID", user.PixivUserID, "restrict", restrict,
			"fetched", result.TotalCount, "new", result.NewCount,
			"updated", result.UpdatedCount, "removed", result.RemovedCount)
	}
}

func (w *NovelBookmarkExportWorker) exportUserForRestrict(user model.PixivUser, restrict string) (BookmarkExportResult, error) {
	start := time.Now()

	run := model.BookmarkExportRun{
		ID:          newBookmarkExportRunID(),
		PixivUserID: user.PixivUserID,
		Restrict:    restrict,
		Status:      model.BookmarkExportStatusRunning,
		StartedAt:   start,
		CreatedAt:   start,
		UpdatedAt:   start,
	}
	if err := db.DB.Create(&run).Error; err != nil {
		return BookmarkExportResult{}, err
	}

	result := BookmarkExportResult{
		RunID:       run.ID,
		PixivUserID: user.PixivUserID,
		Restrict:    restrict,
	}
	err := w.exportUserWithRun(user, run.ID, restrict, &result)
	finishedAt := time.Now()
	elapsed := finishedAt.Sub(start).Round(time.Millisecond)
	updates := map[string]any{
		"total_count":   result.TotalCount,
		"new_count":     result.NewCount,
		"updated_count": result.UpdatedCount,
		"removed_count": result.RemovedCount,
		"finished_at":   finishedAt,
		"updated_at":    finishedAt,
	}
	if err != nil {
		updates["status"] = model.BookmarkExportStatusFailed
		updates["error_message"] = err.Error()
		_ = db.DB.Model(&model.BookmarkExportRun{}).Where("id = ?", run.ID).Updates(updates).Error
		return result, err
	}
	updates["status"] = model.BookmarkExportStatusSuccess
	updates["error_message"] = ""
	_ = db.DB.Model(&model.BookmarkExportRun{}).Where("id = ?", run.ID).Updates(updates).Error
	slog.Debug("Novel bookmark export run finished",
		"pixivUserID", user.PixivUserID, "restrict", restrict,
		"runID", run.ID, "elapsed", elapsed,
		"fetched", result.TotalCount, "new", result.NewCount,
		"updated", result.UpdatedCount, "removed", result.RemovedCount)
	return result, nil
}

func (w *NovelBookmarkExportWorker) exportUserWithRun(user model.PixivUser, runID string, restrict string, result *BookmarkExportResult) error {
	nextURL := w.Pixiv.InitialBookmarkNovelURL(user.PixivUserID, restrict)
	page := 0
	var seenNovelIDs []int64
	for nextURL != "" {
		page++
		_, payload, err := w.Pixiv.GetBookmarkNovels(user, nextURL)
		if err != nil {
			return err
		}
		db.DB.Model(&model.BookmarkExportRun{}).
			Where("id = ?", runID).
			Updates(map[string]any{
				"next_url":         payload.NextURL,
				"last_request_url": nextURL,
				"updated_at":       time.Now(),
			})
		for _, novel := range payload.Novels {
			created, err := upsertBookmarkNovel(user.PixivUserID, restrict, runID, novel)
			if err != nil {
				return err
			}
			result.TotalCount++
			seenNovelIDs = append(seenNovelIDs, novel.ID)
			if created {
				result.NewCount++
			} else {
				result.UpdatedCount++
			}
		}
		nextURL = payload.NextURL
	}

	removed, err := markMissingNovelBookmarksRemoved(user.PixivUserID, restrict, seenNovelIDs)
	if err != nil {
		return err
	}
	result.RemovedCount = int(removed)
	return nil
}

func upsertBookmarkNovel(pixivUserID string, restrict string, runID string, novel PixivBookmarkNovel) (bool, error) {
	if novel.ID <= 0 {
		return false, fmt.Errorf("novel bookmark export received novel without id")
	}

	var existing model.BookmarkNovel
	err := db.DB.Where(
		"pixiv_user_id = ? AND restrict = ? AND novel_id = ?",
		pixivUserID, restrict, novel.ID,
	).First(&existing).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return false, err
	}
	created := errors.Is(err, gorm.ErrRecordNotFound)

	raw, err := json.Marshal(novel)
	if err != nil {
		return false, err
	}
	now := time.Now()

	var seriesID *int64
	var seriesTitle *string
	if novel.Series != nil {
		seriesID = &novel.Series.ID
		seriesTitle = &novel.Series.Title
	}

	record := model.BookmarkNovel{
		PixivUserID:     pixivUserID,
		Restrict:        restrict,
		NovelID:         novel.ID,
		Title:           novel.Title,
		Caption:         novel.Caption,
		UserID:          novel.User.ID,
		UserName:        novel.User.Name,
		TextLength:      novel.TextLength,
		XRestrict:       novel.XRestrict,
		TotalView:       novel.TotalView,
		TotalBookmarks:  novel.TotalBookmarks,
		IsOriginal:      novel.IsOriginal,
		Visible:         novel.Visible,
		IsMuted:         novel.IsMuted,
		NovelAIType:     novel.NovelAIType,
		SeriesID:        seriesID,
		SeriesTitle:     seriesTitle,
		CoverURL:        novel.ImageUrls.Medium,
		NovelJSON:       string(raw),
		LastExportRunID: runID,
		LastSeenAt:      now,
		Removed:         false,
		RemovedAt:       nil,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	if created {
		if err := db.DB.Create(&record).Error; err != nil {
			return false, err
		}
		return true, nil
	}

	if !existing.Removed {
		// Already present and not removed — skip
		return false, nil
	}

	// Was removed, now re-bookmarked — restore
	if err := db.DB.Model(&existing).Updates(map[string]any{
		"title":              record.Title,
		"caption":            record.Caption,
		"user_id":            record.UserID,
		"user_name":          record.UserName,
		"text_length":        record.TextLength,
		"x_restrict":         record.XRestrict,
		"total_view":         record.TotalView,
		"total_bookmarks":    record.TotalBookmarks,
		"is_original":        record.IsOriginal,
		"visible":            record.Visible,
		"is_muted":           record.IsMuted,
		"novel_ai_type":      record.NovelAIType,
		"series_id":          record.SeriesID,
		"series_title":       record.SeriesTitle,
		"cover_url":          record.CoverURL,
		"novel_json":         record.NovelJSON,
		"last_export_run_id": record.LastExportRunID,
		"last_seen_at":       record.LastSeenAt,
		"removed":            false,
		"removed_at":         nil,
		"updated_at":         record.UpdatedAt,
	}).Error; err != nil {
		return false, err
	}
	return false, nil
}

func markMissingNovelBookmarksRemoved(pixivUserID string, restrict string, seenNovelIDs []int64) (int64, error) {
	now := time.Now()
	query := db.DB.Model(&model.BookmarkNovel{}).
		Where("pixiv_user_id = ? AND restrict = ? AND removed = ?", pixivUserID, restrict, false)
	if len(seenNovelIDs) > 0 {
		query = query.Where("novel_id NOT IN ?", seenNovelIDs)
	}
	result := query.Updates(map[string]any{
		"removed":    true,
		"removed_at": now,
		"updated_at": now,
	})
	return result.RowsAffected, result.Error
}
