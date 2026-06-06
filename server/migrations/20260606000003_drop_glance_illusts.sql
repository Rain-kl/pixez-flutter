-- +goose Up
-- +goose StatementBegin
DROP TABLE IF EXISTS glance_illusts;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- No rollback for dropping deprecated glance_illusts table
-- +goose StatementEnd
