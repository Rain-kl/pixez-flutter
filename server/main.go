package main

import (
	"log/slog"
	"os"

	"pixez-sync/config"
	"pixez-sync/db"
	_ "pixez-sync/docs"
	"pixez-sync/handler"
	"pixez-sync/middleware"
	"pixez-sync/service"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// @title PixEz Sync Server API
// @version 1.0
// @description PixEz Sync 后端同步服务 API。除 /mirror/** Pixiv 形态镜像接口外，系统 API 均返回 {success,message,data} 标准响应。
// @BasePath /
// @securityDefinitions.basic BasicAuth
func main() {
	slog.Info("Starting PixEz Sync Server...")

	// Load Configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		slog.Error("Configuration error", "error", err)
		os.Exit(1)
	}

	// Initialize Database and run migrations
	_, err = db.InitDB(cfg.DBPath)
	if err != nil {
		slog.Error("Database initialization error", "error", err)
		os.Exit(1)
	}

	// Configure handlers
	handler.MirrorDir = cfg.MirrorDir
	mirrorWorker := service.NewMirrorWorker(cfg.MirrorDir, cfg.MirrorDownloadConcurrency)
	mirrorWorker.Start()
	bookmarkExportWorker := service.NewBookmarkExportWorker(cfg.BookmarkExportInterval)
	handler.BookmarkExportWorker = bookmarkExportWorker
	bookmarkExportWorker.Start()

	// Bookmark mirror scheduler — auto-enqueue bookmarks for mirroring
	bookmarkMirrorScheduler := service.NewBookmarkMirrorScheduler(mirrorWorker)
	bookmarkMirrorScheduler.Start()

	// Initialize Gin Router
	r := gin.New()
	r.Use(middleware.DebugLogger())
	r.Use(gin.Logger())
	r.Use(gin.Recovery())
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// API Route Group with Basic Authentication
	api := r.Group("/api/pixez", middleware.BasicAuth(cfg.AuthUser, cfg.AuthPass))
	{
		api.GET("/ping", handler.Ping)
		api.GET("/users", handler.ListUsers)
		api.GET("/users/:pixiv_user_id", handler.GetUser)
		api.PUT("/users/:pixiv_user_id", handler.UpsertUser)
		api.DELETE("/users/:pixiv_user_id", handler.DeleteUser)
		api.GET("/users/:pixiv_user_id/bookmarks/illust/removed", handler.ListRemovedBookmarkIllusts)
		api.GET("/users/:pixiv_user_id/sync-data", handler.GetUserData)
		api.POST("/users/:pixiv_user_id/sync-data", handler.PostUserData)
		api.GET("/users/:pixiv_user_id/sync-data/hashes", handler.GetUserDataHashes)
		api.GET("/scheduled-tasks", handler.ListScheduledTasks)
		api.GET("/scheduled-tasks/bookmark-export", handler.GetBookmarkExportTask)
		api.POST("/scheduled-tasks/bookmark-export/run", handler.RunBookmarkExportTask)
		api.POST("/illusts/:illust_id/mirror", handler.MirrorIllust)
		api.GET("/illusts/:illust_id/mirror", handler.CheckIllustMirror)
		api.POST("/illusts/mirror/batch", handler.BatchCheckIllustMirror)
		api.POST("/novels/:novel_id/mirror", handler.MirrorNovel)
		api.GET("/novels/:novel_id/mirror", handler.CheckNovelMirror)
	}

	// Mirror Route Group with Basic Authentication
	mirror := r.Group("/mirror", middleware.BasicAuth(cfg.AuthUser, cfg.AuthPass))
	{
		mirror.GET("/v1/illust/detail", handler.GetMirroredIllustDetail)
		mirror.GET("/v1/novel/detail", handler.GetMirroredNovelDetail)
		mirror.GET("/webview/v2/novel", handler.GetMirroredNovelText)
		mirror.GET("/pximg/*path", handler.ServeMirroredImage)
	}

	slog.Info("Server listening", "address", cfg.ListenAddr)
	if err := r.Run(cfg.ListenAddr); err != nil {
		slog.Error("Failed to run server", "error", err)
		os.Exit(1)
	}
}
