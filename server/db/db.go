package db

import (
	"fmt"
	"log"
	"log/slog"
	"os"
	"time"

	"pixez-sync/migrations"

	"github.com/pressly/goose/v3"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

func InitDB(dbPath string) (*gorm.DB, error) {
	// Initialize GORM with SQLite
	gormLogger := logger.New(log.New(os.Stdout, "\r\n", log.LstdFlags), logger.Config{
		SlowThreshold:             200 * time.Millisecond,
		LogLevel:                  logger.Warn,
		IgnoreRecordNotFoundError: true,
		Colorful:                  false,
	})
	gormDB, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: gormLogger,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Get underlying sql.DB to run goose migrations
	sqlDB, err := gormDB.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get sql.DB: %w", err)
	}

	// Set dialect for goose
	if err := goose.SetDialect("sqlite3"); err != nil {
		return nil, fmt.Errorf("goose failed to set dialect: %w", err)
	}

	// Run goose migrations
	slog.Info("Running database migrations...")
	goose.SetBaseFS(migrations.EmbedMigrations)
	if err := goose.Up(sqlDB, "."); err != nil {
		return nil, fmt.Errorf("goose migration failed: %w", err)
	}
	slog.Info("Database migrations applied successfully")

	DB = gormDB
	return gormDB, nil
}
