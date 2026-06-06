-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS pixiv_users (
    pixiv_user_id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    account TEXT NOT NULL,
    mail_address TEXT,
    user_image TEXT,
    access_token TEXT NOT NULL,
    refresh_token TEXT NOT NULL,
    device_token TEXT,
    is_premium INTEGER DEFAULT 0,
    x_restrict INTEGER DEFAULT 0,
    is_mail_authorized INTEGER DEFAULT 0,
    created_at DATETIME,
    updated_at DATETIME
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS pixiv_users;
-- +goose StatementEnd
