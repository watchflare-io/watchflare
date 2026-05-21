-- +goose Up
ALTER TABLE container_metrics ADD COLUMN IF NOT EXISTS runtime TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE container_metrics DROP COLUMN IF EXISTS runtime;
