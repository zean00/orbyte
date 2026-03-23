CREATE OR REPLACE VIEW analytics_snapshot_reads_v AS
SELECT snapshot_id, generated_at, window_key, payload_json
FROM analytics_snapshots;

CREATE OR REPLACE VIEW analytics_rollup_reads_v AS
SELECT rollup_id, granularity, bucket_start, bucket_end, snapshot_count, payload_json
FROM analytics_rollups;

CREATE MATERIALIZED VIEW IF NOT EXISTS analytics_snapshot_reads_mv AS
SELECT snapshot_id, generated_at, window_key, payload_json
FROM analytics_snapshots;

CREATE MATERIALIZED VIEW IF NOT EXISTS analytics_rollup_reads_mv AS
SELECT rollup_id, granularity, bucket_start, bucket_end, snapshot_count, payload_json
FROM analytics_rollups;

CREATE INDEX IF NOT EXISTS idx_analytics_snapshot_reads_mv_window_generated
ON analytics_snapshot_reads_mv (window_key, generated_at);

CREATE INDEX IF NOT EXISTS idx_analytics_rollup_reads_mv_granularity_start
ON analytics_rollup_reads_mv (granularity, bucket_start);

REFRESH MATERIALIZED VIEW analytics_snapshot_reads_mv;
REFRESH MATERIALIZED VIEW analytics_rollup_reads_mv;
