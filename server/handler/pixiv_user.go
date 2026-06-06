package handler

import (
	"errors"
	"net/http"

	"pixez-sync/db"
	"pixez-sync/model"
	"pixez-sync/response"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Ping handles the health check and auth verification.
// @Summary Health check
// @Description Verifies that the server is reachable and Basic Auth credentials are valid.
// @Tags System
// @Security BasicAuth
// @Produce json
// @Success 200 {object} model.APIResponse{data=model.PingData}
// @Failure 401 {object} model.ErrorResponse
// @Router /api/pixez/ping [get]
func Ping(c *gin.Context) {
	response.RespondSuccess(c, gin.H{
		"status": "ok",
	})
}

// ListUsers retrieves all saved Pixiv users as safe DTOs (excluding tokens).
// @Summary List Pixiv users
// @Description Retrieves all saved Pixiv users without sensitive token fields.
// @Tags Users
// @Security BasicAuth
// @Produce json
// @Success 200 {object} model.APIResponse{data=[]model.PixivUserSafeDTO}
// @Failure 500 {object} model.ErrorResponse
// @Router /api/pixez/users [get]
func ListUsers(c *gin.Context) {
	var users []model.PixivUser
	if err := db.DB.Order("updated_at desc").Find(&users).Error; err != nil {
		response.RespondErrorWithStatus(c, http.StatusInternalServerError, "failed to fetch users")
		return
	}

	dtos := make([]model.PixivUserSafeDTO, len(users))
	for i, u := range users {
		dtos[i] = u.ToSafeDTO()
	}

	response.RespondSuccess(c, dtos)
}

// GetUser retrieves the full credentials of a specific Pixiv user including tokens.
// @Summary Get Pixiv user credentials
// @Description Retrieves a Pixiv user's stored credentials, including access and refresh tokens.
// @Tags Users
// @Security BasicAuth
// @Produce json
// @Param pixiv_user_id path string true "Pixiv user ID"
// @Success 200 {object} model.APIResponse{data=model.PixivUser}
// @Failure 400 {object} model.ErrorResponse
// @Failure 404 {object} model.ErrorResponse
// @Failure 500 {object} model.ErrorResponse
// @Router /api/pixez/users/{pixiv_user_id} [get]
func GetUser(c *gin.Context) {
	userID := c.Param("pixiv_user_id")
	if userID == "" {
		response.RespondBadRequest(c, "pixiv_user_id is required")
		return
	}

	var user model.PixivUser
	if err := db.DB.Where("pixiv_user_id = ?", userID).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.RespondErrorWithStatus(c, http.StatusNotFound, "user not found")
		} else {
			response.RespondErrorWithStatus(c, http.StatusInternalServerError, "failed to fetch user")
		}
		return
	}

	response.RespondSuccess(c, user)
}

// UpsertUser inserts or updates a Pixiv user's credentials.
// @Summary Upsert Pixiv user credentials
// @Description Inserts or updates a Pixiv user's stored credentials. The path Pixiv user ID is authoritative.
// @Tags Users
// @Security BasicAuth
// @Accept json
// @Produce json
// @Param pixiv_user_id path string true "Pixiv user ID"
// @Param payload body model.PixivUser true "Pixiv user credentials"
// @Success 200 {object} model.MessageResponse
// @Failure 400 {object} model.ErrorResponse
// @Failure 500 {object} model.ErrorResponse
// @Router /api/pixez/users/{pixiv_user_id} [put]
func UpsertUser(c *gin.Context) {
	userID := c.Param("pixiv_user_id")
	if userID == "" {
		response.RespondBadRequest(c, "pixiv_user_id is required")
		return
	}

	var input model.PixivUser
	if err := c.ShouldBindJSON(&input); err != nil {
		response.RespondBadRequest(c, err.Error())
		return
	}

	input.PixivUserID = userID // Ensure the primary key matches the path variable.

	// Use GORM's Save which handles Upsert.
	if err := db.DB.Save(&input).Error; err != nil {
		response.RespondErrorWithStatus(c, http.StatusInternalServerError, "failed to save user credentials")
		return
	}

	response.RespondSuccessMessage(c, "updated successfully")
}

// DeleteUser removes a Pixiv user's credentials from the database.
// @Summary Delete Pixiv user
// @Description Deletes a Pixiv user and all synced data associated with that user.
// @Tags Users
// @Security BasicAuth
// @Produce json
// @Param pixiv_user_id path string true "Pixiv user ID"
// @Success 200 {object} model.MessageResponse
// @Failure 400 {object} model.ErrorResponse
// @Failure 404 {object} model.ErrorResponse
// @Failure 500 {object} model.ErrorResponse
// @Router /api/pixez/users/{pixiv_user_id} [delete]
func DeleteUser(c *gin.Context) {
	userID := c.Param("pixiv_user_id")
	if userID == "" {
		response.RespondBadRequest(c, "pixiv_user_id is required")
		return
	}

	var user model.PixivUser
	if err := db.DB.Where("pixiv_user_id = ?", userID).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.RespondErrorWithStatus(c, http.StatusNotFound, "user not found")
		} else {
			response.RespondErrorWithStatus(c, http.StatusInternalServerError, "failed to fetch user")
		}
		return
	}

	// Transaction to delete user and all their associated synced data
	err := db.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("pixiv_user_id = ?", userID).Delete(&model.BanComment{}).Error; err != nil {
			return err
		}
		if err := tx.Where("pixiv_user_id = ?", userID).Delete(&model.BanIllust{}).Error; err != nil {
			return err
		}
		if err := tx.Where("pixiv_user_id = ?", userID).Delete(&model.BanTag{}).Error; err != nil {
			return err
		}
		if err := tx.Where("pixiv_user_id = ?", userID).Delete(&model.BanUser{}).Error; err != nil {
			return err
		}
		if err := tx.Where("pixiv_user_id = ?", userID).Delete(&model.IllustHistory{}).Error; err != nil {
			return err
		}
		if err := tx.Where("pixiv_user_id = ?", userID).Delete(&model.NovelHistory{}).Error; err != nil {
			return err
		}
		if err := tx.Where("pixiv_user_id = ?", userID).Delete(&model.TagHistory{}).Error; err != nil {
			return err
		}
		if err := tx.Where("pixiv_user_id = ?", userID).Delete(&model.BookmarkIllust{}).Error; err != nil {
			return err
		}
		if err := tx.Where("pixiv_user_id = ?", userID).Delete(&model.BookmarkExportRun{}).Error; err != nil {
			return err
		}

		if err := tx.Delete(&user).Error; err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		response.RespondErrorWithStatus(c, http.StatusInternalServerError, "failed to delete user credentials and data")
		return
	}

	response.RespondSuccessMessage(c, "deleted successfully")
}
