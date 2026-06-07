package handler

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"pixez-sync/db"
	"pixez-sync/model"
	"pixez-sync/response"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

var MirrorDir = "./data/mirror"

// MirrorIllust enqueues an illustration mirror task.
// @Summary Enqueue illustration mirror task
// @Description Enqueues an async task to mirror a Pixiv illustration detail response and its original images.
// @Tags Mirror
// @Security BasicAuth
// @Produce json
// @Param illust_id path int true "Pixiv illustration ID"
// @Success 200 {object} model.APIResponse{data=model.MirrorStatusResponse}
// @Failure 400 {object} model.ErrorResponse
// @Failure 500 {object} model.ErrorResponse
// @Router /api/pixez/illusts/{illust_id}/mirror [post]
func MirrorIllust(c *gin.Context) {
	illustID, ok := parseIllustIDParam(c)
	if !ok {
		return
	}

	task, err := enqueueIllustMirrorTask(illustID)
	if err != nil {
		response.RespondErrorWithStatus(c, http.StatusInternalServerError, "failed to enqueue mirror task")
		return
	}

	response.RespondSuccess(c, gin.H{
		"task_id":           task.ID,
		"illust_id":         illustID,
		"status":            task.Status,
		"mirrored":          isIllustMirrored(illustID),
		"total_count":       task.TotalCount,
		"success_count":     task.SuccessCount,
		"failed_count":      task.FailedCount,
		"request_urls_json": task.RequestURLsJSON,
		"retry_urls_json":   task.RetryURLsJSON,
	})
}

// CheckIllustMirror checks an illustration mirror task.
// @Summary Check illustration mirror status
// @Description Returns the latest mirror task status for a Pixiv illustration.
// @Tags Mirror
// @Security BasicAuth
// @Produce json
// @Param illust_id path int true "Pixiv illustration ID"
// @Success 200 {object} model.APIResponse{data=model.MirrorStatusResponse}
// @Failure 400 {object} model.ErrorResponse
// @Failure 500 {object} model.ErrorResponse
// @Router /api/pixez/illusts/{illust_id}/mirror [get]
func CheckIllustMirror(c *gin.Context) {
	illustID, ok := parseIllustIDParam(c)
	if !ok {
		return
	}

	var task model.MirrorTask
	err := db.DB.Where(
		"target_type = ? AND target_id = ?",
		model.MirrorTargetIllust,
		illustID,
	).First(&task).Error
	if err == nil {
	} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		response.RespondErrorWithStatus(c, http.StatusInternalServerError, "failed to query mirror task")
		return
	}

	response.RespondSuccess(c, gin.H{
		"task_id":           task.ID,
		"illust_id":         illustID,
		"status":            task.Status,
		"mirrored":          task.SuccessCount > 0,
		"total_count":       task.TotalCount,
		"success_count":     task.SuccessCount,
		"failed_count":      task.FailedCount,
		"request_urls_json": task.RequestURLsJSON,
		"retry_urls_json":   task.RetryURLsJSON,
	})
}

