// SPDX-License-Identifier: BUSL-1.1

package survey

// store.go persists surveys in SQLite.
//
// It replaces one indented-JSON file per survey. That shape read and rewrote a
// whole survey for every change, and a survey is not small: the twelve real
// AirMapper captures in the reference corpus carry 1,045 walk points and
// 113,551 BSS observations, the largest single one 29,278 observations.
//
// The JSON *format* stays — it is how surveys are imported and exported. What
// is gone is JSON as the database. There is no reader for the old files: this
// is pre-alpha, and a compatibility path for throwaway data is exactly the kind
// of legacy burden the project refuses.

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/MustardSeedNetworks/trellis/core/wifi"
)

const rfc3339Nano = time.RFC3339Nano

// persistSurvey writes a survey and all of its children.
//
// THE CALLER MUST HOLD m.mu. Every mutating method on Manager takes the lock
// and then calls this, so it must not take it again — sync.RWMutex is not
// reentrant and doing so deadlocks on the first save. The previous pair of
// saveSurvey / saveSurveyUnlocked implied one of them locked; neither did, and
// the naming was a trap. One function, one rule, stated here.
//
// The write is a delete-and-reinsert of the survey's children inside one
// transaction. A survey is edited as a whole document by every caller above
// this layer, so a diffing writer would be a lot of machinery to reproduce what
// the caller already handed us — and a partial write is the one outcome that
// must not be reachable, which the transaction guarantees.
func (m *Manager) persistSurvey(survey *Survey) error {
	ctx := context.Background()
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin survey write: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := upsertSurvey(ctx, tx, survey); err != nil {
		return err
	}
	// ON DELETE CASCADE clears floors, points, samples and criteria.
	if _, err := tx.ExecContext(ctx, `DELETE FROM floors WHERE survey_id = ?`, survey.ID); err != nil {
		return fmt.Errorf("clear floors: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		`DELETE FROM pass_fail_criteria WHERE survey_id = ?`, survey.ID); err != nil {
		return fmt.Errorf("clear criteria: %w", err)
	}
	for _, floor := range survey.Floors {
		if err := insertFloor(ctx, tx, survey.ID, floor); err != nil {
			return err
		}
	}
	for _, c := range survey.PassFailCriteria {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO pass_fail_criteria (survey_id, option, name, limit_val, suffix,
				enabled, mode, ap_count, imported)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			survey.ID, c.Option, c.Name, c.Limit, c.Suffix, boolToInt(c.Enabled), c.Mode,
			c.APCount, boolToInt(c.Imported),
		); err != nil {
			return fmt.Errorf("insert criteria: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit survey write: %w", err)
	}
	return nil
}

func upsertSurvey(ctx context.Context, tx *sql.Tx, s *Survey) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO surveys (id, name, description, survey_type, status, active_floor_id,
			interface, iperf_server, test_duration, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name, description = excluded.description,
			survey_type = excluded.survey_type, status = excluded.status,
			active_floor_id = excluded.active_floor_id, interface = excluded.interface,
			iperf_server = excluded.iperf_server, test_duration = excluded.test_duration,
			updated_at = excluded.updated_at`,
		s.ID, s.Name, s.Description, string(s.SurveyType), string(s.Status),
		nullIfEmpty(s.ActiveFloorID), s.Interface, s.IperfServer, s.TestDuration,
		s.CreatedAt.UTC().Format(rfc3339Nano), s.UpdatedAt.UTC().Format(rfc3339Nano),
	)
	if err != nil {
		return fmt.Errorf("upsert survey: %w", err)
	}
	return nil
}

func insertFloor(ctx context.Context, tx *sql.Tx, surveyID string, f *Floor) error {
	var w, h any
	var scale, img any
	if f.FloorPlan != nil {
		w, h, scale, img = f.FloorPlan.Width, f.FloorPlan.Height, f.FloorPlan.ScaleM, f.FloorPlan.ImageData
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO floors (id, survey_id, name, level, created_at, updated_at,
			fp_width, fp_height, fp_scale_m, fp_image)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		f.ID, surveyID, f.Name, f.Level,
		f.CreatedAt.UTC().Format(rfc3339Nano), f.UpdatedAt.UTC().Format(rfc3339Nano),
		w, h, scale, img,
	); err != nil {
		return fmt.Errorf("insert floor %s: %w", f.ID, err)
	}
	for _, p := range f.Samples {
		if err := insertPoint(ctx, tx, surveyID, f.ID, p); err != nil {
			return err
		}
	}
	return nil
}

func insertPoint(ctx context.Context, tx *sql.Tx, surveyID, floorID string, p *SamplePoint) error {
	kind, passive := classifySample(p.SampleData)
	var ssids, bssids, ap24, ap5, ap6, coCh, adjCh any
	if passive != nil {
		ssids, bssids = passive.UniqueSSIDs, passive.UniqueBSSIDs
		ap24, ap5, ap6 = passive.APCount2_4, passive.APCount5, passive.APCount6
		coCh, adjCh = passive.CoChannelAPs, passive.AdjChannelAPs
	}
	res, err := tx.ExecContext(ctx, `
		INSERT INTO survey_points (floor_id, survey_id, x, y, recorded_at, sample_kind,
			unique_ssids, unique_bssids, ap_count_24, ap_count_5, ap_count_6,
			co_channel_aps, adj_channel_aps)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		floorID, surveyID, p.X, p.Y, p.Timestamp.UTC().Format(rfc3339Nano), kind,
		ssids, bssids, ap24, ap5, ap6, coCh, adjCh,
	)
	if err != nil {
		return fmt.Errorf("insert point: %w", err)
	}
	pointID, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("point id: %w", err)
	}
	if passive == nil {
		return nil
	}
	for _, n := range passive.Networks {
		if n == nil {
			continue
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO samples (survey_id, point_id, ssid, bssid, signal_dbm, channel,
				frequency_mhz, security, channel_width, noise_dbm, snr, ht_mode, is_dfs, last_seen)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			surveyID, pointID, n.SSID, n.BSSID, n.Signal, n.Channel, n.Frequency,
			n.Security, n.ChannelWidth, n.NoiseFloor, n.SNR, n.HTMode, boolToInt(n.IsDFS),
			n.LastSeen.UTC().Format(rfc3339Nano),
		); err != nil {
			return fmt.Errorf("insert sample: %w", err)
		}
	}
	return nil
}

