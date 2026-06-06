package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"pixez-sync/db"
	"pixez-sync/middleware"
	"pixez-sync/model"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupMirrorHandlerTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	api := r.Group("/api/pixez", middleware.BasicAuth("admin", "test12345"))
	{
		api.POST("/illusts/:illust_id/mirror", MirrorIllust)
		api.GET("/illusts/:illust_id/mirror", CheckIllustMirror)
	}
	mirror := r.Group("/mirror", middleware.BasicAuth("admin", "test12345"))
	{
		mirror.GET("/v1/illust/detail", GetMirroredIllustDetail)
		mirror.GET("/pximg/*path", ServeMirroredImage)
	}
	return r
}

func setupMirrorHandlerTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	gormDB, err := gorm.Open(sqlite.Open(""), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	if err := gormDB.AutoMigrate(&model.MirrorTask{}, &model.MirrorIllust{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}

	originalDB := db.DB
	db.DB = gormDB
	t.Cleanup(func() {
		db.DB = originalDB
	})
	return gormDB
}

func TestMirrorBusinessEndpointsUseApiNamespace(t *testing.T) {
	setupMirrorHandlerTestDB(t)
	router := setupMirrorHandlerTestRouter()

	req, _ := http.NewRequest("POST", "/api/pixez/illusts/99999/mirror", nil)
	req.SetBasicAuth("admin", "test12345")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d, body: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Success bool `json:"success"`
		Data    struct {
			TaskID   string `json:"task_id"`
			IllustID int64  `json:"illust_id"`
			Status   string `json:"status"`
			Mirrored bool   `json:"mirrored"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if !resp.Success || resp.Data.TaskID == "" || resp.Data.IllustID != 99999 {
		t.Fatalf("unexpected enqueue response: %+v", resp)
	}
	if resp.Data.Status != model.MirrorTaskStatusQueued {
		t.Fatalf("expected queued task, got %s", resp.Data.Status)
	}
	if resp.Data.Mirrored {
		t.Fatalf("expected mirrored false before worker success")
	}

	var tasks []model.MirrorTask
	if err := db.DB.Find(&tasks).Error; err != nil {
		t.Fatalf("failed to query mirror tasks: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected one mirror task, got %d", len(tasks))
	}

	req, _ = http.NewRequest("POST", "/mirror/v1/illust/detail?illust_id=99999", nil)
	req.SetBasicAuth("admin", "test12345")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected old /mirror POST to be unregistered, got %d", w.Code)
	}
}

func TestCheckIllustMirrorReadsTaskSuccessCount(t *testing.T) {
	setupMirrorHandlerTestDB(t)
	router := setupMirrorHandlerTestRouter()

	db.DB.Create(&model.MirrorTask{
		ID:          "task-1",
		TaskType:    model.MirrorTaskTypeIllust,
		TargetType:  model.MirrorTargetIllust,
		TargetID:    99999,
		Status:      model.MirrorTaskStatusProcessing,
		TotalCount:  3,
		FailedCount: 3,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	})

	req, _ := http.NewRequest("GET", "/api/pixez/illusts/99999/mirror", nil)
	req.SetBasicAuth("admin", "test12345")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var resp struct {
		Success bool `json:"success"`
		Data    struct {
			TaskID   string `json:"task_id"`
			Status   string `json:"status"`
			Mirrored bool   `json:"mirrored"`
			Total    int    `json:"total_count"`
			Success  int    `json:"success_count"`
			Failed   int    `json:"failed_count"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Data.Mirrored {
		t.Fatalf("expected mirrored false without successful image count")
	}
	if resp.Data.TaskID != "task-1" || resp.Data.Status != model.MirrorTaskStatusProcessing {
		t.Fatalf("unexpected task status response: %+v", resp.Data)
	}

	db.DB.Create(&model.MirrorIllust{
		IllustID:       99999,
		DetailJSON:     `{"illust":{"id":99999}}`,
		ImageFilesJSON: `[]`,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	})

	req, _ = http.NewRequest("GET", "/api/pixez/illusts/99999/mirror", nil)
	req.SetBasicAuth("admin", "test12345")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Data.Mirrored {
		t.Fatalf("expected mirror_illust metadata alone not to mark mirrored")
	}

	db.DB.Model(&model.MirrorTask{}).Where("id = ?", "task-1").Updates(map[string]any{
		"status":        model.MirrorTaskStatusSuccess,
		"success_count": 1,
		"failed_count":  2,
	})

	req, _ = http.NewRequest("GET", "/api/pixez/illusts/99999/mirror", nil)
	req.SetBasicAuth("admin", "test12345")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if !resp.Data.Mirrored {
		t.Fatalf("expected success_count > 0 to mark mirrored")
	}
}

func TestGetMirroredIllustDetailUsesDatabaseAndRewritesURLs(t *testing.T) {
	setupMirrorHandlerTestDB(t)
	router := setupMirrorHandlerTestRouter()

	db.DB.Create(&model.MirrorIllust{
		IllustID: 99999,
		DetailJSON: `{
			"illust": {
				"id": 99999,
				"image_urls": {
					"medium": "https://i.pximg.net/img-master/img/2026/01/01/00/00/00/99999_p0_master1200.jpg",
					"large": "https:\/\/i.pximg.net\/img-master\/img\/2026\/01\/01\/00\/00\/00\/99999_p0_master1200.jpg"
				}
			}
		}`,
		ImageFilesJSON: `[]`,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	})

	req, _ := http.NewRequest("GET", "/mirror/v1/illust/detail?illust_id=99999", nil)
	req.SetBasicAuth("admin", "test12345")
	req.Host = "my-sync-server.local:8080"
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d, body: %s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	if strings.Contains(body, `"success"`) {
		t.Fatalf("mirror endpoint must not use system response wrapper: %s", body)
	}
	if !strings.Contains(body, "http://my-sync-server.local:8080/mirror/pximg") {
		t.Fatalf("expected unescaped pximg URL rewrite, got: %s", body)
	}
	if !strings.Contains(body, "http:\\/\\/my-sync-server.local:8080\\/mirror\\/pximg") {
		t.Fatalf("expected escaped pximg URL rewrite, got: %s", body)
	}
}

func TestServeMirroredImage(t *testing.T) {
	setupMirrorHandlerTestDB(t)
	router := setupMirrorHandlerTestRouter()

	tmpDir, err := os.MkdirTemp("", "pixez-mirror-handler-test-*")
	if err != nil {
		t.Fatalf("failed to create temp directory: %v", err)
	}
	t.Cleanup(func() {
		os.RemoveAll(tmpDir)
	})

	originalMirrorDir := MirrorDir
	MirrorDir = tmpDir
	t.Cleanup(func() {
		MirrorDir = originalMirrorDir
	})

	illustDir := filepath.Join(tmpDir, "99999")
	if err := os.MkdirAll(illustDir, 0755); err != nil {
		t.Fatalf("failed to create illust directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(illustDir, "99999_p0.jpg"), []byte("fake-image-bytes"), 0644); err != nil {
		t.Fatalf("failed to write image: %v", err)
	}
	if err := os.WriteFile(filepath.Join(illustDir, "99999_p1.png"), []byte("fake-png-bytes"), 0644); err != nil {
		t.Fatalf("failed to write png image: %v", err)
	}

	req, _ := http.NewRequest("GET", "/mirror/pximg/img-original/img/2026/01/01/00/00/00/99999_p0.jpg", nil)
	req.SetBasicAuth("admin", "test12345")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d, body: %s", w.Code, w.Body.String())
	}
	if w.Body.String() != "fake-image-bytes" {
		t.Fatalf("expected fake-image-bytes, got %s", w.Body.String())
	}

	req, _ = http.NewRequest("GET", "/mirror/pximg/img-master/img/2026/01/01/00/00/00/99999_p0_master1200.jpg", nil)
	req.SetBasicAuth("admin", "test12345")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected master request to map to original file, got %d, body: %s", w.Code, w.Body.String())
	}
	if w.Body.String() != "fake-image-bytes" {
		t.Fatalf("expected original file bytes for master request, got %s", w.Body.String())
	}

	req, _ = http.NewRequest("GET", "/mirror/pximg/c/360x360_70/img-master/img/2026/01/01/00/00/00/99999_p0_square1200.jpg", nil)
	req.SetBasicAuth("admin", "test12345")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected square request to map to original file, got %d, body: %s", w.Code, w.Body.String())
	}
	if w.Body.String() != "fake-image-bytes" {
		t.Fatalf("expected original file bytes for square request, got %s", w.Body.String())
	}

	req, _ = http.NewRequest("GET", "/mirror/pximg/img-master/img/2026/01/01/00/00/00/99999_p1_master1200.jpg", nil)
	req.SetBasicAuth("admin", "test12345")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected jpg master request to map to png original file, got %d, body: %s", w.Code, w.Body.String())
	}
	if w.Body.String() != "fake-png-bytes" {
		t.Fatalf("expected png original bytes for jpg master request, got %s", w.Body.String())
	}
}
