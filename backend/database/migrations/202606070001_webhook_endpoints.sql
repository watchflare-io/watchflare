-- +goose Up
CREATE TABLE IF NOT EXISTS webhook_endpoints (
	id         CHAR(36)    NOT NULL PRIMARY KEY,
	url        TEXT        NOT NULL,
	enabled    BOOLEAN     NOT NULL DEFAULT TRUE,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- +goose Down
DROP TABLE IF EXISTS webhook_endpoints;
