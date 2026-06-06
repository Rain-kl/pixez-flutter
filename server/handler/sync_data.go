package handler

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"pixez-sync/db"
	"pixez-sync/model"
	"pixez-sync/response"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func computeHash(lines []string) string {
	if len(lines) == 0 {
		return "empty"
	}
	sort.Strings(lines)
	data := strings.Join(lines, "\n")
	hash := md5.Sum([]byte(data))
	return hex.EncodeToString(hash[:])
}

// GetUserDataHashes returns MD5 checksums for each of the 7 tables for a specific user ID.
// @Summary Get synced data hashes
// @Description Returns MD5 checksums for each synced table of a Pixiv user.
// @Tags Sync Data
// @Security BasicAuth
// @Produce json
// @Param pixiv_user_id path string true "Pixiv user ID"
// @Success 200 {object} model.APIResponse{data=model.UserDataHashes}
// @Failure 400 {object} model.ErrorResponse
// @Router /api/pixez/users/{pixiv_user_id}/sync-data/hashes [get]
func GetUserDataHashes(c *gin.Context) {
	userID := c.Param("pixiv_user_id")
	if userID == "" {
		response.RespondBadRequest(c, "pixiv_user_id is required")
		return
	}

	hashes := make(map[string]string)

	// 1. ban_comments
	{
		var records []model.BanComment
		if err := db.DB.Where("pixiv_user_id = ?", userID).Find(&records).Error; err == nil {
			lines := make([]string, len(records))
			for i, r := range records {
				lines[i] = fmt.Sprintf("%s:%s", r.CommentID, r.Name)
			}
			hashes["ban_comments"] = computeHash(lines)
		} else {
			hashes["ban_comments"] = "empty"
		}
	}

	// 2. ban_illusts
	{
		var records []model.BanIllust
		if err := db.DB.Where("pixiv_user_id = ?", userID).Find(&records).Error; err == nil {
			lines := make([]string, len(records))
			for i, r := range records {
				lines[i] = fmt.Sprintf("%s:%s", r.IllustID, r.Name)
			}
			hashes["ban_illusts"] = computeHash(lines)
		} else {
			hashes["ban_illusts"] = "empty"
		}
	}

	// 3. ban_tags
	{
		var records []model.BanTag
		if err := db.DB.Where("pixiv_user_id = ?", userID).Find(&records).Error; err == nil {
			lines := make([]string, len(records))
			for i, r := range records {
				lines[i] = fmt.Sprintf("%s:%s", r.Name, r.TranslateName)
			}
			hashes["ban_tags"] = computeHash(lines)
		} else {
			hashes["ban_tags"] = "empty"
		}
	}

	// 4. ban_users
	{
		var records []model.BanUser
		if err := db.DB.Where("pixiv_user_id = ?", userID).Find(&records).Error; err == nil {
			lines := make([]string, len(records))
			for i, r := range records {
				lines[i] = fmt.Sprintf("%s:%s", r.UserID, r.Name)
			}
			hashes["ban_users"] = computeHash(lines)
		} else {
			hashes["ban_users"] = "empty"
		}
	}

	// 5. illust_histories
	{
		var records []model.IllustHistory
		if err := db.DB.Where("pixiv_user_id = ?", userID).Find(&records).Error; err == nil {
			lines := make([]string, len(records))
			for i, r := range records {
				lines[i] = fmt.Sprintf("%d:%d:%d", r.IllustID, r.UserID, r.Time)
			}
			hashes["illust_histories"] = computeHash(lines)
		} else {
			hashes["illust_histories"] = "empty"
		}
	}

	// 6. novel_histories
	{
		var records []model.NovelHistory
		if err := db.DB.Where("pixiv_user_id = ?", userID).Find(&records).Error; err == nil {
			lines := make([]string, len(records))
			for i, r := range records {
				lines[i] = fmt.Sprintf("%d:%d:%d", r.NovelID, r.UserID, r.Time)
			}
			hashes["novel_histories"] = computeHash(lines)
		} else {
			hashes["novel_histories"] = "empty"
		}
	}

	// 7. tag_histories
	{
		var records []model.TagHistory
		if err := db.DB.Where("pixiv_user_id = ?", userID).Find(&records).Error; err == nil {
			lines := make([]string, len(records))
			for i, r := range records {
				lines[i] = fmt.Sprintf("%s:%s:%d", r.Name, r.TranslatedName, r.Type)
			}
			hashes["tag_histories"] = computeHash(lines)
		} else {
			hashes["tag_histories"] = "empty"
		}
	}

	response.RespondSuccess(c, hashes)
}

