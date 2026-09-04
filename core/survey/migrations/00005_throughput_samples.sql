-- 00005_throughput_samples.sql — persist active throughput measurements.
--
-- The same hole 00002 closed for associations. survey_points records
-- sample_kind = 'throughput', and the payload had nowhere to go: classifySample
-- named the kind and returned no rows to write, and the load path had no branch
-- for it. A real three-point active survey on this Mac reloaded as three points
-- holding nothing — the rates, and the AP the link ran over, were gone. The
-- kind alone survived, which is what made it look like it had been stored.
--
-- One row per point, like active_samples, for the same reason: a point has
-- exactly one measurement or none, and nullable columns on survey_points would
-- encode the kind twice.
--
-- Latency, jitter and loss are here because ThroughputSample carries them, not
-- because anything measures them yet: a TCP test does not, and a UDP test is a
-- different measurement. They stay at zero until one exists.

-- +goose Up
CREATE TABLE throughput_samples (
	point_id        INTEGER PRIMARY KEY REFERENCES survey_points(id) ON DELETE CASCADE,
	survey_id       TEXT NOT NULL REFERENCES surveys(id) ON DELETE CASCADE,
	ssid            TEXT NOT NULL DEFAULT '',
	bssid           TEXT NOT NULL DEFAULT '',
	-- Zero where the host reported no association: a rate measured with no AP
	-- named is still a rate. The other tables bound this to a real reading
	-- because they cannot exist without one.
	rssi_dbm        INTEGER NOT NULL DEFAULT 0,
	download_mbps   REAL NOT NULL DEFAULT 0,
	upload_mbps     REAL NOT NULL DEFAULT 0,
	latency_ms      REAL NOT NULL DEFAULT 0,
	jitter_ms       REAL NOT NULL DEFAULT 0,
	packet_loss_pct REAL NOT NULL DEFAULT 0,
	CHECK (rssi_dbm <= 0 AND rssi_dbm >= -110),
	CHECK (download_mbps >= 0 AND upload_mbps >= 0)
) STRICT;

CREATE INDEX idx_throughput_samples_survey ON throughput_samples(survey_id);

-- +goose Down
DROP TABLE throughput_samples;
