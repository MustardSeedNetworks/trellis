# Trellis — Data Model & Project Format

## Project bundle (open, documented — unlike AirMagnet's opaque `.svp`)
A directory (zipped for transport) — `*.trellis`:
```
myproject.trellis/
  manifest.json            # schema version, ids, created/modified, app version
  project.sqlite           # relational: entities, placements, settings
  assets/                  # floorplan images / PDFs / (later) CAD
    floor-0.png
  surveys/                 # high-volume measurement clouds, columnar
    survey-2026-06-20.parquet
  reports/                 # generated outputs (optional)
```
**Why split SQLite + Parquet:** structured/relational state (a few thousand rows) lives
in SQLite; measurement point-clouds (100k–millions of rows) live in Parquet/Arrow.
Never stuff measurements into SQLite.

## SQLite schema (sketch — `sqlc` generates Go types)
```sql
building(id, name, created_at)
floor(id, building_id, idx, name, asset_path, elevation_m, height_m,
      interfloor_atten_db, scale_m_per_px, grid_res_m)
material(id, name, atten_db_2_4, atten_db_5, atten_db_6, thickness_m)
wall(id, floor_id, ax, ay, bx, by, material_id)
antenna(id, name, gain_dbi, pattern_blob)         -- pattern tables as blob/JSON
ap(id, floor_id, x, y, z, name)
radio(id, ap_id, band, channel, width_mhz, tx_power_dbm, phy,
      antenna_id, azimuth_deg, downtilt_deg)
requirement(id, floor_id, metric, threshold, zone_geo)   -- e.g. RxP >= -67
survey(id, building_id, started_at, radio_source, parquet_path)
project_setting(key, value)                       -- env params, defaults, units
```
Migrations are versioned; `manifest.json.schema_version` gates compatibility.

## Measurement records (Parquet/Arrow columnar)
One row per (point × heard-BSSID):
```
ts, floor_id, x, y, z, band, channel, bssid, ssid, rssi_dbm, snr_db, phy, source
```
Columnar = fast scans/filters for interpolation + analysis; compresses well; streams.

## In-memory / engine representation
The Go core projects SQLite+Parquet into the protobuf `Scene` (engine.proto) for
compute. Grids never persist in the project by default (they're derived + cheap to
recompute); optionally cache to `reports/` for report generation.

## Identity & units
- All geometry **floor-local metres**; scale calibration converts asset pixels→metres.
- IDs are ULIDs/UUIDs (stable across save/load, mergeable later for collaboration).
- Bands/PHY/metrics use the protobuf enums (single source of truth).

## Versioning & migration
- `manifest.json`: `{ "schema_version": N, "app_version": "x.y.z", ... }`.
- Open older N → run migrations to current; refuse newer N with a clear message.
- The **contracts** (.proto) are versioned independently and govern wire compat.
