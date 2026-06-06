-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS scheduled_tasks (
    name TEXT PRIMARY KEY,
    enabled INTEGER NOT NULL DEFAULT 1,
    interval_seconds INTEGER NOT NULL,
    last_run_at DATETIME,
    next_run_at DATETIME,
    last_duration_ms INTEGER NOT NULL DEFAULT 0,
    last_status TEXT,
    last_error TEXT,
    created_at DATETIME,
    updated_at DATETIME
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS scheduled_tasks;
-- +goose StatementEnd
