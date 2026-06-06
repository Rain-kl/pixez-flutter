package config

import (
	"bufio"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	AuthUser                  string
	AuthPass                  string
	DBPath                    string
	ListenAddr                string
	MirrorDir                 string
	MirrorDownloadConcurrency int
	BookmarkExportInterval    time.Duration
}

// loadEnv reads environment variables from a file if it exists
func loadEnv(filename string) {
	file, err := os.Open(filename)
	if err != nil {
		return // Ignore if file doesn't exist
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		// Remove quotes if present
		if (strings.HasPrefix(val, "\"") && strings.HasSuffix(val, "\"")) ||
			(strings.HasPrefix(val, "'") && strings.HasSuffix(val, "'")) {
			val = val[1 : len(val)-1]
		}
		if os.Getenv(key) == "" {
			os.Setenv(key, val)
		}
	}
}

func init() {
	// Try loading .env first
	loadEnv(".env")

	// Set default slog level from environment variable
	var level slog.Level
	switch strings.ToLower(os.Getenv("LOG_LEVEL")) {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level: level,
	}
	handler := slog.NewTextHandler(os.Stdout, opts)
	slog.SetDefault(slog.New(handler))
}

func LoadConfig() (*Config, error) {

	authUser := os.Getenv("PIXEZ_AUTH_USER")
	authPass := os.Getenv("PIXEZ_AUTH_PASS")

	if authUser == "" || authPass == "" {
		return nil, fmt.Errorf("required environment variables PIXEZ_AUTH_USER and PIXEZ_AUTH_PASS must be set")
	}

	dbPath := os.Getenv("PIXEZ_DB_PATH")
	if dbPath == "" {
		dbPath = "./pixez-sync.db"
	}

	listenAddr := os.Getenv("PIXEZ_LISTEN_ADDR")
	if listenAddr == "" {
		listenAddr = ":8080"
	}

	mirrorDir := os.Getenv("PIXEZ_MIRROR_DIR")
	if mirrorDir == "" {
		mirrorDir = "./data/mirror"
	}

	mirrorDownloadConcurrency := 5
	if raw := os.Getenv("PIXEZ_MIRROR_DOWNLOAD_CONCURRENCY"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			mirrorDownloadConcurrency = parsed
		}
	}

	bookmarkExportInterval := 24 * time.Hour
	if raw := os.Getenv("PIXEZ_BOOKMARK_EXPORT_INTERVAL_HOURS"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			bookmarkExportInterval = time.Duration(parsed) * time.Hour
		}
	}

	return &Config{
		AuthUser:                  authUser,
		AuthPass:                  authPass,
		DBPath:                    dbPath,
		ListenAddr:                listenAddr,
		MirrorDir:                 mirrorDir,
		MirrorDownloadConcurrency: mirrorDownloadConcurrency,
		BookmarkExportInterval:    bookmarkExportInterval,
	}, nil
}
