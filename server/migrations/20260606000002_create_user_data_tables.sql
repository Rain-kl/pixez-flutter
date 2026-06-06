-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS ban_comments (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    pixiv_user_id TEXT NOT NULL,
    comment_id TEXT NOT NULL,
    name TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_ban_comments_user ON ban_comments(pixiv_user_id);

CREATE TABLE IF NOT EXISTS ban_illusts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    pixiv_user_id TEXT NOT NULL,
    illust_id TEXT NOT NULL,
    name TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_ban_illusts_user ON ban_illusts(pixiv_user_id);

CREATE TABLE IF NOT EXISTS ban_tags (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    pixiv_user_id TEXT NOT NULL,
    name TEXT NOT NULL,
    translate_name TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_ban_tags_user ON ban_tags(pixiv_user_id);

CREATE TABLE IF NOT EXISTS ban_users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    pixiv_user_id TEXT NOT NULL,
    user_id TEXT NOT NULL,
    name TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_ban_users_user ON ban_users(pixiv_user_id);

CREATE TABLE IF NOT EXISTS illust_histories (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    pixiv_user_id TEXT NOT NULL,
    illust_id INTEGER NOT NULL,
    user_id INTEGER NOT NULL,
    picture_url TEXT NOT NULL,
    title TEXT,
    user_name TEXT,
    time INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_illust_histories_user ON illust_histories(pixiv_user_id);

CREATE TABLE IF NOT EXISTS novel_histories (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    pixiv_user_id TEXT NOT NULL,
    novel_id INTEGER NOT NULL,
    user_id INTEGER NOT NULL,
    picture_url TEXT NOT NULL,
    title TEXT NOT NULL,
    user_name TEXT NOT NULL,
    time INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_novel_histories_user ON novel_histories(pixiv_user_id);

CREATE TABLE IF NOT EXISTS tag_histories (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    pixiv_user_id TEXT NOT NULL,
    name TEXT NOT NULL,
    translated_name TEXT NOT NULL,
    type INTEGER
);
CREATE INDEX IF NOT EXISTS idx_tag_histories_user ON tag_histories(pixiv_user_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS ban_comments;
DROP TABLE IF EXISTS ban_illusts;
DROP TABLE IF EXISTS ban_tags;
DROP TABLE IF EXISTS ban_users;
DROP TABLE IF EXISTS illust_histories;
DROP TABLE IF EXISTS novel_histories;
DROP TABLE IF EXISTS tag_histories;
-- +goose StatementEnd
