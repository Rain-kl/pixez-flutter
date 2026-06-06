-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS bookmark_export_runs (
    id TEXT PRIMARY KEY,
    pixiv_user_id TEXT NOT NULL,
    restrict TEXT NOT NULL,
    status TEXT NOT NULL,
    total_count INTEGER NOT NULL DEFAULT 0,
    new_count INTEGER NOT NULL DEFAULT 0,
    updated_count INTEGER NOT NULL DEFAULT 0,
    removed_count INTEGER NOT NULL DEFAULT 0,
    error_message TEXT,
    started_at DATETIME NOT NULL,
    finished_at DATETIME,
    next_url TEXT,
    last_request_url TEXT,
    created_at DATETIME,
    updated_at DATETIME
);
CREATE INDEX IF NOT EXISTS idx_bookmark_export_runs_user ON bookmark_export_runs(pixiv_user_id);
CREATE INDEX IF NOT EXISTS idx_bookmark_export_runs_status ON bookmark_export_runs(status);

CREATE TABLE IF NOT EXISTS bookmark_illusts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    pixiv_user_id TEXT NOT NULL,
    restrict TEXT NOT NULL,
    illust_id INTEGER NOT NULL,
    title TEXT,
    type TEXT,
    user_id INTEGER,
    user_name TEXT,
    page_count INTEGER,
    width INTEGER,
    height INTEGER,
    sanity_level INTEGER,
    x_restrict INTEGER,
    total_view INTEGER,
    total_bookmarks INTEGER,
    visible INTEGER NOT NULL DEFAULT 0,
    is_muted INTEGER NOT NULL DEFAULT 0,
    illust_ai_type INTEGER,
    illust_json TEXT NOT NULL,
    last_export_run_id TEXT NOT NULL,
    last_seen_at DATETIME NOT NULL,
    removed INTEGER NOT NULL DEFAULT 0,
    removed_at DATETIME,
    created_at DATETIME,
    updated_at DATETIME,
    UNIQUE(pixiv_user_id, restrict, illust_id)
);
CREATE INDEX IF NOT EXISTS idx_bookmark_illusts_user ON bookmark_illusts(pixiv_user_id);
CREATE INDEX IF NOT EXISTS idx_bookmark_illusts_removed ON bookmark_illusts(removed);
CREATE INDEX IF NOT EXISTS idx_bookmark_illusts_last_run ON bookmark_illusts(last_export_run_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS bookmark_illusts;
DROP TABLE IF EXISTS bookmark_export_runs;
-- +goose StatementEnd
