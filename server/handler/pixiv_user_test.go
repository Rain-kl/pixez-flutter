package handler_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"pixez-sync/db"
	"pixez-sync/handler"
	"pixez-sync/middleware"
	"pixez-sync/model"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	api := r.Group("/api/pixez", middleware.BasicAuth("admin", "test12345"))
	{
		api.GET("/ping", handler.Ping)
		api.GET("/users", handler.ListUsers)
		api.GET("/users/:pixiv_user_id", handler.GetUser)
		api.PUT("/users/:pixiv_user_id", handler.UpsertUser)
		api.DELETE("/users/:pixiv_user_id", handler.DeleteUser)
		api.GET("/users/:pixiv_user_id/sync-data", handler.GetUserData)
		api.POST("/users/:pixiv_user_id/sync-data", handler.PostUserData)
		api.GET("/users/:pixiv_user_id/sync-data/hashes", handler.GetUserDataHashes)
	}
	return r
}

func TestPixivUserFlow(t *testing.T) {
	// Set up memory database for GORM
	gormDB, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open in-memory database: %v", err)
	}

	// Migrate models
	if err := gormDB.AutoMigrate(
		&model.PixivUser{},
		&model.BanComment{},
		&model.BanIllust{},
		&model.BanTag{},
		&model.BanUser{},
		&model.IllustHistory{},
		&model.NovelHistory{},
		&model.TagHistory{},
		&model.BookmarkIllust{},
		&model.BookmarkExportRun{},
	); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}

	// Assign to db.DB package variable
	db.DB = gormDB

	router := setupTestRouter()

	// 1. Test ping without auth (should fail with 401)
	t.Run("PingUnauthorized", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/pixez/ping", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", w.Code)
		}
	})

	// 2. Test ping with auth (should succeed)
	t.Run("PingAuthorized", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/pixez/ping", nil)
		req.SetBasicAuth("admin", "test12345")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}

		var res map[string]any
		json.Unmarshal(w.Body.Bytes(), &res)
		if res["success"] != true {
			t.Errorf("expected success true, got false: %s", w.Body.String())
		}
		data, _ := res["data"].(map[string]any)
		if data["status"] != "ok" {
			t.Errorf("unexpected body: %s", w.Body.String())
		}
	})

	// 3. Test list users (should be empty initially)
	t.Run("ListUsersEmpty", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/pixez/users", nil)
		req.SetBasicAuth("admin", "test12345")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}

		var res struct {
			Success bool                     `json:"success"`
			Message string                   `json:"message"`
			Data    []model.PixivUserSafeDTO `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
			t.Fatalf("failed to unmarshal: %v", err)
		}
		if !res.Success {
			t.Errorf("expected success true, got false")
		}
		if len(res.Data) != 0 {
			t.Errorf("expected 0 users, got %d", len(res.Data))
		}
	})

	// 4. Test PUT user (create new)
	userPayload := model.PixivUser{
		Name:         "PainterName",
		Account:      "painter_acc",
		MailAddress:  "painter@pixiv.net",
		UserImage:    "https://example.com/img.jpg",
		AccessToken:  "token123",
		RefreshToken: "refresh123",
		DeviceToken:  "device123",
		IsPremium:    1,
	}

	t.Run("UpsertUserCreate", func(t *testing.T) {
		payloadBytes, _ := json.Marshal(userPayload)
		req, _ := http.NewRequest("PUT", "/api/pixez/users/987654", bytes.NewBuffer(payloadBytes))
		req.Header.Set("Content-Type", "application/json")
		req.SetBasicAuth("admin", "test12345")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}
	})

	// 5. Test list users (should contain 1 user, without tokens)
	t.Run("ListUsersAfterCreate", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/pixez/users", nil)
		req.SetBasicAuth("admin", "test12345")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}

		var res struct {
			Success bool                     `json:"success"`
			Message string                   `json:"message"`
			Data    []model.PixivUserSafeDTO `json:"data"`
		}
		json.Unmarshal(w.Body.Bytes(), &res)
		if !res.Success {
			t.Errorf("expected success true, got false")
		}
		if len(res.Data) != 1 {
			t.Errorf("expected 1 user, got %d", len(res.Data))
		}
		if res.Data[0].PixivUserID != "987654" || res.Data[0].Name != "PainterName" || res.Data[0].IsPremium != 1 {
			t.Errorf("unexpected user: %+v", res.Data[0])
		}
	})

	// 6. Test GET user detail (should contain tokens)
	t.Run("GetUserDetail", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/pixez/users/987654", nil)
		req.SetBasicAuth("admin", "test12345")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}

		var res struct {
			Success bool            `json:"success"`
			Message string          `json:"message"`
			Data    model.PixivUser `json:"data"`
		}
		json.Unmarshal(w.Body.Bytes(), &res)
		if !res.Success {
			t.Errorf("expected success true, got false")
		}
		u := res.Data
		if u.PixivUserID != "987654" || u.AccessToken != "token123" || u.RefreshToken != "refresh123" {
			t.Errorf("unexpected user detail: %+v", u)
		}
	})

	// 7. Test PUT user (update existing)
	updatePayload := userPayload
	updatePayload.Name = "NewPainterName"
	updatePayload.AccessToken = "new_token123"

	t.Run("UpsertUserUpdate", func(t *testing.T) {
		payloadBytes, _ := json.Marshal(updatePayload)
		req, _ := http.NewRequest("PUT", "/api/pixez/users/987654", bytes.NewBuffer(payloadBytes))
		req.Header.Set("Content-Type", "application/json")
		req.SetBasicAuth("admin", "test12345")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}

		// Verify in DB
		var check model.PixivUser
		gormDB.First(&check, "pixiv_user_id = ?", "987654")
		if check.Name != "NewPainterName" || check.AccessToken != "new_token123" {
			t.Errorf("update failed: %+v", check)
		}
	})

	// 7b. Test GET sync-data (should be empty initially)
	t.Run("GetSyncDataEmpty", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/pixez/users/987654/sync-data", nil)
		req.SetBasicAuth("admin", "test12345")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}

		var res struct {
			Success bool                  `json:"success"`
			Message string                `json:"message"`
			Data    model.UserDataPayload `json:"data"`
		}
		json.Unmarshal(w.Body.Bytes(), &res)
		if !res.Success {
			t.Errorf("expected success true, got false")
		}
		data := res.Data
		if data.BanTags == nil || len(*data.BanTags) != 0 || data.IllustHistories == nil || len(*data.IllustHistories) != 0 {
			t.Errorf("expected empty sync data, got: %+v", data)
		}
	})

	// 7c. Test POST sync-data (upload data)
	banTags := []model.BanTag{
		{Name: "tag1", TranslateName: "tag_one"},
	}
	illustHistories := []model.IllustHistory{
		{IllustID: 1234, UserID: 5678, PictureUrl: "http://example.com/img.png", Title: "t", UserName: "u", Time: 99999},
	}
	syncPayload := model.UserDataPayload{
		BanTags:         &banTags,
		IllustHistories: &illustHistories,
	}

	t.Run("PostSyncData", func(t *testing.T) {
		payloadBytes, _ := json.Marshal(syncPayload)
		req, _ := http.NewRequest("POST", "/api/pixez/users/987654/sync-data", bytes.NewBuffer(payloadBytes))
		req.Header.Set("Content-Type", "application/json")
		req.SetBasicAuth("admin", "test12345")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}
	})

	// 7d. Test GET sync-data (should contain uploaded data)
	t.Run("GetSyncDataNotEmpty", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/pixez/users/987654/sync-data", nil)
		req.SetBasicAuth("admin", "test12345")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}

		var res struct {
			Success bool                  `json:"success"`
			Message string                `json:"message"`
			Data    model.UserDataPayload `json:"data"`
		}
		json.Unmarshal(w.Body.Bytes(), &res)
		if !res.Success {
			t.Errorf("expected success true, got false")
		}
		data := res.Data
		if data.BanTags == nil || len(*data.BanTags) != 1 || (*data.BanTags)[0].Name != "tag1" || data.IllustHistories == nil || len(*data.IllustHistories) != 1 || (*data.IllustHistories)[0].IllustID != 1234 {
			t.Errorf("unexpected sync data: %+v", data)
		}
	})

	// 7d2. Test GET sync-data/hashes
	t.Run("GetSyncDataHashes", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/pixez/users/987654/sync-data/hashes", nil)
		req.SetBasicAuth("admin", "test12345")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}

		var res struct {
			Success bool              `json:"success"`
			Message string            `json:"message"`
			Data    map[string]string `json:"data"`
		}
		json.Unmarshal(w.Body.Bytes(), &res)
		if !res.Success {
			t.Errorf("expected success true, got false")
		}
		hashes := res.Data
		if hashes["ban_tags"] == "empty" || hashes["illust_histories"] == "empty" || hashes["ban_comments"] != "empty" {
			t.Errorf("unexpected hashes: %+v", hashes)
		}
	})

	// 7e. Test POST sync-data (overwrite data, 先删后插)
	banTags2 := []model.BanTag{
		{Name: "tag2", TranslateName: "tag_two"},
	}
	illustHistoriesEmpty := []model.IllustHistory{}
	overwritePayload := model.UserDataPayload{
		BanTags:         &banTags2,
		IllustHistories: &illustHistoriesEmpty,
	}

	t.Run("PostSyncDataOverwrite", func(t *testing.T) {
		payloadBytes, _ := json.Marshal(overwritePayload)
		req, _ := http.NewRequest("POST", "/api/pixez/users/987654/sync-data", bytes.NewBuffer(payloadBytes))
		req.Header.Set("Content-Type", "application/json")
		req.SetBasicAuth("admin", "test12345")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}

		// Verify GET returns only tag2 and no illust history (overwritten)
		reqGet, _ := http.NewRequest("GET", "/api/pixez/users/987654/sync-data", nil)
		reqGet.SetBasicAuth("admin", "test12345")
		wGet := httptest.NewRecorder()
		router.ServeHTTP(wGet, reqGet)

		var res struct {
			Success bool                  `json:"success"`
			Message string                `json:"message"`
			Data    model.UserDataPayload `json:"data"`
		}
		if err := json.Unmarshal(wGet.Body.Bytes(), &res); err != nil {
			t.Fatalf("failed to unmarshal: %v", err)
		}
		if !res.Success {
			t.Errorf("expected success true, got false")
		}
		data := res.Data
		if data.BanTags == nil || len(*data.BanTags) != 1 || (*data.BanTags)[0].Name != "tag2" || data.IllustHistories == nil || len(*data.IllustHistories) != 0 {
			t.Errorf("overwrite failed, got sync data: %+v", data)
		}
	})

	// 8. Test DELETE user (cascade delete synced data)
	t.Run("DeleteUser", func(t *testing.T) {
		req, _ := http.NewRequest("DELETE", "/api/pixez/users/987654", nil)
		req.SetBasicAuth("admin", "test12345")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}

		// Verify deleted user
		var check model.PixivUser
		err := gormDB.First(&check, "pixiv_user_id = ?", "987654").Error
		if err != gorm.ErrRecordNotFound {
			t.Errorf("expected record not found, got err: %v", err)
		}

		// Verify deleted synced tags (cascade)
		var count int64
		gormDB.Model(&model.BanTag{}).Where("pixiv_user_id = ?", "987654").Count(&count)
		if count != 0 {
			t.Errorf("expected ban tags count 0 (cascade delete failed), got %d", count)
		}
	})
}
