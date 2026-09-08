-- 00006_attempted_points.sql — a measurement that failed is still a fact.
--
-- Until now a failed measurement was discarded (core/survey/throughput.go), so
-- a coverage map could not tell a place nobody walked from a place where the
-- test failed. Both drew as a gap. ADR-0009 decides the other way: store the
-- attempt, store no number for it, and keep it out of every layer.
--
-- A column rather than a new sample_kind: widening the sample_kind CHECK means
-- rebuilding survey_points, and its children (samples, active_samples,
-- throughput_samples) reference it ON DELETE CASCADE — dropping the old table
-- mid-rebuild takes every stored reading with it. The kind stays what was
-- attempted, which is also what the load path already tolerates having no
-- payload row for.

-- +goose Up
ALTER TABLE survey_points ADD COLUMN failure TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE survey_points DROP COLUMN failure;
