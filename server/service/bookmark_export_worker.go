package service

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"pixez-sync/db"
	"pixez-sync/model"

	"gorm.io/gorm"
)

var shanghaiLoc *time.Location

func init() {
	var err error
	shanghaiLoc, err = time.LoadLocation("Asia/Shanghai")
	if err != nil {
		shanghaiLoc = time.FixedZone("CST", 8*60*60)
	}
}

func nowShanghai() time.Time {
	return time.Now().In(shanghaiLoc)
}

var bookmarkRestricts = []string{"public", "private"}

const scheduledTaskInitialDelay = 1 * time.Minute

type BookmarkExportWorker struct {
	Interval time.Duration
	Pixiv    *PixivUtils
	stopCh   chan struct{}
	runMu    sync.Mutex
}

type BookmarkExportResult struct {
	RunID        string
	PixivUserID  string
	Restrict     string
	TotalCount   int
	NewCount     int
	UpdatedCount int
	RemovedCount int
}

type bookmarkIllustSaveStatus int

const (
	bookmarkIllustSaveSkipped bookmarkIllustSaveStatus = iota
	bookmarkIllustSaveCreated
	bookmarkIllustSaveUpdated
	bookmarkIllustSaveRemoved
)

func NewBookmarkExportWorker(interval time.Duration) *BookmarkExportWorker {
	if interval <= 0 {
		interval = 24 * time.Hour
	}
	return &BookmarkExportWorker{
		Interval: interval,
		Pixiv:    NewPixivUtils(),
		stopCh:   make(chan struct{}),
	}
}

func (w *BookmarkExportWorker) Start() {
	go w.loop()
}

func (w *BookmarkExportWorker) Stop() {
	close(w.stopCh)
}

