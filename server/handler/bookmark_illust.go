package handler

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"

	"pixez-sync/db"
	"pixez-sync/model"
	"pixez-sync/response"

	"github.com/gin-gonic/gin"
)

const (
	defaultRemovedBookmarkLimit = 30
	maxRemovedBookmarkLimit     = 100
)

// ListRemovedBookmarkIllusts retrieves bookmarked illustrations that disappeared from Pixiv bookmark exports.
// @Summary List removed bookmark illustrations
// @Description Retrieves exported bookmark illustrations that disappeared from Pixiv bookmark lists.
// @Tags Bookmarks
// @Security BasicAuth
// @Produce json
// @Param pixiv_user_id path string true "Pixiv user ID"
// @Param restrict query string false "Bookmark restrict filter: public or private"
// @Param offset query int false "Pagination offset"
// @Param limit query int false "Pagination limit, max 100"
// @Success 200 {object} model.APIResponse
// @Failure 400 {object} model.ErrorResponse
// @Failure 500 {object} model.ErrorResponse
// @Router /api/pixez/users/{pixiv_user_id}/bookmarks/illust/removed [get]
func ListRemovedBookmarkIllusts(c *gin.Context) {
	userID := c.Param("pixiv_user_id")
	if userID == "" {
		response.RespondBadRequest(c, "pixiv_user_id is required")
		return
	}

	offset := parseNonNegativeInt(c.Query("offset"), 0)
	limit := parseNonNegativeInt(c.Query("limit"), defaultRemovedBookmarkLimit)
	if limit <= 0 {
		limit = defaultRemovedBookmarkLimit
	}
	if limit > maxRemovedBookmarkLimit {
		limit = maxRemovedBookmarkLimit
	}

	query := db.DB.
		Where("pixiv_user_id = ? AND removed = ?", userID, true).
		Order("removed_at desc, updated_at desc, id desc")

	if restrict := c.Query("restrict"); restrict == "public" || restrict == "private" {
		query = query.Where("restrict = ?", restrict)
	}

	var total int64
	if err := query.Model(&model.BookmarkIllust{}).Count(&total).Error; err != nil {
		response.RespondErrorWithStatus(c, http.StatusInternalServerError, "failed to count removed bookmarks")
		return
	}

	var records []model.BookmarkIllust
	if err := query.Limit(limit).Offset(offset).Find(&records).Error; err != nil {
		response.RespondErrorWithStatus(c, http.StatusInternalServerError, "failed to fetch removed bookmarks")
		return
	}

	illusts := make([]json.RawMessage, 0, len(records))
	for _, record := range records {
		if !json.Valid([]byte(record.IllustJSON)) {
			continue
		}
		illusts = append(illusts, json.RawMessage(record.IllustJSON))
	}

	nextURL := ""
	nextOffset := offset + limit
	if int64(nextOffset) < total {
		nextURL = buildRemovedBookmarkNextURL(userID, c.Query("restrict"), nextOffset, limit)
	}

	response.RespondSuccess(c, gin.H{
		"illusts":  illusts,
		"next_url": nextURL,
	})
}

func parseNonNegativeInt(raw string, fallback int) int {
	if raw == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil || parsed < 0 {
		return fallback
	}
	return parsed
}

func buildRemovedBookmarkNextURL(userID string, restrict string, offset int, limit int) string {
	values := url.Values{}
	values.Set("offset", strconv.Itoa(offset))
	values.Set("limit", strconv.Itoa(limit))
	if restrict == "public" || restrict == "private" {
		values.Set("restrict", restrict)
	}
	return "/api/pixez/users/" + url.PathEscape(userID) + "/bookmarks/illust/removed?" + values.Encode()
}
