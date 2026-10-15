// SPDX-License-Identifier: BUSL-1.1

// Package linkliveoracle reads the Link-Live oracle directory
// scripts/linklive-fetch-oracle.sh writes: heatmap.json states each survey's
// scale and plan size, and each processed record carries the walk, what was
// heard at every position and where the operator placed the APs, all in the
// floor plan's own pixel space. The data is the vendor's and is never
// committed, so only tests import this package, and only when
// TRELLIS_LL_ORACLE names the directory (docs/12-CROSS-PRODUCT-ORACLE.md).
package linkliveoracle

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Env names the oracle directory.
const Env = "TRELLIS_LL_ORACLE"

// feetToMetres converts the pixels-per-foot scale Link-Live states.
const feetToMetres = 0.3048

// Survey is one fetched survey: its metadata from heatmap.json and its
// processed record.
type Survey struct {
	ID       string  `json:"_id"`
	FileName string  `json:"fileName"`
	ScalePpf float64 `json:"floorPlanScalePpf"`
	WidthPx  int     `json:"floorPlanWidthPx"`
	HeightPx int     `json:"floorPlanHeightPx"`
	Mode     string  `json:"surveyMode"`

	Points      []Point     `json:"-"`
	APLocations []Placement `json:"-"`
}

// MetresPerPixel is the survey's scale, or 0 when Link-Live states none.
func (s Survey) MetresPerPixel() float64 {
	if s.ScalePpf <= 0 {
		return 0
	}
	return feetToMetres / s.ScalePpf
}

// Placement is an AP the operator, or Link-Live's estimator, put on the plan.
type Placement struct {
	X     float64 `json:"x"`
	Y     float64 `json:"y"`
	Label string  `json:"label"`
}

// SurveyorNamed reports whether the surveyor placed this AP. Link-Live labels
// a placement it estimated with the BSS address ("RuckusWi:58b633-c652d9") and
// does so for every BSS it heard, neighbours' routers included, so an address
// label is not ground truth. A placement the surveyor made carries the AP's
// name ("WA-EVERETT-AP7_", or the address the AP reports as its name,
// "10.24.23.87"), and every radio of that AP reports the same value as apName.
func (p Placement) SurveyorNamed() bool {
	if p.Label == "" {
		return false
	}
	_, address, found := strings.Cut(p.Label, ":")
	return !found || !strings.Contains(address, "-")
}

// Point is one walked position.
type Point struct {
	X   float64    `json:"x"`
	Y   float64    `json:"y"`
	APs []Observed `json:"aps"`
}

// Observed is one BSS heard at a position. A record with no signal is a
// position Link-Live holds no reading for, and is not a measurement.
type Observed struct {
	BSSID   string          `json:"bssid"`
	APName  string          `json:"apName"`
	Signal  *int            `json:"signal"`
	Band    string          `json:"band"`
	Channel json.RawMessage `json:"channelPrimary"`
}

// CarrierMHz turns Link-Live's band label and primary channel into a carrier,
// or 0 when they do not name one. The channel is a number, or a string such as
// "97E" for a 6 GHz channel.
func (o Observed) CarrierMHz() int {
	text := strings.TrimSuffix(strings.Trim(string(o.Channel), `"`), "E")
	ch, err := strconv.Atoi(text)
	if err != nil || ch <= 0 {
		return 0
	}
	switch {
	case strings.HasPrefix(o.Band, "2.4"):
		if ch == 14 {
			return 2484
		}
		return 2407 + 5*ch
	case strings.HasPrefix(o.Band, "5"):
		return 5000 + 5*ch
	case strings.HasPrefix(o.Band, "6"):
		return 5950 + 5*ch
	}
	return 0
}

// NormalizeBSSID reduces Link-Live's "Vendor:aabbcc-ddeeff" to bare hex.
func NormalizeBSSID(s string) string {
	if _, address, found := strings.Cut(s, ":"); found && strings.Contains(address, "-") {
		s = address
	}
	return strings.ToLower(strings.NewReplacer(":", "", "-", "").Replace(s))
}

// Dir returns the oracle directory Env names, with a leading "~/" expanded,
// and false when Env is unset.
func Dir() (string, bool) {
	dir := os.Getenv(Env)
	if dir == "" {
		return "", false
	}
	if strings.HasPrefix(dir, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			dir = filepath.Join(home, dir[2:])
		}
	}
	return dir, true
}

// Load reads every survey heatmap.json lists that has been fetched into dir;
// heatmap.json lists every survey in the organisation. It is an error for none
// to have been.
func Load(dir string) ([]Survey, error) {
	// Rooted at the directory the operator named, so a survey id read from
	// heatmap.json cannot reach outside it.
	root := os.DirFS(dir)
	var entries []Survey
	if err := readJSON(root, "heatmap.json", &entries); err != nil {
		return nil, err
	}
	var surveys []Survey
	for _, e := range entries {
		name := e.ID + ".json.gz"
		if _, err := fs.Stat(root, name); err != nil {
			continue
		}
		var record struct {
			Points      []Point     `json:"points"`
			APLocations []Placement `json:"apLocations"`
		}
		if err := readJSON(root, name, &record); err != nil {
			return nil, err
		}
		e.Points, e.APLocations = record.Points, record.APLocations
		surveys = append(surveys, e)
	}
	if len(surveys) == 0 {
		return nil, fmt.Errorf("no processed survey under %s matches heatmap.json", dir)
	}
	return surveys, nil
}

// readJSON decodes a file the fetch script wrote, gzipped or not. Older
// records are a bare array of points rather than an object.
func readJSON(root fs.FS, path string, into any) error {
	body, err := fs.ReadFile(root, path)
	if err != nil {
		return err
	}
	if bytes.HasPrefix(body, []byte{0x1f, 0x8b}) {
		gz, err := gzip.NewReader(bytes.NewReader(body))
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		if body, err = io.ReadAll(gz); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
	}
	if bytes.HasPrefix(bytes.TrimSpace(body), []byte("[")) && path != "heatmap.json" {
		body = append(append([]byte(`{"points":`), body...), '}')
	}
	if err := json.Unmarshal(body, into); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}
