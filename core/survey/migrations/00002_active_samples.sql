-- 00002_active_samples.sql — persist active-survey measurements.
--
-- 00001 modelled the passive walk only: `samples` holds every BSS seen from a
-- point. An active walk records something different — the association the radio
-- actually held — and it had nowhere to go, so it was parsed and then dropped
-- on the way to disk. The 245-point "Time Square 9th Floor" capture recovered
-- 245 associations and persisted none of them, and the import test still passed
-- because it only counted points.
--
-- One row per point rather than columns on survey_points: an active point has
-- exactly one association and a passive point has none, so a nullable column
-- group would encode "which kind is this" twice, once in sample_kind and once
-- in whether the columns are set.

-- +goose Up
CREATE TABLE active_samples (
	point_id       INTEGER PRIMARY KEY REFERENCES survey_points(id) ON DELETE CASCADE,
	survey_id      TEXT NOT NULL REFERENCES surveys(id) ON DELETE CASCADE,
	ssid           TEXT NOT NULL DEFAULT '',
	bssid          TEXT NOT NULL DEFAULT '',
	rssi_dbm       INTEGER NOT NULL,
	data_rate_mbps REAL NOT NULL DEFAULT 0,
	roaming_event  INTEGER NOT NULL DEFAULT 0 CHECK (roaming_event IN (0,1)),
	previous_bssid TEXT NOT NULL DEFAULT '',
	roam_count     INTEGER NOT NULL DEFAULT 0,
	-- Same bound as samples.signal_dbm: a receiver cannot report a positive
	-- dBm, and below -110 is not a reading.
	CHECK (rssi_dbm <= 0 AND rssi_dbm >= -110)
) STRICT;

CREATE INDEX idx_active_samples_survey ON active_samples(survey_id);

-- +goose Down
DROP TABLE active_samples;
