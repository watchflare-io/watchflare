-- +goose Up
CREATE TABLE IF NOT EXISTS notification_channels (
	id            CHAR(36)     NOT NULL PRIMARY KEY,
	name          VARCHAR(100) NOT NULL,
	url_encrypted TEXT         NOT NULL,
	categories    TEXT[]       NOT NULL DEFAULT ARRAY['alerts']::TEXT[],
	enabled       BOOLEAN      NOT NULL DEFAULT TRUE,
	created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
	updated_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_notification_channels_enabled ON notification_channels(enabled) WHERE enabled = TRUE;

-- +goose Down
DROP TABLE IF EXISTS notification_channels;