// classifySample names the sample union arm and returns the passive payload
// when that is what it is. SampleData is `any` on the wire because a point
// carries one of three shapes; the schema records which.
func classifySample(data any) (string, *PassiveSample) {
	switch v := data.(type) {
	case *PassiveSample:
		return "passive", v
	case PassiveSample:
		return "passive", &v
	case *ActiveSample, ActiveSample:
		return "active", nil
	case *ThroughputSample, ThroughputSample:
		return "throughput", nil
	}
	// JSON-decoded samples arrive as map[string]any. Re-decode rather than
	// guess: a point whose kind we cannot name would violate the CHECK, and a
	// silent "passive" default would put active data in a passive column.
	if m, ok := data.(map[string]any); ok {
		if _, hasNetworks := m["networks"]; hasNetworks {
			var ps PassiveSample
			if raw, err := json.Marshal(m); err == nil {
				if json.Unmarshal(raw, &ps) == nil {
					return "passive", &ps
				}
			}
		}
		if _, hasThroughput := m["throughputMbps"]; hasThroughput {
			return "throughput", nil
		}
		return "active", nil
	}
	return "passive", nil
}

// LoadSurveys reads every survey into the in-memory map.
func (m *Manager) LoadSurveys() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	ctx := context.Background()
	rows, err := m.db.QueryContext(ctx, `
		SELECT id, name, description, survey_type, status, COALESCE(active_floor_id,''),
			interface, iperf_server, test_duration, created_at, updated_at
		FROM surveys ORDER BY created_at`)
	if err != nil {
		return fmt.Errorf("list surveys: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var loaded []*Survey
	for rows.Next() {
		var s Survey
		var st, status, created, updated string
		if err := rows.Scan(&s.ID, &s.Name, &s.Description, &st, &status, &s.ActiveFloorID,
			&s.Interface, &s.IperfServer, &s.TestDuration, &created, &updated); err != nil {
			return fmt.Errorf("scan survey: %w", err)
		}
		s.SurveyType, s.Status = Type(st), Status(status)
		s.CreatedAt, _ = time.Parse(rfc3339Nano, created)
		s.UpdatedAt, _ = time.Parse(rfc3339Nano, updated)
		loaded = append(loaded, &s)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate surveys: %w", err)
	}

	for _, s := range loaded {
		if err := m.loadFloors(ctx, s); err != nil {
			return err
		}
		if err := m.loadCriteria(ctx, s); err != nil {
			return err
		}
		m.surveys[s.ID] = s
	}
	return nil
}

func (m *Manager) loadFloors(ctx context.Context, s *Survey) error {
	rows, err := m.db.QueryContext(ctx, `
		SELECT id, name, level, created_at, updated_at, fp_width, fp_height, fp_scale_m, fp_image
		FROM floors WHERE survey_id = ? ORDER BY level`, s.ID)
	if err != nil {
		return fmt.Errorf("list floors: %w", err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var f Floor
		var created, updated string
		var w, h sql.NullInt64
		var scale sql.NullFloat64
		var img sql.NullString
		if err := rows.Scan(&f.ID, &f.Name, &f.Level, &created, &updated,
			&w, &h, &scale, &img); err != nil {
			return fmt.Errorf("scan floor: %w", err)
		}
		f.CreatedAt, _ = time.Parse(rfc3339Nano, created)
		f.UpdatedAt, _ = time.Parse(rfc3339Nano, updated)
		if w.Valid && h.Valid {
			f.FloorPlan = &FloorPlan{
				Width: int(w.Int64), Height: int(h.Int64),
				ScaleM: scale.Float64, ImageData: img.String,
			}
		}
		s.Floors = append(s.Floors, &f)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate floors: %w", err)
	}
	for _, f := range s.Floors {
		if err := m.loadPoints(ctx, f); err != nil {
			return err
		}
	}
	return nil
}

func (m *Manager) loadPoints(ctx context.Context, f *Floor) error {
	rows, err := m.db.QueryContext(ctx, `
		SELECT id, x, y, recorded_at, sample_kind, unique_ssids, unique_bssids,
			ap_count_24, ap_count_5, ap_count_6, co_channel_aps, adj_channel_aps
		FROM survey_points WHERE floor_id = ? ORDER BY id`, f.ID)
	if err != nil {
		return fmt.Errorf("list points: %w", err)
	}
	defer func() { _ = rows.Close() }()

	type pending struct {
		point *SamplePoint
		id    int64
		kind  string
		ps    *PassiveSample
	}
	var todo []pending
	for rows.Next() {
		var id int64
		var p SamplePoint
		var recorded, kind string
		var ssids, bssids, ap24, ap5, ap6, coCh, adjCh sql.NullInt64
		if err := rows.Scan(&id, &p.X, &p.Y, &recorded, &kind,
			&ssids, &bssids, &ap24, &ap5, &ap6, &coCh, &adjCh); err != nil {
			return fmt.Errorf("scan point: %w", err)
		}
		p.Timestamp, _ = time.Parse(rfc3339Nano, recorded)
		var ps *PassiveSample
		if kind == "passive" {
			ps = &PassiveSample{
				UniqueSSIDs: int(ssids.Int64), UniqueBSSIDs: int(bssids.Int64),
				APCount2_4: int(ap24.Int64), APCount5: int(ap5.Int64),
				APCount6: int(ap6.Int64), CoChannelAPs: int(coCh.Int64),
				AdjChannelAPs: int(adjCh.Int64),
			}
			p.SampleData = ps
		}
		todo = append(todo, pending{point: &p, id: id, kind: kind, ps: ps})
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate points: %w", err)
	}
	for _, t := range todo {
		if t.ps != nil {
			nets, err := m.loadSamples(ctx, t.id)
			if err != nil {
				return err
			}
			t.ps.Networks = nets
		}
		f.Samples = append(f.Samples, t.point)
	}
	return nil
}

func (m *Manager) loadSamples(ctx context.Context, pointID int64) ([]*wifi.ScannedNetwork, error) {
	rows, err := m.db.QueryContext(ctx, `
		SELECT ssid, bssid, signal_dbm, channel, frequency_mhz, security, channel_width,
			noise_dbm, snr, ht_mode, is_dfs, last_seen
		FROM samples WHERE point_id = ? ORDER BY id`, pointID)
	if err != nil {
		return nil, fmt.Errorf("list samples: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []*wifi.ScannedNetwork
	for rows.Next() {
		var n wifi.ScannedNetwork
		var dfs int
		var lastSeen string
		if err := rows.Scan(&n.SSID, &n.BSSID, &n.Signal, &n.Channel, &n.Frequency,
			&n.Security, &n.ChannelWidth, &n.NoiseFloor, &n.SNR, &n.HTMode,
			&dfs, &lastSeen); err != nil {
			return nil, fmt.Errorf("scan sample: %w", err)
		}
		n.IsDFS = dfs == 1
		n.LastSeen, _ = time.Parse(rfc3339Nano, lastSeen)
		out = append(out, &n)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate samples: %w", err)
	}
	return out, nil
}

func (m *Manager) loadCriteria(ctx context.Context, s *Survey) error {
	rows, err := m.db.QueryContext(ctx, `
		SELECT option, name, limit_val, suffix, enabled, mode, ap_count, imported
		FROM pass_fail_criteria WHERE survey_id = ? ORDER BY id`, s.ID)
	if err != nil {
		return fmt.Errorf("list criteria: %w", err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var c PassFailCriterion
		var enabled, imported int
		if err := rows.Scan(&c.Option, &c.Name, &c.Limit, &c.Suffix, &enabled, &c.Mode,
			&c.APCount, &imported); err != nil {
			return fmt.Errorf("scan criteria: %w", err)
		}
		c.Enabled, c.Imported = enabled == 1, imported == 1
		s.PassFailCriteria = append(s.PassFailCriteria, c)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate criteria: %w", err)
	}
	return nil
}

// deleteSurveyRow removes a survey; children go with it via ON DELETE CASCADE.
func (m *Manager) deleteSurveyRow(id string) error {
	res, err := m.db.ExecContext(context.Background(), `DELETE FROM surveys WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete survey: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrSurveyNotFound
	}
	return nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// appendPoint writes a single new point and its observations, and touches the
// survey's updated_at.
//
// It exists because persistSurvey rewrites the whole survey, and a walk adds
// points one at a time: rewriting every prior point on each addition makes an
// import quadratic. Measured on the largest reference survey (142 points,
// 29,278 observations), the rewrite path took 43.8s; appending takes a fraction
// of that, because each call writes one point instead of all of them.
//
// THE CALLER MUST HOLD m.mu, on the same terms as persistSurvey.
func (m *Manager) appendPoint(surveyID, floorID string, s *Survey, p *SamplePoint) error {
	ctx := context.Background()
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin append: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := insertPoint(ctx, tx, surveyID, floorID, p); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE surveys SET updated_at = ? WHERE id = ?`,
		s.UpdatedAt.UTC().Format(rfc3339Nano), surveyID); err != nil {
		return fmt.Errorf("touch survey: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE floors SET updated_at = ? WHERE id = ?`,
		s.UpdatedAt.UTC().Format(rfc3339Nano), floorID); err != nil {
		return fmt.Errorf("touch floor: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit append: %w", err)
	}
	return nil
}
