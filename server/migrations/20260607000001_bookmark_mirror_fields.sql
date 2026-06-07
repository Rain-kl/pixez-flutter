-- +goose Up
-- Add mirror tracking fields to bookmark_illusts
ALTER TABLE bookmark_illusts ADD COLUMN mirror_status INTEGER NOT NULL DEFAULT 0;
ALTER TABLE bookmark_illusts ADD COLUMN mirror_retry_count INTEGER NOT NULL DEFAULT 0;

-- Add index for scheduler to quickly find unmixed / failed bookmarks
CREATE INDEX idx_bookmark_illust_mirror_status ON bookmark_illusts(mirror_status);

-- Replace composite unique index with target_id-only unique index
-- so that mirror_tasks enforces one active task per illust
DROP INDEX IF EXISTS idx_mirror_task_target;
CREATE UNIQUE INDEX idx_mirror_task_target_id ON mirror_tasks(target_type, target_id);

-- +goose Down
DROP INDEX IF EXISTS idx_mirror_task_target_id;
CREATE UNIQUE INDEX idx_mirror_task_target ON mirror_tasks(task_type, target_type, target_id);
DROP INDEX IF EXISTS idx_bookmark_illust_mirror_status;
ALTER TABLE bookmark_illusts DROP COLUMN mirror_retry_count;
ALTER TABLE bookmark_illusts DROP COLUMN mirror_status;
