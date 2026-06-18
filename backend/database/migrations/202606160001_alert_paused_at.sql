-- +goose Up
-- Migration 011: alert_incidents.paused_at
-- Adds a paused state so an open incident can be suspended while its host is paused.

ALTER TABLE alert_incidents ADD COLUMN paused_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_alert_incidents_active
    ON alert_incidents(host_id, metric_type)
    WHERE resolved_at IS NULL AND paused_at IS NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_alert_incidents_active;
ALTER TABLE alert_incidents DROP COLUMN paused_at;
