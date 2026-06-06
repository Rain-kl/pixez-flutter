-- +goose Up
-- +goose StatementBegin
ALTER TABLE mirror_tasks ADD COLUMN request_urls_json TEXT;
ALTER TABLE mirror_tasks ADD COLUMN total_count INTEGER NOT NULL DEFAULT 0;
ALTER TABLE mirror_tasks ADD COLUMN success_count INTEGER NOT NULL DEFAULT 0;
ALTER TABLE mirror_tasks ADD COLUMN failed_count INTEGER NOT NULL DEFAULT 0;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- SQLite cannot drop columns before 3.35 without rebuilding the table.
-- Existing rows keep the extra tracking columns on rollback.
-- +goose StatementEnd
