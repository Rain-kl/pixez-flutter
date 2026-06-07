package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"pixez-sync/db"
	"pixez-sync/model"

	"gorm.io/gorm"
)

type MirrorWorker struct {
	MirrorDir           string
	DownloadConcurrency int
	PollInterval        time.Duration
	Pixiv               *PixivUtils
	stopCh              chan struct{}
}

type ImageFileRecord struct {
	URL      string `json:"url"`
	Path     string `json:"path"`
	Filename string `json:"filename"`
}

func NewMirrorWorker(mirrorDir string, downloadConcurrency int) *MirrorWorker {
	if downloadConcurrency <= 0 {
		downloadConcurrency = 5
	}
	return &MirrorWorker{
		MirrorDir:           mirrorDir,
		DownloadConcurrency: downloadConcurrency,
		PollInterval:        2 * time.Second,
		Pixiv:               NewPixivUtils(),
		stopCh:              make(chan struct{}),
	}
}

func (w *MirrorWorker) Start() {
	go w.loop()
}

func (w *MirrorWorker) Stop() {
	close(w.stopCh)
}

func (w *MirrorWorker) loop() {
	ticker := time.NewTicker(w.PollInterval)
	defer ticker.Stop()

	for {
		w.ProcessOne()
		select {
		case <-w.stopCh:
			return
		case <-ticker.C:
		}
	}
}

func (w *MirrorWorker) ProcessOne() bool {
	task, ok := w.claimQueuedTask()
	if !ok {
		return false
	}

	var err error
	switch task.TaskType {
	case model.MirrorTaskTypeIllust:
		err = w.processIllustTask(task)
	case model.MirrorTaskTypeNovel:
		err = w.processNovelTask(task)
	default:
		err = fmt.Errorf("unsupported mirror task type: %s", task.TaskType)
	}

	if err != nil {
		slog.Error("Mirror task failed", "taskID", task.ID, "error", err)
		if updateErr := w.markTaskFailed(task.ID, err.Error(), nil); updateErr != nil {
			slog.Error("Failed to update mirror task failure", "taskID", task.ID, "error", updateErr)
		}
		w.updateBookmarkMirrorStatus(task.TargetType, task.TargetID, model.BookmarkMirrorFailed)
		return true
	}

	if updateErr := w.markTaskSuccess(task.ID); updateErr != nil {
		slog.Error("Failed to update mirror task success", "taskID", task.ID, "error", updateErr)
	}
	w.updateBookmarkMirrorStatus(task.TargetType, task.TargetID, model.BookmarkMirrorDone)
	return true
}

// updateBookmarkMirrorStatus writes the mirror result back to the
// bookmark_illusts row so the scheduler can track progress.
func (w *MirrorWorker) updateBookmarkMirrorStatus(targetType string, targetID int64, status int) {
	if targetType != model.MirrorTargetIllust {
		return
	}
	now := time.Now()
	result := db.DB.Model(&model.BookmarkIllust{}).
		Where("illust_id = ?", targetID).
		Updates(map[string]any{
			"mirror_status": status,
			"updated_at":    now,
		})
	if result.Error != nil {
		slog.Error("Failed to update bookmark mirror_status",
			"illustID", targetID, "status", status, "error", result.Error)
	} else if result.RowsAffected > 0 {
		slog.Debug("Bookmark mirror_status updated",
			"illustID", targetID, "status", status)
	}
}

func (w *MirrorWorker) claimQueuedTask() (model.MirrorTask, bool) {
	var task model.MirrorTask
	now := time.Now()
	err := db.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("status = ?", model.MirrorTaskStatusQueued).
			Order("created_at asc").
			First(&task).Error; err != nil {
			return err
		}
		return tx.Model(&model.MirrorTask{}).
			Where("id = ? AND status = ?", task.ID, model.MirrorTaskStatusQueued).
			Updates(map[string]any{
				"status":        model.MirrorTaskStatusProcessing,
				"attempt_count": gorm.Expr("attempt_count + 1"),
				"locked_at":     now,
				"started_at":    now,
				"updated_at":    now,
			}).Error
	})
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			slog.Error("Failed to claim mirror task", "error", err)
		}
		return model.MirrorTask{}, false
	}
	task.Status = model.MirrorTaskStatusProcessing
	return task, true
}

