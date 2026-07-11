-- +goose Up
CREATE TABLE container_states (
	host_id                  CHAR(36)         NOT NULL,
	container_id             TEXT             NOT NULL,
	container_name           TEXT             NOT NULL,
	image                    TEXT             NOT NULL DEFAULT '',
	cpu_percent              DOUBLE PRECISION NOT NULL DEFAULT 0,
	memory_used_bytes        BIGINT           NOT NULL DEFAULT 0,
	memory_limit_bytes       BIGINT           NOT NULL DEFAULT 0,
	network_rx_bytes_per_sec BIGINT           NOT NULL DEFAULT 0,
	network_tx_bytes_per_sec BIGINT           NOT NULL DEFAULT 0,
	runtime                  TEXT             NOT NULL DEFAULT '',
	status                   TEXT             NOT NULL DEFAULT '',
	health                   TEXT             NOT NULL DEFAULT '',
	ports                    TEXT             NOT NULL DEFAULT '',
	updated_at               TIMESTAMPTZ      NOT NULL DEFAULT now(),
	PRIMARY KEY (host_id, container_id),
	CONSTRAINT fk_container_states_host FOREIGN KEY (host_id) REFERENCES hosts(id) ON DELETE CASCADE
);

CREATE INDEX idx_container_states_host ON container_states (host_id);

-- +goose Down
DROP TABLE container_states;