// GetUserData fetches all synced user data for a specific user ID.
// @Summary Get synced user data
// @Description Fetches all or selected synced data tables for a Pixiv user.
// @Tags Sync Data
// @Security BasicAuth
// @Produce json
// @Param pixiv_user_id path string true "Pixiv user ID"
// @Param tables query string false "Comma-separated table names, for example ban_comments,illust_histories"
// @Success 200 {object} model.APIResponse{data=model.UserDataPayload}
// @Failure 400 {object} model.ErrorResponse
// @Failure 500 {object} model.ErrorResponse
// @Router /api/pixez/users/{pixiv_user_id}/sync-data [get]
func GetUserData(c *gin.Context) {
	userID := c.Param("pixiv_user_id")
	if userID == "" {
		response.RespondBadRequest(c, "pixiv_user_id is required")
		return
	}

	var payload model.UserDataPayload

	// Query each table for records belonging to the given user ID
	var banComments []model.BanComment
	if err := db.DB.Where("pixiv_user_id = ?", userID).Find(&banComments).Error; err != nil {
		response.RespondErrorWithStatus(c, http.StatusInternalServerError, "failed to fetch ban comments")
		return
	}
	payload.BanComments = &banComments

	var banIllusts []model.BanIllust
	if err := db.DB.Where("pixiv_user_id = ?", userID).Find(&banIllusts).Error; err != nil {
		response.RespondErrorWithStatus(c, http.StatusInternalServerError, "failed to fetch ban illusts")
		return
	}
	payload.BanIllusts = &banIllusts

	var banTags []model.BanTag
	if err := db.DB.Where("pixiv_user_id = ?", userID).Find(&banTags).Error; err != nil {
		response.RespondErrorWithStatus(c, http.StatusInternalServerError, "failed to fetch ban tags")
		return
	}
	payload.BanTags = &banTags

	var banUsers []model.BanUser
	if err := db.DB.Where("pixiv_user_id = ?", userID).Find(&banUsers).Error; err != nil {
		response.RespondErrorWithStatus(c, http.StatusInternalServerError, "failed to fetch ban users")
		return
	}
	payload.BanUsers = &banUsers

	var illustHistories []model.IllustHistory
	if err := db.DB.Where("pixiv_user_id = ?", userID).Find(&illustHistories).Error; err != nil {
		response.RespondErrorWithStatus(c, http.StatusInternalServerError, "failed to fetch illust histories")
		return
	}
	payload.IllustHistories = &illustHistories

	var novelHistories []model.NovelHistory
	if err := db.DB.Where("pixiv_user_id = ?", userID).Find(&novelHistories).Error; err != nil {
		response.RespondErrorWithStatus(c, http.StatusInternalServerError, "failed to fetch novel histories")
		return
	}
	payload.NovelHistories = &novelHistories

	var tagHistories []model.TagHistory
	if err := db.DB.Where("pixiv_user_id = ?", userID).Find(&tagHistories).Error; err != nil {
		response.RespondErrorWithStatus(c, http.StatusInternalServerError, "failed to fetch tag histories")
		return
	}
	payload.TagHistories = &tagHistories

	// Support optional table queries (e.g. GET /sync-data?tables=ban_comments,ban_illusts)
	tablesParam := c.Query("tables")
	if tablesParam != "" {
		requested := make(map[string]bool)
		for _, s := range strings.Split(tablesParam, ",") {
			requested[strings.TrimSpace(s)] = true
		}
		if !requested["ban_comments"] {
			payload.BanComments = nil
		}
		if !requested["ban_illusts"] {
			payload.BanIllusts = nil
		}
		if !requested["ban_tags"] {
			payload.BanTags = nil
		}
		if !requested["ban_users"] {
			payload.BanUsers = nil
		}
		if !requested["illust_histories"] {
			payload.IllustHistories = nil
		}
		if !requested["novel_histories"] {
			payload.NovelHistories = nil
		}
		if !requested["tag_histories"] {
			payload.TagHistories = nil
		}
	}

	response.RespondSuccess(c, payload)
}