func (w *MirrorWorker) processIllustTask(task model.MirrorTask) error {
	var user model.PixivUser
	if err := db.DB.Order("updated_at desc").First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("no Pixiv user token available; sync an account before mirroring")
		}
		return fmt.Errorf("query Pixiv user token failed: %w", err)
	}

	detailURL := fmt.Sprintf("https://app-api.pixiv.net/v1/illust/detail?filter=for_android&illust_id=%d", task.TargetID)
	data, detail, err := w.Pixiv.GetIllustDetail(user, task.TargetID)
	if err != nil {
		return fmt.Errorf("request=%s error=%w", detailURL, err)
	}

	imageURLs := collectIllustImageURLs(detail)
	illustDir := filepath.Join(w.MirrorDir, fmt.Sprintf("%d", task.TargetID))
	if err := os.MkdirAll(illustDir, 0755); err != nil {
		return fmt.Errorf("create mirror directory failed: %w", err)
	}

	files, failedURLs := w.downloadImages(task.ID, task.TargetID, imageURLs, illustDir)
	requestURLsJSON, err := json.Marshal(imageURLs)
	if err != nil {
		return fmt.Errorf("encode request URL list failed: %w", err)
	}
	filesJSON, err := json.Marshal(files)
	if err != nil {
		return fmt.Errorf("encode image file list failed: %w", err)
	}
	failedJSON, err := json.Marshal(failedURLs)
	if err != nil {
		return fmt.Errorf("encode retry URL list failed: %w", err)
	}

	now := time.Now()
	record := model.MirrorIllust{
		IllustID:       task.TargetID,
		DetailJSON:     string(data),
		ImageFilesJSON: string(filesJSON),
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	if err := db.DB.Transaction(func(tx *gorm.DB) error {
		var existing model.MirrorIllust
		if err := tx.Where("illust_id = ?", task.TargetID).First(&existing).Error; err == nil {
			if err := tx.Model(&model.MirrorIllust{}).
				Where("illust_id = ?", task.TargetID).
				Updates(map[string]any{
					"detail_json":      record.DetailJSON,
					"image_files_json": record.ImageFilesJSON,
					"updated_at":       now,
				}).Error; err != nil {
				return err
			}
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		} else if err := tx.Create(&record).Error; err != nil {
			return err
		}

		return tx.Model(&model.MirrorTask{}).
			Where("id = ?", task.ID).
			Updates(map[string]any{
				"request_urls_json": string(requestURLsJSON),
				"retry_urls_json":   string(failedJSON),
				"total_count":       len(imageURLs),
				"success_count":     len(files),
				"failed_count":      len(failedURLs),
				"updated_at":        now,
			}).Error
	}); err != nil {
		return err
	}

	if len(files) == 0 {
		return fmt.Errorf("failed to download all %d images for illust_id=%d retry_urls=%s", len(imageURLs), task.TargetID, string(failedJSON))
	}
	if len(failedURLs) > 0 {
		slog.Warn("Mirror task completed with missing images", "taskID", task.ID, "success", len(files), "failed", len(failedURLs))
	}
	slog.Info("Mirror task download finished", "taskID", task.ID, "illustID", task.TargetID, "total", len(imageURLs), "success", len(files), "failed", len(failedURLs))
	return nil
}

