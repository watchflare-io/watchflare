-- +goose Up
-- The legacy webhook_endpoints table is replaced by notification_channels.
-- Existing rows are intentionally lost: the URL format has changed and the
-- user is expected to re-add channels through the new Settings > Notifications
-- page. This is announced in the CHANGELOG.
DROP TABLE IF EXISTS webhook_endpoints;

-- +goose Down
CREATE TABLE IF NOT EXISTS webhook_endpoints (
	id         CHAR(36)    NOT NULL PRIMARY KEY,
	url        TEXT        NOT NULL,
	enabled    BOOLEAN     NOT NULL DEFAULT TRUE,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
