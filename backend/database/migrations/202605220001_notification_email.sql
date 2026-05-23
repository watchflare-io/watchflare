-- +goose Up
ALTER TABLE smtp_settings ADD COLUMN IF NOT EXISTS notification_email VARCHAR(255) NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE smtp_settings DROP COLUMN IF EXISTS notification_email;