func (w *MirrorWorker) processNovelTask(task model.MirrorTask) error {
	var user model.PixivUser
	if err := db.DB.Order("updated_at desc").First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("no Pixiv user token available; sync an account before mirroring")
		}
		return fmt.Errorf("query Pixiv user token failed: %w", err)
	}

	detailData, _, err := w.Pixiv.GetNovelDetail(user, task.TargetID)
	if err != nil {
		return fmt.Errorf("GetNovelDetail novel_id=%d error=%w", task.TargetID, err)
	}

	textData, _, err := w.Pixiv.GetNovelText(user, task.TargetID)
	if err != nil {
		return fmt.Errorf("GetNovelText novel_id=%d error=%w", task.TargetID, err)
	}

	now := time.Now()
	record := model.MirrorNovel{
		NovelID:    task.TargetID,
		DetailJSON: string(detailData),
		TextJSON:   string(textData),
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	if err := db.DB.Transaction(func(tx *gorm.DB) error {
		var existing model.MirrorNovel
		if err := tx.Where("novel_id = ?", task.TargetID).First(&existing).Error; err == nil {
			if err := tx.Model(&model.MirrorNovel{}).
				Where("novel_id = ?", task.TargetID).
				Updates(map[string]any{
					"detail_json": record.DetailJSON,
					"text_json":   record.TextJSON,
					"updated_at":  now,
				}).Error; err != nil {
				return err
			}
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		} else if err := tx.Create(&record).Error; err != nil {
			return err
		}

		return tx.Model(&model.MirrorTask{}).
			Where("id = ?", task.ID).
			Updates(map[string]any{
				"total_count":   1,
				"success_count": 1,
				"failed_count":  0,
				"updated_at":    now,
			}).Error
	}); err != nil {
		return err
	}

	slog.Info("Novel mirror task finished", "taskID", task.ID, "novelID", task.TargetID)
	return nil
}

func (w *MirrorWorker) downloadImages(taskID string, illustID int64, imageURLs []string, illustDir string) ([]ImageFileRecord, []string) {
	type result struct {
		record ImageFileRecord
		url    string
		err    error
	}

	slog.Info("Mirror task download started", "taskID", taskID, "illustID", illustID, "total", len(imageURLs), "concurrency", w.DownloadConcurrency)

	jobs := make(chan string)
	results := make(chan result)
	workerCount := w.DownloadConcurrency
	if len(imageURLs) < workerCount {
		workerCount = len(imageURLs)
	}
	if workerCount <= 0 {
		return nil, nil
	}

	var wg sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for imgURL := range jobs {
				filename := extractFilename(imgURL)
				if filename == "" {
					results <- result{url: imgURL, err: fmt.Errorf("invalid image url filename")}
					continue
				}
				destPath := filepath.Join(illustDir, filename)
				if err := w.Pixiv.DownloadFile(imgURL, destPath); err != nil {
					results <- result{url: imgURL, err: err}
					continue
				}
				results <- result{
					url: imgURL,
					record: ImageFileRecord{
						URL:      imgURL,
						Path:     destPath,
						Filename: filename,
					},
				}
			}
		}()
	}

	go func() {
		for _, imgURL := range imageURLs {
			jobs <- imgURL
		}
		close(jobs)
		wg.Wait()
		close(results)
	}()

	files := make([]ImageFileRecord, 0, len(imageURLs))
	failed := make([]string, 0)
	done := 0
	for res := range results {
		done++
		if res.err != nil {
			failed = append(failed, res.url)
			slog.Info("Mirror image download failed", "taskID", taskID, "illustID", illustID, "done", done, "total", len(imageURLs), "success", len(files), "failed", len(failed), "url", res.url, "error", res.err)
			continue
		}
		files = append(files, res.record)
		slog.Info("Mirror image download succeeded", "taskID", taskID, "illustID", illustID, "done", done, "total", len(imageURLs), "success", len(files), "failed", len(failed), "url", res.url, "file", res.record.Filename)
	}
	return files, failed
}

func (w *MirrorWorker) markTaskSuccess(taskID string) error {
	now := time.Now()
	return db.DB.Model(&model.MirrorTask{}).
		Where("id = ?", taskID).
		Updates(map[string]any{
			"status":        model.MirrorTaskStatusSuccess,
			"error_message": "",
			"finished_at":   now,
			"updated_at":    now,
		}).Error
}

