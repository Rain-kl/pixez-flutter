-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS bookmark_novels (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    pixiv_user_id TEXT NOT NULL,
    restrict TEXT NOT NULL,
    novel_id INTEGER NOT NULL,
    title TEXT,
    caption TEXT,
    user_id INTEGER,
    user_name TEXT,
    text_length INTEGER,
    x_restrict INTEGER,
    total_view INTEGER,
    total_bookmarks INTEGER,
    is_original INTEGER NOT NULL DEFAULT 0,
    visible INTEGER NOT NULL DEFAULT 0,
    is_muted INTEGER NOT NULL DEFAULT 0,
    novel_ai_type INTEGER,
    series_id INTEGER,
    series_title TEXT,
    cover_url TEXT,
    novel_json TEXT NOT NULL,
    last_export_run_id TEXT NOT NULL,
    last_seen_at DATETIME NOT NULL,
    mirror_status INTEGER NOT NULL DEFAULT 0,
    mirror_retry_count INTEGER NOT NULL DEFAULT 0,
    removed INTEGER NOT NULL DEFAULT 0,
    removed_at DATETIME,
    created_at DATETIME,
    updated_at DATETIME,
    UNIQUE(pixiv_user_id, restrict, novel_id)
);
CREATE INDEX IF NOT EXISTS idx_bookmark_novels_user ON bookmark_novels(pixiv_user_id);
CREATE INDEX IF NOT EXISTS idx_bookmark_novels_removed ON bookmark_novels(removed);
CREATE INDEX IF NOT EXISTS idx_bookmark_novels_last_run ON bookmark_novels(last_export_run_id);
CREATE INDEX IF NOT EXISTS idx_bookmark_novels_mirror_status ON bookmark_novels(mirror_status);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS bookmark_novels;
-- +goose StatementEnd