// PostUserData replaces all synced user data for a specific user ID.
// It uses a single transaction to first delete all old data for the user, then insert the new data.
// @Summary Replace synced user data
// @Description Replaces the provided synced data tables for a Pixiv user in a single database transaction.
// @Tags Sync Data
// @Security BasicAuth
// @Accept json
// @Produce json
// @Param pixiv_user_id path string true "Pixiv user ID"
// @Param payload body model.UserDataPayload true "Synced user data payload"
// @Success 200 {object} model.MessageResponse
// @Failure 400 {object} model.ErrorResponse
// @Failure 500 {object} model.ErrorResponse
// @Router /api/pixez/users/{pixiv_user_id}/sync-data [post]
func PostUserData(c *gin.Context) {
	userID := c.Param("pixiv_user_id")
	if userID == "" {
		response.RespondBadRequest(c, "pixiv_user_id is required")
		return
	}

	var payload model.UserDataPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		response.RespondBadRequest(c, err.Error())
		return
	}

	// Run all deletions and insertions inside a database transaction
	err := db.DB.Transaction(func(tx *gorm.DB) error {
		// Only delete and insert tables that are explicitly provided (not nil)
		if payload.BanComments != nil {
			if err := tx.Where("pixiv_user_id = ?", userID).Delete(&model.BanComment{}).Error; err != nil {
				return err
			}
			if len(*payload.BanComments) > 0 {
				for i := range *payload.BanComments {
					(*payload.BanComments)[i].PixivUserID = userID
				}
				if err := tx.Create(payload.BanComments).Error; err != nil {
					return err
				}
			}
		}

		if payload.BanIllusts != nil {
			if err := tx.Where("pixiv_user_id = ?", userID).Delete(&model.BanIllust{}).Error; err != nil {
				return err
			}
			if len(*payload.BanIllusts) > 0 {
				for i := range *payload.BanIllusts {
					(*payload.BanIllusts)[i].PixivUserID = userID
				}
				if err := tx.Create(payload.BanIllusts).Error; err != nil {
					return err
				}
			}
		}

		if payload.BanTags != nil {
			if err := tx.Where("pixiv_user_id = ?", userID).Delete(&model.BanTag{}).Error; err != nil {
				return err
			}
			if len(*payload.BanTags) > 0 {
				for i := range *payload.BanTags {
					(*payload.BanTags)[i].PixivUserID = userID
				}
				if err := tx.Create(payload.BanTags).Error; err != nil {
					return err
				}
			}
		}

		if payload.BanUsers != nil {
			if err := tx.Where("pixiv_user_id = ?", userID).Delete(&model.BanUser{}).Error; err != nil {
				return err
			}
			if len(*payload.BanUsers) > 0 {
				for i := range *payload.BanUsers {
					(*payload.BanUsers)[i].PixivUserID = userID
				}
				if err := tx.Create(payload.BanUsers).Error; err != nil {
					return err
				}
			}
		}

		if payload.IllustHistories != nil {
			if err := tx.Where("pixiv_user_id = ?", userID).Delete(&model.IllustHistory{}).Error; err != nil {
				return err
			}
			if len(*payload.IllustHistories) > 0 {
				for i := range *payload.IllustHistories {
					(*payload.IllustHistories)[i].PixivUserID = userID
				}
				if err := tx.Create(payload.IllustHistories).Error; err != nil {
					return err
				}
			}
		}

		if payload.NovelHistories != nil {
			if err := tx.Where("pixiv_user_id = ?", userID).Delete(&model.NovelHistory{}).Error; err != nil {
				return err
			}
			if len(*payload.NovelHistories) > 0 {
				for i := range *payload.NovelHistories {
					(*payload.NovelHistories)[i].PixivUserID = userID
				}
				if err := tx.Create(payload.NovelHistories).Error; err != nil {
					return err
				}
			}
		}

		if payload.TagHistories != nil {
			if err := tx.Where("pixiv_user_id = ?", userID).Delete(&model.TagHistory{}).Error; err != nil {
				return err
			}
			if len(*payload.TagHistories) > 0 {
				for i := range *payload.TagHistories {
					(*payload.TagHistories)[i].PixivUserID = userID
				}
				if err := tx.Create(payload.TagHistories).Error; err != nil {
					return err
				}
			}
		}

		return nil
	})

	if err != nil {
		response.RespondErrorWithStatus(c, http.StatusInternalServerError, "failed to save user data")
		return
	}

	response.RespondSuccessMessage(c, "sync data saved successfully")
}
