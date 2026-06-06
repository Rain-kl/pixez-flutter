package handler

import (
	"net/http"

	"pixez-sync/db"
	"pixez-sync/model"
	"pixez-sync/response"

	"github.com/gin-gonic/gin"
)

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
