package handler

import (
	"errors"
	"net/http"

	"pixez-sync/db"
	"pixez-sync/model"
	"pixez-sync/response"
	"pixez-sync/service"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

var BookmarkExportWorker *service.BookmarkExportWorker

// ListScheduledTasks retrieves all scheduled tasks and their status.
// @Summary List scheduled tasks
// @Description Retrieves all scheduled background tasks with their run status, timing, and error info.
// @Tags System
// @Security BasicAuth
// @Produce json
// @Success 200 {object} model.APIResponse{data=[]model.ScheduledTask}
// @Failure 500 {object} model.ErrorResponse
// @Router /api/pixez/scheduled-tasks [get]
func ListScheduledTasks(c *gin.Context) {
	var tasks []model.ScheduledTask
	if err := db.DB.Order("name").Find(&tasks).Error; err != nil {
		response.RespondErrorWithStatus(c, http.StatusInternalServerError, "failed to fetch scheduled tasks")
		return
	}
	response.RespondSuccess(c, tasks)
}

// GetBookmarkExportTask retrieves the bookmark export scheduled task status.
// @Summary Get bookmark export task
// @Description Retrieves the bookmark export scheduled task status.
// @Tags System
// @Security BasicAuth
// @Produce json
// @Success 200 {object} model.APIResponse{data=model.ScheduledTask}
// @Failure 404 {object} model.ErrorResponse
// @Failure 500 {object} model.ErrorResponse
// @Router /api/pixez/scheduled-tasks/bookmark-export [get]
func GetBookmarkExportTask(c *gin.Context) {
	var task model.ScheduledTask
	if err := db.DB.First(&task, "name = ?", model.ScheduledTaskBookmarkExport).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.RespondErrorWithStatus(c, http.StatusNotFound, "scheduled task not found")
			return
		}
		response.RespondErrorWithStatus(c, http.StatusInternalServerError, "failed to fetch scheduled task")
		return
	}
	response.RespondSuccess(c, task)
}

// RunBookmarkExportTask triggers the bookmark export scheduled task immediately.
// @Summary Run bookmark export task
// @Description Starts the bookmark export task asynchronously. If the task is already running, returns 409.
// @Tags System
// @Security BasicAuth
// @Produce json
// @Success 200 {object} model.APIResponse
// @Failure 409 {object} model.ErrorResponse
// @Failure 500 {object} model.ErrorResponse
// @Router /api/pixez/scheduled-tasks/bookmark-export/run [post]
func RunBookmarkExportTask(c *gin.Context) {
	if BookmarkExportWorker == nil {
		response.RespondErrorWithStatus(c, http.StatusInternalServerError, "bookmark export worker is not configured")
		return
	}
	if !BookmarkExportWorker.RunOnceAsync() {
		response.RespondErrorWithStatus(c, http.StatusConflict, "bookmark export task is already running")
		return
	}
	response.RespondSuccess(c, gin.H{
		"name":   model.ScheduledTaskBookmarkExport,
		"status": model.ScheduledTaskStatusRunning,
	})
}
