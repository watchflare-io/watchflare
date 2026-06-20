-- +goose NO TRANSACTION
-- Required: TimescaleDB policy functions cannot run inside a transaction

-- +goose Up
-- =====================================================
-- Migration: fix continuous aggregate refresh windows
--
-- The original refresh policies used a start_offset (90 days / 1 year) far
-- larger than the raw metrics retention (24h). Every refresh recomputed
-- buckets whose source rows had already been dropped by retention, which
-- DELETED the previously materialized aggregate rows. Only the last ~day of
-- buckets survived, so the 7d and 30d views showed just a few recent points.
--
-- Fix: bound each start_offset below the retention of its source data so the
-- refresh never reaches into dropped regions, and raise the raw metrics
-- retention to 3 days so the 8h aggregate (materialized ~16h after a bucket
-- closes) always has its source data available when it runs.
-- =====================================================

-- -----------------------------------------------------
-- Raw metrics retention: 24h -> 3 days
-- (8h aggregate needs raw data to survive well past 24h to materialize)
-- -----------------------------------------------------
SELECT remove_retention_policy('metrics', if_exists => TRUE);
SELECT add_retention_policy('metrics', INTERVAL '3 days', if_not_exists => TRUE);

-- -----------------------------------------------------
-- System metrics: re-bound refresh start_offset
-- -----------------------------------------------------
SELECT remove_continuous_aggregate_policy('metrics_10min', if_exists => TRUE);
SELECT add_continuous_aggregate_policy('metrics_10min',
    start_offset      => INTERVAL '3 hours',
    end_offset        => INTERVAL '10 minutes',
    schedule_interval => INTERVAL '10 minutes',
    if_not_exists     => TRUE
);

SELECT remove_continuous_aggregate_policy('metrics_15min', if_exists => TRUE);
SELECT add_continuous_aggregate_policy('metrics_15min',
    start_offset      => INTERVAL '3 hours',
    end_offset        => INTERVAL '15 minutes',
    schedule_interval => INTERVAL '15 minutes',
    if_not_exists     => TRUE
);

SELECT remove_continuous_aggregate_policy('metrics_2h', if_exists => TRUE);
SELECT add_continuous_aggregate_policy('metrics_2h',
    start_offset      => INTERVAL '1 day',
    end_offset        => INTERVAL '2 hours',
    schedule_interval => INTERVAL '2 hours',
    if_not_exists     => TRUE
);

SELECT remove_continuous_aggregate_policy('metrics_8h', if_exists => TRUE);
SELECT add_continuous_aggregate_policy('metrics_8h',
    start_offset      => INTERVAL '2 days',
    end_offset        => INTERVAL '8 hours',
    schedule_interval => INTERVAL '8 hours',
    if_not_exists     => TRUE
);

-- -----------------------------------------------------
-- Container metrics: same flaw, raw retention is already 30 days so only the
-- start_offset needs bounding (well below 30 days).
-- -----------------------------------------------------
SELECT remove_continuous_aggregate_policy('container_metrics_10min', if_exists => TRUE);
SELECT add_continuous_aggregate_policy('container_metrics_10min',
    start_offset      => INTERVAL '3 hours',
    end_offset        => INTERVAL '10 minutes',
    schedule_interval => INTERVAL '10 minutes',
    if_not_exists     => TRUE
);

SELECT remove_continuous_aggregate_policy('container_metrics_15min', if_exists => TRUE);
SELECT add_continuous_aggregate_policy('container_metrics_15min',
    start_offset      => INTERVAL '3 hours',
    end_offset        => INTERVAL '15 minutes',
    schedule_interval => INTERVAL '15 minutes',
    if_not_exists     => TRUE
);

SELECT remove_continuous_aggregate_policy('container_metrics_2h', if_exists => TRUE);
SELECT add_continuous_aggregate_policy('container_metrics_2h',
    start_offset      => INTERVAL '1 day',
    end_offset        => INTERVAL '2 hours',
    schedule_interval => INTERVAL '2 hours',
    if_not_exists     => TRUE
);

