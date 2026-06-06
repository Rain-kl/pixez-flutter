package service

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"pixez-sync/db"
	"pixez-sync/model"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type mockPixivTransport struct {
	roundTripFunc func(req *http.Request) (*http.Response, error)
}

func (m *mockPixivTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return m.roundTripFunc(req)
}

func setupMirrorWorkerTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	gormDB, err := gorm.Open(sqlite.Open(""), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	if err := gormDB.AutoMigrate(&model.PixivUser{}, &model.MirrorTask{}, &model.MirrorIllust{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}

	originalDB := db.DB
	db.DB = gormDB
	t.Cleanup(func() {
		db.DB = originalDB
	})
	return gormDB
}

func TestMirrorWorkerProcessesIllustTask(t *testing.T) {
	setupMirrorWorkerTestDB(t)

	tmpDir, err := os.MkdirTemp("", "pixez-mirror-worker-test-*")
	if err != nil {
		t.Fatalf("failed to create temp directory: %v", err)
	}
	t.Cleanup(func() {
		os.RemoveAll(tmpDir)
	})

	db.DB.Create(&model.PixivUser{
		PixivUserID:  "12345",
		Name:         "TestUser",
		Account:      "test_acc",
		AccessToken:  "valid_access_token",
		RefreshToken: "valid_refresh_token",
		UpdatedAt:    time.Now(),
	})
	db.DB.Create(&model.MirrorTask{
		ID:                 "task-1",
		TaskType:           model.MirrorTaskTypeIllust,
		TargetType:         model.MirrorTargetIllust,
		TargetID:           99999,
		Status:             model.MirrorTaskStatusQueued,
		RequestPayloadJSON: `{"illust_id":99999}`,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	})

	originalTransport := pixivClient.Transport
	pixivClient.Transport = &mockPixivTransport{
		roundTripFunc: func(req *http.Request) (*http.Response, error) {
			if req.URL.Host == "app-api.pixiv.net" && req.URL.Path == "/v1/illust/detail" {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body: io.NopCloser(bytes.NewBufferString(`{
						"illust": {
							"id": 99999,
							"image_urls": {
								"square_medium": "https://i.pximg.net/99999_p0_square1200.jpg",
								"medium": "https://i.pximg.net/99999_p0_master1200.jpg",
								"large": "https://i.pximg.net/99999_p0_master1200.jpg"
							},
							"meta_single_page": {
								"original_image_url": "https://i.pximg.net/99999_p0.jpg"
							},
							"meta_pages": []
						}
					}`)),
					Header: make(http.Header),
				}, nil
			}
			if req.URL.Host == "i.pximg.net" {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(bytes.NewBufferString("fake-image-bytes")),
					Header:     make(http.Header),
				}, nil
			}
			return &http.Response{
				StatusCode: http.StatusNotFound,
				Body:       io.NopCloser(bytes.NewBufferString("not found")),
				Header:     make(http.Header),
			}, nil
		},
	}
	t.Cleanup(func() {
		pixivClient.Transport = originalTransport
	})

	worker := NewMirrorWorker(tmpDir, 2)
	if processed := worker.ProcessOne(); !processed {
		t.Fatalf("expected worker to process one queued task")
	}

	var task model.MirrorTask
	if err := db.DB.First(&task, "id = ?", "task-1").Error; err != nil {
		t.Fatalf("failed to query task: %v", err)
	}
	if task.Status != model.MirrorTaskStatusSuccess {
		t.Fatalf("expected task success, got %s: %s", task.Status, task.ErrorMessage)
	}
	if task.TotalCount != 1 || task.SuccessCount != 1 || task.FailedCount != 0 {
		t.Fatalf("unexpected task counts: total=%d success=%d failed=%d", task.TotalCount, task.SuccessCount, task.FailedCount)
	}
	if task.RequestURLsJSON == "" {
		t.Fatalf("expected request URL list to be stored")
	}
	if strings.Contains(task.RequestURLsJSON, "master1200") || strings.Contains(task.RequestURLsJSON, "square1200") {
		t.Fatalf("expected only original URL to be requested, got %s", task.RequestURLsJSON)
	}

	var illust model.MirrorIllust
	if err := db.DB.First(&illust, "illust_id = ?", 99999).Error; err != nil {
		t.Fatalf("expected mirror_illust record, got error: %v", err)
	}
	if illust.DetailJSON == "" || illust.ImageFilesJSON == "" {
		t.Fatalf("expected detail and image files JSON to be stored")
	}
	if _, err := os.Stat(tmpDir + "/99999/99999_p0.jpg"); err != nil {
		t.Fatalf("expected original image file to exist: %v", err)
	}
}

