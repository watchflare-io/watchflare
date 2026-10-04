-- +goose Up
CREATE TABLE container_image_updates (
	host_id        CHAR(36)    NOT NULL,
	container_id   TEXT        NOT NULL,
	container_name TEXT        NOT NULL DEFAULT '',
	image          TEXT        NOT NULL DEFAULT '',
	local_digest   TEXT        NOT NULL DEFAULT '',
	remote_digest  TEXT        NOT NULL DEFAULT '',
	status         TEXT        NOT NULL DEFAULT '',
	error          TEXT        NOT NULL DEFAULT '',
	local_version  TEXT        NOT NULL DEFAULT '',
	remote_version TEXT        NOT NULL DEFAULT '',
	commits_behind INTEGER     NOT NULL DEFAULT 0,
	checked_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
	PRIMARY KEY (host_id, container_id),
	CONSTRAINT fk_container_image_updates_host FOREIGN KEY (host_id) REFERENCES hosts(id) ON DELETE CASCADE
);

-- +goose Down
DROP TABLE container_image_updates;
