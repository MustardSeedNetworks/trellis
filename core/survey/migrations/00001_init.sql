-- 00001_init.sql — the survey store.
--
-- Replaces one indented-JSON file per survey. That shape read and rewrote a
-- whole survey for any change, and a survey is not small: the twelve real
-- AirMapper captures in the reference corpus hold 1,045 walk points and
-- 113,551 BSS observations between them, the largest single survey being
-- 29,278 observations.
--
-- Observations are the row that multiplies, so they are their own table rather
-- than a blob hanging off a point. docs/03-TECH-STACK.md reserves Parquet/Arrow
-- for "millions of points"; at 113k that bar is two orders of magnitude away,
-- so this stays relational — but `samples` is keyed (survey_id, point_id) so a
-- single survey can be spilled to columnar later without reshaping anything.

-- +goose Up
CREATE TABLE surveys (
	id              TEXT PRIMARY KEY,
	name            TEXT NOT NULL,
	description     TEXT NOT NULL DEFAULT '',
	survey_type     TEXT NOT NULL,
	status          TEXT NOT NULL,
	active_floor_id TEXT,
	interface       TEXT NOT NULL DEFAULT '',
	iperf_server    TEXT NOT NULL DEFAULT '',
	test_duration   INTEGER NOT NULL DEFAULT 0,
	created_at      TEXT NOT NULL,
	updated_at      TEXT NOT NULL
) STRICT;

CREATE TABLE floors (
	id         TEXT PRIMARY KEY,
	survey_id  TEXT NOT NULL REFERENCES surveys(id) ON DELETE CASCADE,
	name       TEXT NOT NULL,
	level      INTEGER NOT NULL,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	-- Floor plan. Nullable as a group: a floor may exist before its plan is
	-- uploaded. image_data is the data URL the UI renders.
	fp_width   INTEGER,
	fp_height  INTEGER,
	fp_scale_m REAL,
	fp_image   TEXT
) STRICT;

-- One walk position. AirMapper calls it a survey point; x/y are pixels on the
-- floor plan, which is why they are bounded by the plan's dimensions rather
-- than by anything geographic.
CREATE TABLE survey_points (
	id          INTEGER PRIMARY KEY AUTOINCREMENT,
	floor_id    TEXT NOT NULL REFERENCES floors(id) ON DELETE CASCADE,
	survey_id   TEXT NOT NULL REFERENCES surveys(id) ON DELETE CASCADE,
	x           INTEGER NOT NULL,
	y           INTEGER NOT NULL,
	recorded_at TEXT NOT NULL,
	sample_kind TEXT NOT NULL CHECK (sample_kind IN ('passive','active','throughput')),
	-- Per-point aggregates the heatmap reads directly, mirroring
	-- PassiveSample's computed fields. Null where the sample kind does not
	-- produce them.
	unique_ssids    INTEGER,
	unique_bssids   INTEGER,
	ap_count_24     INTEGER,
	ap_count_5      INTEGER,
	ap_count_6      INTEGER,
	co_channel_aps  INTEGER,
	adj_channel_aps INTEGER
) STRICT;

-- One observed BSS at one point. This is the table that grows.
CREATE TABLE samples (
	id            INTEGER PRIMARY KEY AUTOINCREMENT,
	survey_id     TEXT NOT NULL REFERENCES surveys(id) ON DELETE CASCADE,
	point_id      INTEGER NOT NULL REFERENCES survey_points(id) ON DELETE CASCADE,
	ssid          TEXT NOT NULL DEFAULT '',
	bssid         TEXT NOT NULL DEFAULT '',
	signal_dbm    INTEGER NOT NULL,
	channel       INTEGER NOT NULL DEFAULT 0,
	frequency_mhz INTEGER NOT NULL DEFAULT 0,
	security      TEXT NOT NULL DEFAULT '',
	channel_width INTEGER NOT NULL DEFAULT 0,
	noise_dbm     INTEGER NOT NULL DEFAULT 0,
	snr           INTEGER NOT NULL DEFAULT 0,
	ht_mode       TEXT NOT NULL DEFAULT '',
	is_dfs        INTEGER NOT NULL DEFAULT 0 CHECK (is_dfs IN (0,1)),
	last_seen     TEXT NOT NULL,
	-- A radio cannot report a positive dBm, and anything below -110 is noise
	-- floor territory rather than a reading. The reference corpus spans
	-- -84..-25, so this rejects decode errors without rejecting real data.
	CHECK (signal_dbm <= 0 AND signal_dbm >= -110)
) STRICT;

CREATE TABLE pass_fail_criteria (
	id        INTEGER PRIMARY KEY AUTOINCREMENT,
	survey_id TEXT NOT NULL REFERENCES surveys(id) ON DELETE CASCADE,
	option    TEXT NOT NULL,
	name      TEXT NOT NULL DEFAULT '',
	limit_val REAL NOT NULL,
	suffix    TEXT NOT NULL DEFAULT '',
	enabled   INTEGER NOT NULL DEFAULT 1 CHECK (enabled IN (0,1)),
	mode      TEXT NOT NULL DEFAULT '',
	ap_count  INTEGER NOT NULL DEFAULT 0,
	imported  INTEGER NOT NULL DEFAULT 0 CHECK (imported IN (0,1))
) STRICT;

CREATE INDEX idx_floors_survey         ON floors(survey_id);
CREATE INDEX idx_points_floor          ON survey_points(floor_id);
CREATE INDEX idx_points_survey         ON survey_points(survey_id);
CREATE INDEX idx_samples_point         ON samples(point_id);
CREATE INDEX idx_samples_survey        ON samples(survey_id);
CREATE INDEX idx_samples_survey_bssid  ON samples(survey_id, bssid);
CREATE INDEX idx_criteria_survey       ON pass_fail_criteria(survey_id);

-- +goose Down
DROP TABLE pass_fail_criteria;
DROP TABLE samples;
DROP TABLE survey_points;
DROP TABLE floors;
DROP TABLE surveys;