SELECT remove_continuous_aggregate_policy('container_metrics_8h', if_exists => TRUE);
SELECT add_continuous_aggregate_policy('container_metrics_8h',
    start_offset      => INTERVAL '2 days',
    end_offset        => INTERVAL '8 hours',
    schedule_interval => INTERVAL '8 hours',
    if_not_exists     => TRUE
);

-- -----------------------------------------------------
-- Rematerialize the currently available range so the views are consistent
-- after the offset change. Data older than the surviving raw rows is gone
-- and cannot be reconstructed.
-- -----------------------------------------------------
CALL refresh_continuous_aggregate('metrics_10min', NULL, NULL);
CALL refresh_continuous_aggregate('metrics_15min', NULL, NULL);
CALL refresh_continuous_aggregate('metrics_2h',    NULL, NULL);
CALL refresh_continuous_aggregate('metrics_8h',    NULL, NULL);
CALL refresh_continuous_aggregate('container_metrics_10min', NULL, NULL);
CALL refresh_continuous_aggregate('container_metrics_15min', NULL, NULL);
CALL refresh_continuous_aggregate('container_metrics_2h',    NULL, NULL);
CALL refresh_continuous_aggregate('container_metrics_8h',    NULL, NULL);

-- +goose Down
-- Restores the original (defective) refresh windows and 24h raw retention.
SELECT remove_retention_policy('metrics', if_exists => TRUE);
SELECT add_retention_policy('metrics', INTERVAL '24 hours', if_not_exists => TRUE);

SELECT remove_continuous_aggregate_policy('metrics_10min', if_exists => TRUE);
SELECT add_continuous_aggregate_policy('metrics_10min',
    start_offset => INTERVAL '14 days', end_offset => INTERVAL '10 minutes',
    schedule_interval => INTERVAL '10 minutes', if_not_exists => TRUE);

SELECT remove_continuous_aggregate_policy('metrics_15min', if_exists => TRUE);
SELECT add_continuous_aggregate_policy('metrics_15min',
    start_offset => INTERVAL '30 days', end_offset => INTERVAL '15 minutes',
    schedule_interval => INTERVAL '15 minutes', if_not_exists => TRUE);

SELECT remove_continuous_aggregate_policy('metrics_2h', if_exists => TRUE);
SELECT add_continuous_aggregate_policy('metrics_2h',
    start_offset => INTERVAL '90 days', end_offset => INTERVAL '2 hours',
    schedule_interval => INTERVAL '2 hours', if_not_exists => TRUE);

SELECT remove_continuous_aggregate_policy('metrics_8h', if_exists => TRUE);
SELECT add_continuous_aggregate_policy('metrics_8h',
    start_offset => INTERVAL '1 year', end_offset => INTERVAL '8 hours',
    schedule_interval => INTERVAL '8 hours', if_not_exists => TRUE);

SELECT remove_continuous_aggregate_policy('container_metrics_10min', if_exists => TRUE);
SELECT add_continuous_aggregate_policy('container_metrics_10min',
    start_offset => INTERVAL '14 days', end_offset => INTERVAL '10 minutes',
    schedule_interval => INTERVAL '10 minutes', if_not_exists => TRUE);

SELECT remove_continuous_aggregate_policy('container_metrics_15min', if_exists => TRUE);
SELECT add_continuous_aggregate_policy('container_metrics_15min',
    start_offset => INTERVAL '30 days', end_offset => INTERVAL '15 minutes',
    schedule_interval => INTERVAL '15 minutes', if_not_exists => TRUE);

SELECT remove_continuous_aggregate_policy('container_metrics_2h', if_exists => TRUE);
SELECT add_continuous_aggregate_policy('container_metrics_2h',
    start_offset => INTERVAL '90 days', end_offset => INTERVAL '2 hours',
    schedule_interval => INTERVAL '2 hours', if_not_exists => TRUE);

SELECT remove_continuous_aggregate_policy('container_metrics_8h', if_exists => TRUE);
SELECT add_continuous_aggregate_policy('container_metrics_8h',
    start_offset => INTERVAL '1 year', end_offset => INTERVAL '8 hours',
    schedule_interval => INTERVAL '8 hours', if_not_exists => TRUE);