func TestMirrorWorkerStoresFailedImageURLs(t *testing.T) {
	setupMirrorWorkerTestDB(t)

	tmpDir, err := os.MkdirTemp("", "pixez-mirror-worker-fail-test-*")
	if err != nil {
		t.Fatalf("failed to create temp directory: %v", err)
	}
	t.Cleanup(func() {
		os.RemoveAll(tmpDir)
	})

	db.DB.Create(&model.PixivUser{
		PixivUserID:  "12345",
		Name:         "TestUser",
		Account:      "test_acc",
		AccessToken:  "valid_access_token",
		RefreshToken: "valid_refresh_token",
		UpdatedAt:    time.Now(),
	})
	db.DB.Create(&model.MirrorTask{
		ID:         "task-2",
		TaskType:   model.MirrorTaskTypeIllust,
		TargetType: model.MirrorTargetIllust,
		TargetID:   10000,
		Status:     model.MirrorTaskStatusQueued,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	})

	originalTransport := pixivClient.Transport
	pixivClient.Transport = &mockPixivTransport{
		roundTripFunc: func(req *http.Request) (*http.Response, error) {
			if req.URL.Host == "app-api.pixiv.net" {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body: io.NopCloser(bytes.NewBufferString(`{
						"illust": {
							"id": 10000,
							"image_urls": {"medium": "https://i.pximg.net/10000_p0_master1200.jpg"},
							"meta_single_page": {"original_image_url": "https://i.pximg.net/10000_p0.jpg"},
							"meta_pages": []
						}
					}`)),
					Header: make(http.Header),
				}, nil
			}
			return &http.Response{
				StatusCode: http.StatusForbidden,
				Body:       io.NopCloser(bytes.NewBufferString("blocked")),
				Header:     make(http.Header),
			}, nil
		},
	}
	t.Cleanup(func() {
		pixivClient.Transport = originalTransport
	})

	worker := NewMirrorWorker(tmpDir, 1)
	worker.ProcessOne()

	var task model.MirrorTask
	if err := db.DB.First(&task, "id = ?", "task-2").Error; err != nil {
		t.Fatalf("failed to query task: %v", err)
	}
	if task.Status != model.MirrorTaskStatusFailed {
		t.Fatalf("expected failed task, got %s", task.Status)
	}
	if task.RetryURLsJSON == "" || task.ErrorMessage == "" {
		t.Fatalf("expected retry URLs and error message, got retry=%q error=%q", task.RetryURLsJSON, task.ErrorMessage)
	}
	if strings.Contains(task.RequestURLsJSON, "master1200") || strings.Contains(task.RetryURLsJSON, "master1200") {
		t.Fatalf("expected failed task URL JSON to contain only original URLs, request=%s retry=%s", task.RequestURLsJSON, task.RetryURLsJSON)
	}
	if task.TotalCount != 1 || task.SuccessCount != 0 || task.FailedCount != 1 {
		t.Fatalf("unexpected failed task counts: total=%d success=%d failed=%d", task.TotalCount, task.SuccessCount, task.FailedCount)
	}

	var count int64
	db.DB.Model(&model.MirrorIllust{}).Where("illust_id = ?", 10000).Count(&count)
	if count != 1 {
		t.Fatalf("expected failed task to keep mirror_illust metadata record, got %d", count)
	}
}

func TestMirrorWorkerMarksPartialDownloadsAsSuccess(t *testing.T) {
	setupMirrorWorkerTestDB(t)

	tmpDir, err := os.MkdirTemp("", "pixez-mirror-worker-partial-test-*")
	if err != nil {
		t.Fatalf("failed to create temp directory: %v", err)
	}
	t.Cleanup(func() {
		os.RemoveAll(tmpDir)
	})

	db.DB.Create(&model.PixivUser{
		PixivUserID:  "12345",
		Name:         "TestUser",
		Account:      "test_acc",
		AccessToken:  "valid_access_token",
		RefreshToken: "valid_refresh_token",
		UpdatedAt:    time.Now(),
	})
	db.DB.Create(&model.MirrorTask{
		ID:         "task-3",
		TaskType:   model.MirrorTaskTypeIllust,
		TargetType: model.MirrorTargetIllust,
		TargetID:   20000,
		Status:     model.MirrorTaskStatusQueued,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	})

	originalTransport := pixivClient.Transport
	pixivClient.Transport = &mockPixivTransport{
		roundTripFunc: func(req *http.Request) (*http.Response, error) {
			if req.URL.Host == "app-api.pixiv.net" {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body: io.NopCloser(bytes.NewBufferString(`{
						"illust": {
							"id": 20000,
							"image_urls": {
								"medium": "https://i.pximg.net/20000_p0_master1200.jpg",
								"large": "https://i.pximg.net/20000_p0_large.jpg"
							},
							"meta_single_page": {"original_image_url": "https://i.pximg.net/20000_p0.jpg"},
							"meta_pages": []
						}
					}`)),
					Header: make(http.Header),
				}, nil
			}
			if req.URL.Path == "/20000_p0.jpg" {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(bytes.NewBufferString("fake-image-bytes")),
					Header:     make(http.Header),
				}, nil
			}
			return &http.Response{
				StatusCode: http.StatusBadGateway,
				Body:       io.NopCloser(bytes.NewBufferString("bad gateway")),
				Header:     make(http.Header),
			}, nil
		},
	}
	t.Cleanup(func() {
		pixivClient.Transport = originalTransport
	})

	worker := NewMirrorWorker(tmpDir, 2)
	worker.ProcessOne()

	var task model.MirrorTask
	if err := db.DB.First(&task, "id = ?", "task-3").Error; err != nil {
		t.Fatalf("failed to query task: %v", err)
	}
	if task.Status != model.MirrorTaskStatusSuccess {
		t.Fatalf("expected partial task success, got %s: %s", task.Status, task.ErrorMessage)
	}
	if task.TotalCount != 1 || task.SuccessCount != 1 || task.FailedCount != 0 {
		t.Fatalf("unexpected partial counts: total=%d success=%d failed=%d", task.TotalCount, task.SuccessCount, task.FailedCount)
	}
	if strings.Contains(task.RequestURLsJSON, "master1200") || strings.Contains(task.RequestURLsJSON, "_large") {
		t.Fatalf("expected partial test to request only original URL, got %s", task.RequestURLsJSON)
	}

	var count int64
	db.DB.Model(&model.MirrorIllust{}).Where("illust_id = ?", 20000).Count(&count)
	if count != 1 {
		t.Fatalf("expected partial success to write mirror_illust, got %d", count)
	}
}
