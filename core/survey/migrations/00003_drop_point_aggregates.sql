-- +goose Up
-- The per-point aggregates duplicated state that is fully derivable from the
-- samples rows, and duplicated state drifts: they were computed from an
-- unsorted Networks slice, so co_channel_aps and adj_channel_aps counted
-- interference against an arbitrary AP rather than the strongest one. The load
-- path reads every point's samples anyway, so recomputing costs nothing and
-- leaves one source of truth.
ALTER TABLE survey_points DROP COLUMN unique_ssids;
ALTER TABLE survey_points DROP COLUMN unique_bssids;
ALTER TABLE survey_points DROP COLUMN ap_count_24;
ALTER TABLE survey_points DROP COLUMN ap_count_5;
ALTER TABLE survey_points DROP COLUMN ap_count_6;
ALTER TABLE survey_points DROP COLUMN co_channel_aps;
ALTER TABLE survey_points DROP COLUMN adj_channel_aps;

-- +goose Down
ALTER TABLE survey_points ADD COLUMN unique_ssids INTEGER;
ALTER TABLE survey_points ADD COLUMN unique_bssids INTEGER;
ALTER TABLE survey_points ADD COLUMN ap_count_24 INTEGER;
ALTER TABLE survey_points ADD COLUMN ap_count_5 INTEGER;
ALTER TABLE survey_points ADD COLUMN ap_count_6 INTEGER;
ALTER TABLE survey_points ADD COLUMN co_channel_aps INTEGER;
ALTER TABLE survey_points ADD COLUMN adj_channel_aps INTEGER;
