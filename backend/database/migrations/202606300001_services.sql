-- +goose Up
CREATE TABLE IF NOT EXISTS services (
	id            BIGSERIAL PRIMARY KEY,
	host_id       CHAR(36) NOT NULL REFERENCES hosts(id) ON DELETE CASCADE,
	name          TEXT NOT NULL,
	description   TEXT NOT NULL DEFAULT '',
	enabled_state TEXT NOT NULL DEFAULT '',
	active_state  TEXT NOT NULL DEFAULT '',
	sub_state     TEXT NOT NULL DEFAULT '',
	collected_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	CONSTRAINT services_host_name_unique UNIQUE (host_id, name)
);

CREATE INDEX IF NOT EXISTS idx_services_host ON services(host_id);

-- +goose Down
DROP TABLE IF EXISTS services;