func (w *MirrorWorker) markTaskFailed(taskID string, message string, retryURLsJSON []byte) error {
	now := time.Now()
	updates := map[string]any{
		"status":        model.MirrorTaskStatusFailed,
		"error_message": message,
		"finished_at":   now,
		"updated_at":    now,
	}
	if retryURLsJSON != nil {
		updates["retry_urls_json"] = string(retryURLsJSON)
	}
	return db.DB.Model(&model.MirrorTask{}).Where("id = ?", taskID).Updates(updates).Error
}

// RetryFailedDownloads re-downloads the URLs in retryURLs for the given illust,
// appends newly succeeded files to the existing mirror_illust record, and updates
// the mirror_task counters. Returns (newSuccess, newFailed, error).
func (w *MirrorWorker) RetryFailedDownloads(task model.MirrorTask) (int, int, error) {
	if task.RetryURLsJSON == "" {
		return 0, 0, nil
	}
	var retryURLs []string
	if err := json.Unmarshal([]byte(task.RetryURLsJSON), &retryURLs); err != nil {
		return 0, 0, fmt.Errorf("parse retry_urls_json: %w", err)
	}
	if len(retryURLs) == 0 {
		return 0, 0, nil
	}

	illustDir := filepath.Join(w.MirrorDir, fmt.Sprintf("%d", task.TargetID))
	if err := os.MkdirAll(illustDir, 0755); err != nil {
		return 0, 0, fmt.Errorf("ensure mirror dir: %w", err)
	}

	newFiles, stillFailed := w.downloadImages(task.ID, task.TargetID, retryURLs, illustDir)

	if len(newFiles) == 0 {
		return 0, len(stillFailed), nil
	}

	// Merge new files into the existing mirror_illust.image_files_json.
	var existingFiles []ImageFileRecord
	var record model.MirrorIllust
	if err := db.DB.Where("illust_id = ?", task.TargetID).First(&record).Error; err != nil {
		return 0, 0, fmt.Errorf("read mirror_illust: %w", err)
	}
	if record.ImageFilesJSON != "" {
		_ = json.Unmarshal([]byte(record.ImageFilesJSON), &existingFiles)
	}

	// Deduplicate by filename.
	existingSet := make(map[string]bool)
	for _, f := range existingFiles {
		existingSet[f.Filename] = true
	}
	for _, f := range newFiles {
		if !existingSet[f.Filename] {
			existingFiles = append(existingFiles, f)
		}
	}
	mergedJSON, _ := json.Marshal(existingFiles)

	stillFailedJSON, _ := json.Marshal(stillFailed)
	now := time.Now()

	err := db.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.MirrorIllust{}).
			Where("illust_id = ?", task.TargetID).
			Updates(map[string]any{
				"image_files_json": string(mergedJSON),
				"updated_at":       now,
			}).Error; err != nil {
			return err
		}

		newSuccessCount := task.SuccessCount + len(newFiles)
		newFailedCount := len(stillFailed)
		var retryJSON any
		if len(stillFailed) > 0 {
			retryJSON = string(stillFailedJSON)
		} else {
			retryJSON = ""
		}
		return tx.Model(&model.MirrorTask{}).
			Where("id = ?", task.ID).
			Updates(map[string]any{
				"success_count":   newSuccessCount,
				"failed_count":    newFailedCount,
				"retry_urls_json": retryJSON,
				"updated_at":      now,
			}).Error
	})
	if err != nil {
		return 0, 0, err
	}
	return len(newFiles), len(stillFailed), nil
}

func collectIllustImageURLs(detail PixivIllustDetail) []string {
	seen := make(map[string]bool)
	var urls []string
	add := func(v string) {
		if v == "" || seen[v] {
			return
		}
		seen[v] = true
		urls = append(urls, v)
	}

	add(detail.Illust.MetaSinglePage.OriginalImageURL)
	for _, page := range detail.Illust.MetaPages {
		add(page.ImageUrls.Original)
	}
	return urls
}

func extractFilename(fileURL string) string {
	parsed, err := url.Parse(fileURL)
	if err != nil {
		return ""
	}
	parts := strings.Split(parsed.Path, "/")
	return parts[len(parts)-1]
}