// BatchCheckIllustMirror checks mirror status for multiple illustrations.
// @Summary Batch check illustration mirror status
// @Description Accepts a JSON array of illust_ids and returns the set of IDs that have been successfully mirrored. This is a PixEz Sync Backend business operation, not a Pixiv mirror endpoint.
// @Tags Mirror
// @Security BasicAuth
// @Accept json
// @Produce json
// @Param body body object true "JSON object with illust_ids array" example({"illust_ids":[123,456,789]})
// @Success 200 {object} model.APIResponse{data=object}
// @Failure 400 {object} model.ErrorResponse
// @Failure 500 {object} model.ErrorResponse
// @Router /api/pixez/illusts/mirror/batch [post]
func BatchCheckIllustMirror(c *gin.Context) {
	var req struct {
		IllustIDs []int64 `json:"illust_ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.RespondBadRequest(c, "invalid request body: expected {\"illust_ids\": [...]}")
		return
	}
	if len(req.IllustIDs) == 0 {
		response.RespondBadRequest(c, "illust_ids must not be empty")
		return
	}
	if len(req.IllustIDs) > 500 {
		response.RespondBadRequest(c, "illust_ids must not exceed 500 items")
		return
	}

	var tasks []model.MirrorTask
	if err := db.DB.Where(
		"target_type = ? AND target_id IN ? AND success_count > 0",
		model.MirrorTargetIllust,
		req.IllustIDs,
	).Find(&tasks).Error; err != nil {
		response.RespondErrorWithStatus(c, http.StatusInternalServerError, "failed to query mirror tasks")
		return
	}

	mirroredIDs := make([]int64, 0, len(tasks))
	for _, t := range tasks {
		mirroredIDs = append(mirroredIDs, t.TargetID)
	}

	response.RespondSuccess(c, gin.H{
		"mirrored_ids": mirroredIDs,
	})
}

// GetMirroredIllustDetail returns a mirrored Pixiv detail response.
// @Summary Get mirrored Pixiv illustration detail
// @Description Reads a cached Pixiv /v1/illust/detail response and rewrites pximg URLs to this server's /mirror/pximg path. This endpoint intentionally returns the Pixiv response shape, not the standard PixEz Sync API envelope.
// @Tags Pixiv Mirror
// @Security BasicAuth
// @Produce json
// @Param illust_id query int true "Pixiv illustration ID"
// @Success 200 {object} object
// @Failure 400 {object} model.BasicErrorResponse
// @Failure 404 {object} model.BasicErrorResponse
// @Failure 500 {object} model.BasicErrorResponse
// @Router /mirror/v1/illust/detail [get]
func GetMirroredIllustDetail(c *gin.Context) {
	illustIDRaw := c.Query("illust_id")
	illustID, err := strconv.ParseInt(illustIDRaw, 10, 64)
	if illustIDRaw == "" || err != nil || illustID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "illust_id is required"})
		return
	}

	var record model.MirrorIllust
	if err := db.DB.Where("illust_id = ?", illustID).First(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "mirror not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read mirrored data"})
		}
		return
	}

	prefix := mirrorURLPrefix(c)
	escapedPrefix := strings.ReplaceAll(prefix, "/", "\\/")
	dataStr := record.DetailJSON
	dataStr = strings.ReplaceAll(dataStr, "https://i.pximg.net", prefix)
	dataStr = strings.ReplaceAll(dataStr, "https://s.pximg.net", prefix)
	dataStr = strings.ReplaceAll(dataStr, "https:\\/\\/i.pximg.net", escapedPrefix)
	dataStr = strings.ReplaceAll(dataStr, "https:\\/\\/s.pximg.net", escapedPrefix)

	c.Header("Content-Type", "application/json; charset=utf-8")
	c.String(http.StatusOK, dataStr)
}

// ServeMirroredImage serves a mirrored image file.
// @Summary Serve mirrored pximg file
// @Description Serves a locally mirrored image file. Master and square file names are mapped back to the original cached page image when possible.
// @Tags Pixiv Mirror
// @Security BasicAuth
// @Produce octet-stream
// @Param path path string true "pximg path"
// @Success 200 {file} binary
// @Failure 400 {object} model.BasicErrorResponse
// @Failure 404 {object} model.BasicErrorResponse
// @Failure 500 {object} model.BasicErrorResponse
// @Router /mirror/pximg/{path} [get]
var proxyClient = &http.Client{Timeout: 45 * time.Second}

func downloadFileFromPixiv(fileURL string, destPath string) error {
	req, err := http.NewRequest("GET", fileURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Referer", "https://app-api.pixiv.net/")
	req.Header.Set("User-Agent", "PixivAndroidApp/5.0.155 (Android 10.0; Pixel C)")

	resp, err := proxyClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("pixiv returned status %d", resp.StatusCode)
	}

	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

func ServeMirroredImage(c *gin.Context) {
	path := c.Param("path")
	if path == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "path is required"})
		return
	}

	// Sanitize: remove leading slash and reject path traversal
	path = strings.TrimPrefix(path, "/")
	if strings.Contains(path, "..") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid image path"})
		return
	}

	parts := strings.Split(path, "/")
	filename := parts[len(parts)-1]
	if filename == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid image path"})
		return
	}

	var filePath string
	var found bool

	// 1. Try to find the file using findMirroredImageFile (by illust ID mapping)
	idx := strings.Index(filename, "_")
	if idx != -1 {
		illustID := filename[:idx]
		if fp, err := findMirroredImageFile(illustID, filename); err == nil {
			filePath = fp
			found = true
		}
	}

	// 2. Try exact file path match — only within an illust ID directory
	if !found {
		// Extract the first path segment as the potential illust ID directory.
		if firstSlash := strings.Index(path, "/"); firstSlash != -1 {
			illustDirCandidate := path[:firstSlash]
			if isNumericDir(illustDirCandidate) {
				fp := filepath.Join(MirrorDir, path)
				// Verify the resolved path is still under MirrorDir (no traversal).
				absFp, _ := filepath.Abs(fp)
				absMirror, _ := filepath.Abs(MirrorDir)
				if strings.HasPrefix(absFp, absMirror+string(os.PathSeparator)) {
					if _, err := os.Stat(fp); err == nil {
						filePath = fp
						found = true
					}
				}
			}
		}
	}

	// 3. Fallback: proxy download on the fly and cache into the correct illust ID directory
	if !found {
		illustIDStr := ""
		if idx := strings.Index(filename, "_"); idx != -1 {
			illustIDStr = filename[:idx]
		}
		if illustIDStr == "" || !isNumericDir(illustIDStr) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "cannot determine illust ID from filename"})
			return
		}

		destDir := filepath.Join(MirrorDir, illustIDStr)
		destPath := filepath.Join(destDir, filename)

		absDest, _ := filepath.Abs(destPath)
		absMirror, _ := filepath.Abs(MirrorDir)
		if !strings.HasPrefix(absDest, absMirror+string(os.PathSeparator)) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid image path"})
			return
		}

		if err := os.MkdirAll(destDir, 0755); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create directories"})
			return
		}

		pixivURL := "https://i.pximg.net/" + path
		if err := downloadFileFromPixiv(pixivURL, destPath); err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("failed to proxy image from Pixiv: %v", err)})
			return
		}
		filePath = destPath
	}

	c.File(filePath)
}

// isNumericDir returns true if s is a non-empty string of digits only,
// i.e., a valid illust ID directory name like "123456".
func isNumericDir(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func findMirroredImageFile(illustID string, filename string) (string, error) {
	illustDir := filepath.Join(MirrorDir, illustID)
	originalFilename := originalImageFilename(filename)
	filePath := filepath.Join(illustDir, originalFilename)
	if _, err := os.Stat(filePath); err == nil {
		return filePath, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}

	ext := filepath.Ext(originalFilename)
	base := strings.TrimSuffix(originalFilename, ext)
	matches, err := filepath.Glob(filepath.Join(illustDir, base+".*"))
	if err != nil {
		return "", err
	}
	if len(matches) == 0 {
		return "", os.ErrNotExist
	}
	return matches[0], nil
}

func originalImageFilename(filename string) string {
	ext := filepath.Ext(filename)
	base := strings.TrimSuffix(filename, ext)
	if idx := strings.Index(base, "_master"); idx != -1 {
		return base[:idx] + ext
	}
	if idx := strings.Index(base, "_square"); idx != -1 {
		return base[:idx] + ext
	}
	return filename
}

func parseIllustIDParam(c *gin.Context) (int64, bool) {
	raw := c.Param("illust_id")
	illustID, err := strconv.ParseInt(raw, 10, 64)
	if raw == "" || err != nil || illustID <= 0 {
		response.RespondBadRequest(c, "illust_id is required")
		return 0, false
	}
	return illustID, true
}

func enqueueIllustMirrorTask(illustID int64) (model.MirrorTask, error) {
	var existing model.MirrorTask
	err := db.DB.Where(
		"target_type = ? AND target_id = ?",
		model.MirrorTargetIllust,
		illustID,
	).First(&existing).Error
	if err == nil {
		return existing, nil
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return model.MirrorTask{}, err
	}

	payload, _ := json.Marshal(gin.H{"illust_id": illustID})
	now := time.Now()
	task := model.MirrorTask{
		ID:                 newTaskID(),
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

func isIllustMirrored(illustID int64) bool {
	var count int64
	if err := db.DB.Model(&model.MirrorTask{}).Where(
		"target_type = ? AND target_id = ? AND success_count > 0",
		model.MirrorTargetIllust,
		illustID,
	).Count(&count).Error; err != nil {
		return false
	}
	return count > 0
}

func mirrorURLPrefix(c *gin.Context) string {
	scheme := "http"
	if c.Request.TLS != nil || c.Request.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	return scheme + "://" + c.Request.Host + "/mirror/pximg"
}

func newTaskID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err == nil {
		return hex.EncodeToString(b[:])
	}
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// MirrorNovel enqueues a novel mirror task.
// @Summary Enqueue novel mirror task
// @Description Enqueues an async task to mirror a Pixiv novel detail and its text content.
// @Tags Mirror
// @Security BasicAuth
// @Produce json
// @Param novel_id path int true "Pixiv novel ID"
// @Success 200 {object} model.APIResponse{data=model.MirrorStatusResponse}
// @Failure 400 {object} model.ErrorResponse
// @Failure 500 {object} model.ErrorResponse
// @Router /api/pixez/novels/{novel_id}/mirror [post]
func MirrorNovel(c *gin.Context) {
	novelID, ok := parseNovelIDParam(c)
	if !ok {
		return
	}

	task, err := enqueueNovelMirrorTask(novelID)
	if err != nil {
		response.RespondErrorWithStatus(c, http.StatusInternalServerError, "failed to enqueue novel mirror task")
		return
	}

	response.RespondSuccess(c, gin.H{
		"task_id":       task.ID,
		"novel_id":      novelID,
		"status":        task.Status,
		"mirrored":      isNovelMirrored(novelID),
		"total_count":   task.TotalCount,
		"success_count": task.SuccessCount,
		"failed_count":  task.FailedCount,
	})
}

// CheckNovelMirror checks a novel mirror task.
// @Summary Check novel mirror status
// @Description Returns the latest mirror task status for a Pixiv novel.
// @Tags Mirror
// @Security BasicAuth
// @Produce json
// @Param novel_id path int true "Pixiv novel ID"
// @Success 200 {object} model.APIResponse{data=model.MirrorStatusResponse}
// @Failure 400 {object} model.ErrorResponse
// @Failure 500 {object} model.ErrorResponse
// @Router /api/pixez/novels/{novel_id}/mirror [get]
func CheckNovelMirror(c *gin.Context) {
	novelID, ok := parseNovelIDParam(c)
	if !ok {
		return
	}

	var task model.MirrorTask
	err := db.DB.Where(
		"target_type = ? AND target_id = ?",
		model.MirrorTargetNovel,
		novelID,
	).First(&task).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		response.RespondErrorWithStatus(c, http.StatusInternalServerError, "failed to query novel mirror task")
		return
	}

	response.RespondSuccess(c, gin.H{
		"task_id":       task.ID,
		"novel_id":      novelID,
		"status":        task.Status,
		"mirrored":      task.SuccessCount > 0,
		"total_count":   task.TotalCount,
		"success_count": task.SuccessCount,
		"failed_count":  task.FailedCount,
	})
}

// GetMirroredNovelDetail returns a mirrored Pixiv novel detail response.
// @Summary Get mirrored Pixiv novel detail
// @Description Reads a cached Pixiv /v2/novel/detail response and rewrites pximg URLs to this server's /mirror/pximg path.
// @Tags Pixiv Mirror
// @Security BasicAuth
// @Produce json
// @Param novel_id query int true "Pixiv novel ID"
// @Success 200 {object} object
// @Failure 400 {object} model.BasicErrorResponse
// @Failure 404 {object} model.BasicErrorResponse
// @Failure 500 {object} model.BasicErrorResponse
// @Router /mirror/v1/novel/detail [get]
func GetMirroredNovelDetail(c *gin.Context) {
	novelIDRaw := c.Query("novel_id")
	novelID, err := strconv.ParseInt(novelIDRaw, 10, 64)
	if novelIDRaw == "" || err != nil || novelID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "novel_id is required"})
		return
	}

	var record model.MirrorNovel
	if err := db.DB.Where("novel_id = ?", novelID).First(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "novel mirror not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read mirrored novel data"})
		}
		return
	}

	prefix := mirrorURLPrefix(c)
	escapedPrefix := strings.ReplaceAll(prefix, "/", "\\/")
	dataStr := record.DetailJSON
	dataStr = strings.ReplaceAll(dataStr, "https://i.pximg.net", prefix)
	dataStr = strings.ReplaceAll(dataStr, "https://s.pximg.net", prefix)
	dataStr = strings.ReplaceAll(dataStr, "https:\\/\\/i.pximg.net", escapedPrefix)
	dataStr = strings.ReplaceAll(dataStr, "https:\\/\\/s.pximg.net", escapedPrefix)

	c.Header("Content-Type", "application/json; charset=utf-8")
	c.String(http.StatusOK, dataStr)
}

// GetMirroredNovelText returns the mirrored novel text content.
// @Summary Get mirrored Pixiv novel text
// @Description Reads the cached webview novel JSON (NovelWebResponse) from the mirror and rewrites pximg URLs to this server's /mirror/pximg path.
// @Tags Pixiv Mirror
// @Security BasicAuth
// @Produce json
// @Param novel_id query int true "Pixiv novel ID"
// @Success 200 {object} object
// @Failure 400 {object} model.BasicErrorResponse
// @Failure 404 {object} model.BasicErrorResponse
// @Failure 500 {object} model.BasicErrorResponse
// @Router /mirror/webview/v2/novel [get]
func GetMirroredNovelText(c *gin.Context) {
	novelIDRaw := c.Query("novel_id")
	novelID, err := strconv.ParseInt(novelIDRaw, 10, 64)
	if novelIDRaw == "" || err != nil || novelID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "novel_id is required"})
		return
	}

	var record model.MirrorNovel
	if err := db.DB.Where("novel_id = ?", novelID).First(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "novel mirror not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read mirrored novel data"})
		}
		return
	}

	prefix := mirrorURLPrefix(c)
	escapedPrefix := strings.ReplaceAll(prefix, "/", "\\/")
	dataStr := record.TextJSON
	dataStr = strings.ReplaceAll(dataStr, "https://i.pximg.net", prefix)
	dataStr = strings.ReplaceAll(dataStr, "https://s.pximg.net", prefix)
	dataStr = strings.ReplaceAll(dataStr, "https:\\/\\/i.pximg.net", escapedPrefix)
	dataStr = strings.ReplaceAll(dataStr, "https:\\/\\/s.pximg.net", escapedPrefix)

	c.Header("Content-Type", "application/json; charset=utf-8")
	c.String(http.StatusOK, dataStr)
}

func parseNovelIDParam(c *gin.Context) (int64, bool) {
	raw := c.Param("novel_id")
	novelID, err := strconv.ParseInt(raw, 10, 64)
	if raw == "" || err != nil || novelID <= 0 {
		response.RespondBadRequest(c, "novel_id is required")
		return 0, false
	}
	return novelID, true
}

func enqueueNovelMirrorTask(novelID int64) (model.MirrorTask, error) {
	var existing model.MirrorTask
	err := db.DB.Where(
		"target_type = ? AND target_id = ?",
		model.MirrorTargetNovel,
		novelID,
	).First(&existing).Error
	if err == nil {
		return existing, nil
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return model.MirrorTask{}, err
	}

	payload, _ := json.Marshal(gin.H{"novel_id": novelID})
	now := time.Now()
	task := model.MirrorTask{
		ID:                 newTaskID(),
		TaskType:           model.MirrorTaskTypeNovel,
		TargetType:         model.MirrorTargetNovel,
		TargetID:           novelID,
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

func isNovelMirrored(novelID int64) bool {
	var count int64
	if err := db.DB.Model(&model.MirrorTask{}).Where(
		"target_type = ? AND target_id = ? AND success_count > 0",
		model.MirrorTargetNovel,
		novelID,
	).Count(&count).Error; err != nil {
		return false
	}
	return count > 0
}

// ListMirroredIllusts returns a paginated list of mirrored illustrations.
func ListMirroredIllusts(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	var tasks []model.MirrorTask
	var total int64

	query := db.DB.Model(&model.MirrorTask{}).
		Where("task_type = ? AND target_type = ?", model.MirrorTaskTypeIllust, model.MirrorTargetIllust)
	query.Count(&total)

	if err := query.
		Order("updated_at DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&tasks).Error; err != nil {
		response.RespondErrorWithStatus(c, http.StatusInternalServerError, "failed to list mirror tasks")
		return
	}

	type MirrorIllustItem struct {
		TaskID       string    `json:"task_id"`
		IllustID     int64     `json:"illust_id"`
		Status       string    `json:"status"`
		SuccessCount int       `json:"success_count"`
		TotalCount   int       `json:"total_count"`
		HasMirror    bool      `json:"has_mirror"`
		CreatedAt    time.Time `json:"created_at"`
		UpdatedAt    time.Time `json:"updated_at"`
	}

	items := make([]MirrorIllustItem, 0, len(tasks))
	for _, t := range tasks {
		items = append(items, MirrorIllustItem{
			TaskID:       t.ID,
			IllustID:     t.TargetID,
			Status:       t.Status,
			SuccessCount: t.SuccessCount,
			TotalCount:   t.TotalCount,
			HasMirror:    t.SuccessCount > 0,
			CreatedAt:    t.CreatedAt,
			UpdatedAt:    t.UpdatedAt,
		})
	}

	response.RespondSuccess(c, gin.H{
		"items":     items,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// ListMirroredNovels returns a paginated list of mirrored novels.
func ListMirroredNovels(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	var tasks []model.MirrorTask
	var total int64

	query := db.DB.Model(&model.MirrorTask{}).
		Where("task_type = ? AND target_type = ?", model.MirrorTaskTypeNovel, model.MirrorTargetNovel)
	query.Count(&total)

	if err := query.
		Order("updated_at DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&tasks).Error; err != nil {
		response.RespondErrorWithStatus(c, http.StatusInternalServerError, "failed to list mirror tasks")
		return
	}

	type MirrorNovelItem struct {
		TaskID    string    `json:"task_id"`
		NovelID   int64     `json:"novel_id"`
		Status    string    `json:"status"`
		HasMirror bool      `json:"has_mirror"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
	}

	items := make([]MirrorNovelItem, 0, len(tasks))
	for _, t := range tasks {
		items = append(items, MirrorNovelItem{
			TaskID:    t.ID,
			NovelID:   t.TargetID,
			Status:    t.Status,
			HasMirror: t.SuccessCount > 0,
			CreatedAt: t.CreatedAt,
			UpdatedAt: t.UpdatedAt,
		})
	}

	response.RespondSuccess(c, gin.H{
		"items":     items,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// DeleteMirroredIllust deletes a mirrored illustration and its task.
func DeleteMirroredIllust(c *gin.Context) {
	illustID, ok := parseIllustIDParam(c)
	if !ok {
		return
	}

	db.DB.Where("illust_id = ?", illustID).Delete(&model.MirrorIllust{})
	illustDir := filepath.Join(MirrorDir, strconv.FormatInt(illustID, 10))
	os.RemoveAll(illustDir)
	db.DB.Where("task_type = ? AND target_type = ? AND target_id = ?",
		model.MirrorTaskTypeIllust, model.MirrorTargetIllust, illustID).
		Delete(&model.MirrorTask{})

	response.RespondSuccess(c, gin.H{"deleted": true, "illust_id": illustID})
}

// DeleteMirroredNovel deletes a mirrored novel and its task.
func DeleteMirroredNovel(c *gin.Context) {
	novelID, ok := parseNovelIDParam(c)
	if !ok {
		return
	}

	db.DB.Where("novel_id = ?", novelID).Delete(&model.MirrorNovel{})
	db.DB.Where("task_type = ? AND target_type = ? AND target_id = ?",
		model.MirrorTaskTypeNovel, model.MirrorTargetNovel, novelID).
		Delete(&model.MirrorTask{})

	response.RespondSuccess(c, gin.H{"deleted": true, "novel_id": novelID})
}

// BatchDeleteMirroredItems batch-deletes mirrored items by IDs.
func BatchDeleteMirroredItems(c *gin.Context) {
	var body struct {
		TargetType string  `json:"target_type"` // "illust" or "novel"
		IDs        []int64 `json:"ids"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || len(body.IDs) == 0 {
		response.RespondBadRequest(c, "invalid request body")
		return
	}

	deletedCount := 0
	for _, id := range body.IDs {
		switch body.TargetType {
		case "illust":
			db.DB.Where("illust_id = ?", id).Delete(&model.MirrorIllust{})
			illustDir := filepath.Join(MirrorDir, strconv.FormatInt(id, 10))
			os.RemoveAll(illustDir)
			db.DB.Where("task_type = ? AND target_type = ? AND target_id = ?",
				model.MirrorTaskTypeIllust, model.MirrorTargetIllust, id).
				Delete(&model.MirrorTask{})
			deletedCount++
		case "novel":
			db.DB.Where("novel_id = ?", id).Delete(&model.MirrorNovel{})
			db.DB.Where("task_type = ? AND target_type = ? AND target_id = ?",
				model.MirrorTaskTypeNovel, model.MirrorTargetNovel, id).
				Delete(&model.MirrorTask{})
			deletedCount++
		}
	}

	response.RespondSuccess(c, gin.H{"deleted_count": deletedCount})
}
