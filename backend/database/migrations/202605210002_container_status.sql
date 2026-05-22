-- +goose Up
ALTER TABLE container_metrics ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT '';
ALTER TABLE container_metrics ADD COLUMN IF NOT EXISTS health TEXT NOT NULL DEFAULT '';
ALTER TABLE container_metrics ADD COLUMN IF NOT EXISTS ports  TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE container_metrics DROP COLUMN IF EXISTS status;
ALTER TABLE container_metrics DROP COLUMN IF EXISTS health;
ALTER TABLE container_metrics DROP COLUMN IF EXISTS ports;