func (w *BookmarkExportWorker) loop() {
	w.ensureScheduledTask()

	for {
		wait := w.timeUntilNextRun()
		slog.Info("Bookmark export scheduled",
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

func (w *BookmarkExportWorker) ensureScheduledTask() {
	var task model.ScheduledTask
	err := db.DB.First(&task, "name = ?", model.ScheduledTaskBookmarkExport).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		slog.Error("Failed to query scheduled task", "error", err)
		return
	}
	if err == nil {
		// Update interval if config changed
		db.DB.Model(&model.ScheduledTask{}).
			Where("name = ?", model.ScheduledTaskBookmarkExport).
			Update("interval_seconds", int(w.Interval.Seconds()))
		return
	}

	now := nowShanghai()
	nextRun := now.Add(scheduledTaskInitialDelay)
	task = model.ScheduledTask{
		Name:            model.ScheduledTaskBookmarkExport,
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

func (w *BookmarkExportWorker) timeUntilNextRun() time.Duration {
	var task model.ScheduledTask
	if err := db.DB.First(&task, "name = ?", model.ScheduledTaskBookmarkExport).Error; err != nil {
		return w.Interval
	}
	if task.NextRunAt == nil {
		return w.Interval
	}
	// Normalize to Shanghai timezone for consistent comparison with SQLite-stored timestamps
	nextRunShanghai := task.NextRunAt.In(shanghaiLoc)
	d := nextRunShanghai.Sub(nowShanghai())
	if d <= 0 {
		return 0
	}
	return d
}

func (w *BookmarkExportWorker) RunOnceAsync() bool {
	if !w.runMu.TryLock() {
		return false
	}
	go func() {
		defer w.runMu.Unlock()
		w.runOnceAndUpdateLocked()
	}()
	return true
}

func (w *BookmarkExportWorker) RunOnceAndUpdate() bool {
	if !w.runMu.TryLock() {
		return false
	}
	defer w.runMu.Unlock()
	w.runOnceAndUpdateLocked()
	return true
}

func (w *BookmarkExportWorker) runOnceAndUpdateLocked() {
	w.ensureScheduledTask()

	now := nowShanghai()
	nextRun := now.Add(w.Interval)

	db.DB.Model(&model.ScheduledTask{}).
		Where("name = ?", model.ScheduledTaskBookmarkExport).
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
		slog.Error("Bookmark export pass failed", "error", err, "elapsed", elapsed.Round(time.Millisecond))
	} else {
		updates["last_status"] = model.ScheduledTaskStatusSuccess
		updates["last_error"] = ""
		slog.Info("Bookmark export pass finished", "elapsed", elapsed.Round(time.Millisecond), "nextRunAt", nextRun)
	}
	db.DB.Model(&model.ScheduledTask{}).
		Where("name = ?", model.ScheduledTaskBookmarkExport).
		Updates(updates)
}

func (w *BookmarkExportWorker) ExportAllUsersOnce() error {
	var users []model.PixivUser
	if err := db.DB.Order("updated_at desc").Find(&users).Error; err != nil {
		return fmt.Errorf("query Pixiv users failed: %w", err)
	}
	slog.Info("Bookmark export pass starting", "userCount", len(users))
	for _, user := range users {
		w.ExportUserOnce(user)
	}
	slog.Info("Bookmark export pass completed", "userCount", len(users))
	return nil
}

func (w *BookmarkExportWorker) ExportUserOnce(user model.PixivUser) {
	slog.Debug("Exporting bookmarks for user", "pixivUserID", user.PixivUserID, "name", user.Name)
	for _, restrict := range bookmarkRestricts {
		result, err := w.exportUserForRestrict(user, restrict)
		if err != nil {
			slog.Error("Bookmark export failed for user",
				"pixivUserID", user.PixivUserID,
				"restrict", restrict,
				"error", err)
			continue
		}
		slog.Info("Bookmark export completed for user",
			"pixivUserID", user.PixivUserID,
			"restrict", restrict,
			"fetched", result.TotalCount,
			"new", result.NewCount,
			"updated", result.UpdatedCount,
			"removed", result.RemovedCount)
	}
}

func (w *BookmarkExportWorker) exportUserForRestrict(user model.PixivUser, restrict string) (BookmarkExportResult, error) {
	slog.Debug("Starting bookmark export run", "pixivUserID", user.PixivUserID, "restrict", restrict)
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
		slog.Debug("Bookmark export run failed",
			"pixivUserID", user.PixivUserID, "restrict", restrict,
			"runID", run.ID, "elapsed", elapsed, "error", err)
		return result, err
	}
	updates["status"] = model.BookmarkExportStatusSuccess
	updates["error_message"] = ""
	if updateErr := db.DB.Model(&model.BookmarkExportRun{}).Where("id = ?", run.ID).Updates(updates).Error; updateErr != nil {
		return result, updateErr
	}
	slog.Debug("Bookmark export run finished",
		"pixivUserID", user.PixivUserID, "restrict", restrict,
		"runID", run.ID, "elapsed", elapsed,
		"fetched", result.TotalCount, "new", result.NewCount,
		"updated", result.UpdatedCount, "removed", result.RemovedCount)
	return result, nil
}

func newBookmarkExportRunID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err == nil {
		return hex.EncodeToString(b[:])
	}
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

func (w *BookmarkExportWorker) exportUserWithRun(user model.PixivUser, runID string, restrict string, result *BookmarkExportResult) error {
	nextURL := w.Pixiv.InitialBookmarkIllustURL(user.PixivUserID, restrict)
	slog.Debug("Bookmark export starting pagination",
		"pixivUserID", user.PixivUserID, "restrict", restrict, "initialURL", nextURL)

	page := 0
	seenIllustIDs := make([]int64, 0)
	for nextURL != "" {
		page++
		_, payload, err := w.Pixiv.GetBookmarkIllusts(user, nextURL)
		if err != nil {
			slog.Debug("Bookmark export page fetch failed",
				"pixivUserID", user.PixivUserID, "restrict", restrict,
				"page", page, "url", nextURL, "error", err)
			return err
		}
		slog.Debug("Bookmark export page fetched",
			"pixivUserID", user.PixivUserID, "restrict", restrict,
			"page", page, "illustsInPage", len(payload.Illusts),
			"hasNext", payload.NextURL != "",
			"url", nextURL)

		if err := db.DB.Model(&model.BookmarkExportRun{}).
			Where("id = ?", runID).
			Updates(map[string]any{
				"next_url":         payload.NextURL,
				"last_request_url": nextURL,
				"updated_at":       time.Now(),
			}).Error; err != nil {
			return err
		}
		for _, illust := range payload.Illusts {
			saveStatus, err := upsertBookmarkIllust(user.PixivUserID, restrict, runID, illust)
			if err != nil {
				slog.Debug("Bookmark upsert failed",
					"pixivUserID", user.PixivUserID, "restrict", restrict,
					"illustID", illust.ID, "error", err)
				return err
			}
			result.TotalCount++
			if saveStatus != bookmarkIllustSaveRemoved {
				seenIllustIDs = append(seenIllustIDs, illust.ID)
			}
			if saveStatus == bookmarkIllustSaveCreated {
				result.NewCount++
			} else if saveStatus == bookmarkIllustSaveUpdated {
				result.UpdatedCount++
			} else if saveStatus == bookmarkIllustSaveRemoved {
				result.RemovedCount++
			}
		}
		nextURL = payload.NextURL
	}

	removed, err := markMissingBookmarksRemoved(user.PixivUserID, restrict, seenIllustIDs)
	if err != nil {
		return err
	}
	result.RemovedCount += int(removed)
	slog.Debug("Bookmark export missing marks applied",
		"pixivUserID", user.PixivUserID, "restrict", restrict, "removedCount", removed)
	return nil
}

func upsertBookmarkIllust(pixivUserID string, restrict string, runID string, illust PixivIllust) (bookmarkIllustSaveStatus, error) {
	if illust.ID <= 0 {
		return bookmarkIllustSaveSkipped, fmt.Errorf("bookmark export received illust without id")
	}

	var existing model.BookmarkIllust
	err := db.DB.Where(
		"pixiv_user_id = ? AND restrict = ? AND illust_id = ?",
		pixivUserID,
		restrict,
		illust.ID,
	).First(&existing).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return bookmarkIllustSaveSkipped, err
	}
	created := errors.Is(err, gorm.ErrRecordNotFound)
	isLimitUnknown := pixivIllustHasLimitUnknownImage(illust)
	if isLimitUnknown && !created {
		if existing.Removed {
			return bookmarkIllustSaveSkipped, nil
		}
		now := time.Now()
		if err := db.DB.Model(&existing).Updates(map[string]any{
			"removed":    true,
			"removed_at": now,
			"updated_at": now,
		}).Error; err != nil {
			return bookmarkIllustSaveSkipped, err
		}
		return bookmarkIllustSaveRemoved, nil
	}
	if !created && !existing.Removed {
		return bookmarkIllustSaveSkipped, nil
	}

	raw, err := json.Marshal(illust)
	if err != nil {
		return bookmarkIllustSaveSkipped, err
	}
	now := time.Now()

	record := model.BookmarkIllust{
		PixivUserID:     pixivUserID,
		Restrict:        restrict,
		IllustID:        illust.ID,
		Title:           illust.Title,
		Type:            illust.Type,
		UserID:          illust.User.ID,
		UserName:        illust.User.Name,
		PageCount:       illust.PageCount,
		Width:           illust.Width,
		Height:          illust.Height,
		SanityLevel:     illust.SanityLevel,
		XRestrict:       illust.XRestrict,
		TotalView:       illust.TotalView,
		TotalBookmarks:  illust.TotalBookmarks,
		Visible:         illust.Visible,
		IsMuted:         illust.IsMuted,
		IllustAIType:    illust.IllustAIType,
		IllustJSON:      string(raw),
		LastExportRunID: runID,
		LastSeenAt:      now,
		Removed:         isLimitUnknown,
		RemovedAt:       nil,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if isLimitUnknown {
		record.RemovedAt = &now
	}

	if created {
		if err := db.DB.Create(&record).Error; err != nil {
			return bookmarkIllustSaveSkipped, err
		}
		if isLimitUnknown {
			return bookmarkIllustSaveRemoved, nil
		}
		return bookmarkIllustSaveCreated, nil
	}

	if err := db.DB.Model(&existing).Updates(
		map[string]any{
			"title":              record.Title,
			"type":               record.Type,
			"user_id":            record.UserID,
			"user_name":          record.UserName,
			"page_count":         record.PageCount,
			"width":              record.Width,
			"height":             record.Height,
			"sanity_level":       record.SanityLevel,
			"x_restrict":         record.XRestrict,
			"total_view":         record.TotalView,
			"total_bookmarks":    record.TotalBookmarks,
			"visible":            record.Visible,
			"is_muted":           record.IsMuted,
			"illust_ai_type":     record.IllustAIType,
			"illust_json":        record.IllustJSON,
			"last_export_run_id": record.LastExportRunID,
			"last_seen_at":       record.LastSeenAt,
			"removed":            false,
			"removed_at":         nil,
			"updated_at":         record.UpdatedAt,
		},
	).Error; err != nil {
		return bookmarkIllustSaveSkipped, err
	}
	return bookmarkIllustSaveUpdated, nil
}

func pixivIllustHasLimitUnknownImage(illust PixivIllust) bool {
	if strings.Contains(illust.ImageUrls.SquareMedium, "limit_unknown_360") ||
		strings.Contains(illust.ImageUrls.Medium, "limit_unknown_360") ||
		strings.Contains(illust.ImageUrls.Large, "limit_unknown_360") ||
		strings.Contains(illust.MetaSinglePage.OriginalImageURL, "limit_unknown_360") {
		return true
	}
	for _, page := range illust.MetaPages {
		if strings.Contains(page.ImageUrls.SquareMedium, "limit_unknown_360") ||
			strings.Contains(page.ImageUrls.Medium, "limit_unknown_360") ||
			strings.Contains(page.ImageUrls.Large, "limit_unknown_360") ||
			strings.Contains(page.ImageUrls.Original, "limit_unknown_360") {
			return true
		}
	}
	return false
}

func markMissingBookmarksRemoved(pixivUserID string, restrict string, seenIllustIDs []int64) (int64, error) {
	now := time.Now()
	query := db.DB.Model(&model.BookmarkIllust{}).
		Where("pixiv_user_id = ? AND restrict = ? AND removed = ?", pixivUserID, restrict, false)
	if len(seenIllustIDs) > 0 {
		query = query.Where("illust_id NOT IN ?", seenIllustIDs)
	}
	result := query.Updates(map[string]any{
		"removed":    true,
		"removed_at": now,
		"updated_at": now,
	})
	return result.RowsAffected, result.Error
}
