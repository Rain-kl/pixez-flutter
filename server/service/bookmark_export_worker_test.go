package service

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"pixez-sync/db"
	"pixez-sync/model"
)

func TestBookmarkExportWorkerUpsertsAndMarksMissingRemoved(t *testing.T) {
	setupMirrorWorkerTestDB(t)
	if err := db.DB.AutoMigrate(&model.BookmarkExportRun{}, &model.BookmarkIllust{}); err != nil {
		t.Fatalf("failed to migrate bookmark tables: %v", err)
	}

	user := model.PixivUser{
		PixivUserID:  "58490893",
		Name:         "TestUser",
		Account:      "test_acc",
		AccessToken:  "valid_access_token",
		RefreshToken: "valid_refresh_token",
		UpdatedAt:    time.Now(),
	}
	if err := db.DB.Create(&user).Error; err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	call := 0
	originalTransport := pixivClient.Transport
	pixivClient.Transport = &mockPixivTransport{
		roundTripFunc: func(req *http.Request) (*http.Response, error) {
			if req.URL.Host != "app-api.pixiv.net" || req.URL.Path != "/v1/user/bookmarks/illust" {
				return &http.Response{
					StatusCode: http.StatusNotFound,
					Body:       io.NopCloser(bytes.NewBufferString("not found")),
					Header:     make(http.Header),
				}, nil
			}
			if got := req.Header.Get("User-Agent"); got != pixivAppUserAgent {
				t.Fatalf("unexpected Pixiv User-Agent: %s", got)
			}
			if !strings.HasPrefix(req.Header.Get("Authorization"), "Bearer ") {
				t.Fatalf("expected Authorization bearer header")
			}
			call++
			body := `{"illusts":[` + bookmarkIllustJSON(100, "first") + `,` + bookmarkIllustJSON(200, "second") + `],"next_url":null}`
			if call == 2 {
				body = `{"illusts":[` + bookmarkIllustJSON(100, "first updated") + `],"next_url":null}`
			} else if call == 3 {
				body = `{"illusts":[` + bookmarkLimitUnknownIllustJSON(100, "first hidden") + `],"next_url":null}`
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewBufferString(body)),
				Header:     make(http.Header),
			}, nil
		},
	}
	t.Cleanup(func() {
		pixivClient.Transport = originalTransport
	})

	worker := NewBookmarkExportWorker(24 * time.Hour)
	result, err := worker.exportUserForRestrict(user, "public")
	if err != nil {
		t.Fatalf("first export failed: %v", err)
	}
	if result.TotalCount != 2 || result.NewCount != 2 || result.RemovedCount != 0 {
		t.Fatalf("unexpected first export result: %+v", result)
	}

	result, err = worker.exportUserForRestrict(user, "public")
	if err != nil {
		t.Fatalf("second export failed: %v", err)
	}
	if result.TotalCount != 1 || result.UpdatedCount != 0 || result.RemovedCount != 1 {
		t.Fatalf("unexpected second export result: %+v", result)
	}

	var kept model.BookmarkIllust
	if err := db.DB.First(&kept, "pixiv_user_id = ? AND illust_id = ?", user.PixivUserID, 100).Error; err != nil {
		t.Fatalf("expected kept bookmark: %v", err)
	}
	if kept.Removed || !strings.Contains(kept.IllustJSON, "first") || strings.Contains(kept.IllustJSON, "first updated") {
		t.Fatalf("expected bookmark 100 to stay unchanged and active: removed=%v json=%s", kept.Removed, kept.IllustJSON)
	}

	var removed model.BookmarkIllust
	if err := db.DB.First(&removed, "pixiv_user_id = ? AND illust_id = ?", user.PixivUserID, 200).Error; err != nil {
		t.Fatalf("expected removed bookmark record to remain: %v", err)
	}
	if !removed.Removed || removed.RemovedAt == nil {
		t.Fatalf("expected bookmark 200 to be marked removed, got removed=%v removedAt=%v", removed.Removed, removed.RemovedAt)
	}

	var hiddenIllust PixivIllust
	if err := json.Unmarshal([]byte(bookmarkLimitUnknownIllustJSON(100, "first hidden")), &hiddenIllust); err != nil {
		t.Fatalf("failed to unmarshal limit_unknown illust: %v", err)
	}
	if !pixivIllustHasLimitUnknownImage(hiddenIllust) {
		t.Fatalf("expected test helper to generate limit_unknown image urls: %+v", hiddenIllust.ImageUrls)
	}

	result, err = worker.exportUserForRestrict(user, "public")
	if err != nil {
		t.Fatalf("third export failed: %v", err)
	}
	if result.TotalCount != 1 || result.UpdatedCount != 0 || result.RemovedCount != 1 {
		t.Fatalf("unexpected third export result: %+v", result)
	}
	if err := db.DB.First(&kept, "pixiv_user_id = ? AND illust_id = ?", user.PixivUserID, 100).Error; err != nil {
		t.Fatalf("expected hidden bookmark: %v", err)
	}
	if !kept.Removed || kept.RemovedAt == nil {
		t.Fatalf("expected limit_unknown bookmark 100 to be marked removed: removed=%v removedAt=%v", kept.Removed, kept.RemovedAt)
	}

	var count int64
	db.DB.Model(&model.BookmarkIllust{}).Where("pixiv_user_id = ?", user.PixivUserID).Count(&count)
	if count != 2 {
		t.Fatalf("expected removed bookmark to stay in database, got count=%d", count)
	}
}

