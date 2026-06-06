-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS mirror_tasks (
    id TEXT PRIMARY KEY,
    task_type TEXT NOT NULL,
    target_type TEXT NOT NULL,
    target_id INTEGER NOT NULL,
    status TEXT NOT NULL,
    request_payload_json TEXT,
    retry_urls_json TEXT,
    error_message TEXT,
    attempt_count INTEGER NOT NULL DEFAULT 0,
    locked_at DATETIME,
    started_at DATETIME,
    finished_at DATETIME,
    created_at DATETIME,
    updated_at DATETIME,
    UNIQUE(task_type, target_type, target_id)
);
CREATE INDEX IF NOT EXISTS idx_mirror_tasks_status ON mirror_tasks(status);
CREATE INDEX IF NOT EXISTS idx_mirror_tasks_target ON mirror_tasks(task_type, target_type, target_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS mirror_tasks;
-- +goose StatementEnd
