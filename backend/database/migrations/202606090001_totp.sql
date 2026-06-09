-- +goose Up
ALTER TABLE users ADD COLUMN IF NOT EXISTS totp_secret TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS totp_enabled BOOLEAN NOT NULL DEFAULT FALSE;

CREATE TABLE IF NOT EXISTS totp_backup_codes (
	id          CHAR(36)    NOT NULL PRIMARY KEY,
	user_id     CHAR(36)    NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	code_hash   VARCHAR(64) NOT NULL,
	used_at     TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_totp_backup_codes_user_id ON totp_backup_codes(user_id);

-- +goose Down
DROP TABLE IF EXISTS totp_backup_codes;
ALTER TABLE users DROP COLUMN IF EXISTS totp_enabled;
ALTER TABLE users DROP COLUMN IF EXISTS totp_secret;