func bookmarkLimitUnknownIllustJSON(id int64, title string) string {
	return strings.Replace(
		bookmarkIllustJSON(id, title),
		`"image_urls": {
			"square_medium": "https://i.pximg.net/`+intToString(id)+`_p0_square1200.jpg",
			"medium": "https://i.pximg.net/`+intToString(id)+`_p0_master1200.jpg",
			"large": "https://i.pximg.net/`+intToString(id)+`_p0_master1200.jpg"
		}`,
		`"image_urls": {
			"square_medium": "https://s.pximg.net/common/images/limit_unknown_360.png",
			"medium": "https://s.pximg.net/common/images/limit_unknown_360.png",
			"large": "https://s.pximg.net/common/images/limit_unknown_360.png"
		}`,
		1,
	)
}

func bookmarkIllustJSON(id int64, title string) string {
	return `{
		"id": ` + intToString(id) + `,
		"title": "` + title + `",
		"type": "illust",
		"image_urls": {
			"square_medium": "https://i.pximg.net/` + intToString(id) + `_p0_square1200.jpg",
			"medium": "https://i.pximg.net/` + intToString(id) + `_p0_master1200.jpg",
			"large": "https://i.pximg.net/` + intToString(id) + `_p0_master1200.jpg"
		},
		"caption": "",
		"restrict": 0,
		"user": {
			"id": 43053815,
			"name": "Author",
			"account": "author",
			"profile_image_urls": {"medium": "https://i.pximg.net/user.jpg"},
			"is_followed": true,
			"is_accept_request": false
		},
		"tags": [{"name": "tag", "translated_name": null}],
		"tools": [],
		"create_date": "2026-06-06T19:40:26+09:00",
		"page_count": 1,
		"width": 1664,
		"height": 2448,
		"sanity_level": 6,
		"x_restrict": 1,
		"series": null,
		"meta_single_page": {},
		"meta_pages": [],
		"total_view": 147,
		"total_bookmarks": 31,
		"is_bookmarked": true,
		"visible": true,
		"is_muted": false,
		"illust_ai_type": 2,
		"illust_book_style": 0,
		"request": null,
		"restriction_attributes": ["restricted_mode"]
	}`
}

func intToString(v int64) string {
	return strconv.FormatInt(v, 10)
}
