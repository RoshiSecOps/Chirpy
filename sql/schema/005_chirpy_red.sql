-- +goose Up
ALTER TABLE users ADD COLUMN is_chirpy_red BOOLEAN NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE users drop COLUMN is_chirpy_red;