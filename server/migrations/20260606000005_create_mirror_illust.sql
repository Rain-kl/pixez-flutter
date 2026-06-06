-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS mirror_illust (
    illust_id INTEGER PRIMARY KEY,
    detail_json TEXT NOT NULL,
    image_files_json TEXT NOT NULL,
    created_at DATETIME,
    updated_at DATETIME
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS mirror_illust;
-- +goose StatementEnd
