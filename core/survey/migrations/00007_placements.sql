-- 00007_placements.sql — AP and client placements, kept on their floor.
--
-- Both importers set placements through UpdateImportedData, whose contract is
-- that they survive a reload, but no table held them: a daemon restart dropped
-- every imported placement (#821). They were also survey-level, so a survey
-- with several floors could not say which plan an AP sits on. Keyed by floor
-- here, and cascaded with it, since a placement is a pixel on one plan.
--
-- tx_power_dbm is nullable on purpose: most sources do not state a power, and a
-- predicted map built on an invented one would carry its error silently.

-- +goose Up
CREATE TABLE ap_placements (
	id           INTEGER PRIMARY KEY AUTOINCREMENT,
	floor_id     TEXT NOT NULL REFERENCES floors(id) ON DELETE CASCADE,
	placement_id TEXT NOT NULL,
	x            INTEGER NOT NULL,
	y            INTEGER NOT NULL,
	label        TEXT NOT NULL DEFAULT '',
	bssid        TEXT NOT NULL DEFAULT '',
	vendor       TEXT NOT NULL DEFAULT '',
	notes        TEXT NOT NULL DEFAULT '',
	imported     INTEGER NOT NULL DEFAULT 0 CHECK (imported IN (0,1)),
	tx_power_dbm REAL,
	band         TEXT NOT NULL DEFAULT ''
) STRICT;

CREATE TABLE client_placements (
	id           INTEGER PRIMARY KEY AUTOINCREMENT,
	floor_id     TEXT NOT NULL REFERENCES floors(id) ON DELETE CASCADE,
	placement_id TEXT NOT NULL,
	x            INTEGER NOT NULL,
	y            INTEGER NOT NULL,
	label        TEXT NOT NULL DEFAULT '',
	mac          TEXT NOT NULL DEFAULT '',
	imported     INTEGER NOT NULL DEFAULT 0 CHECK (imported IN (0,1))
) STRICT;

CREATE INDEX idx_ap_placements_floor     ON ap_placements(floor_id);
CREATE INDEX idx_client_placements_floor ON client_placements(floor_id);

-- +goose Down
DROP TABLE client_placements;
DROP TABLE ap_placements;
