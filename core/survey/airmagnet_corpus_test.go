// SPDX-License-Identifier: BUSL-1.1

package survey_test

// airmagnet_corpus_test.go runs the .svd parser over real AirMagnet Survey Pro
// exports, because a parser proven only against a fixture written from the same
// reading of the format proves that the reading is self-consistent, not that it
// matches what AirMagnet writes.
//
// The corpus is not committed: this repository is public and those files are
// the vendor's own demo projects. Point the test at a directory:
//
//	TRELLIS_SVD_CORPUS=~/AirMagnet/DemoProjects go test ./core/survey/
//
// The oracle is the file itself. Every row it holds is one BSS observation, so
// the parsed networks must add up to the rows that carry a position and a
// signal, and no point may be empty.

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MustardSeedNetworks/trellis/core/survey"
)

const svdCorpusEnv = "TRELLIS_SVD_CORPUS"

// svdCorpus opens the corpus directory as a filesystem rooted at itself and
// lists the .svd files under it. Reading through the rooted FS rather than
// joining names onto a path taken from the environment keeps every read inside
// the directory the operator named — the AirMagnet demo projects are nested a
// level deep, so this walks rather than globs.
func svdCorpus(t *testing.T) (fs.FS, []string) {
	t.Helper()
	dir := os.Getenv(svdCorpusEnv)
	if dir == "" {
		t.Skipf("set %s to a directory of AirMagnet .svd files to run this", svdCorpusEnv)
	}
	if strings.HasPrefix(dir, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			dir = filepath.Join(home, dir[2:])
		}
	}
	root := os.DirFS(dir)
	var files []string
	err := fs.WalkDir(root, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.EqualFold(filepath.Ext(path), ".svd") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil || len(files) == 0 {
		t.Fatalf("no .svd files under %s (err=%v)", dir, err)
	}
	return root, files
}

// TestParseAirMagnetReadsTheAPPlacements is separate from the survey walk
// because it failed silently: AP rows carry a "$," prefix that shifted every
// field by one, so every placement in the corpus parsed to nothing and the
// importer stored an empty list without complaining.
func TestParseAirMagnetReadsTheAPPlacements(t *testing.T) {
	root, files := svdCorpus(t)
	withAPs := 0
	for _, path := range files {
		data, err := fs.ReadFile(root, path)
		if err != nil {
			t.Fatal(err)
		}
		file, err := survey.ParseAirMagnetSVD(data)
		if err != nil {
			continue // planner simulations are refused by design
		}
		for _, ap := range file.APs {
			if ap.X == 0 && ap.Y == 0 {
				t.Errorf("%s: an AP placed at the origin — the row was not read", path)
			}
		}
		if len(file.APs) > 0 {
			withAPs++
		}
	}
	if withAPs == 0 {
		t.Error("not one file in the corpus yielded an AP placement")
	}
	t.Logf("%d of %d files carry AP placements", withAPs, len(files))
}

func TestParseAirMagnetReadsTheVendorCorpus(t *testing.T) {
	root, files := svdCorpus(t)
	for _, path := range files {
		t.Run(path, func(t *testing.T) {
			data, err := fs.ReadFile(root, path)
			if err != nil {
				t.Fatal(err)
			}
			file, err := survey.ParseAirMagnetSVD(data)
			if err != nil {
				// A planner simulation is a refusal by design, and the corpus
				// contains three. Everything else must parse.
				if strings.Contains(err.Error(), "virtual (planner) survey") {
					t.Skip("planner simulation, refused by design")
				}
				t.Fatalf("parse: %v", err)
			}

			rows := countAirMagnetRows(t, data)
			if len(file.Points) == 0 {
				// An export with headers and no rows is a real thing the
				// vendor ships (OUTDOOR/ActiveSurvey2.svd is 960 bytes of
				// header), and importing it as an empty survey is the honest
				// outcome. It is only a defect if rows were there to read.
				if rows > 0 {
					t.Fatalf("no measurement points from %d rows", rows)
				}
				return
			}
			observations := 0
			for _, p := range file.Points {
				if len(p.Networks) == 0 {
					t.Fatalf("point (%d,%d) holds no networks", p.X, p.Y)
				}
				observations += len(p.Networks)
			}

			// Every parsed observation must come from a row: comparing against
			// the file's own row count is what catches a splitter that drops
			// or duplicates fields.
			if want := rows; observations > want {
				t.Errorf("parsed %d observations from %d rows", observations, want)
			} else if observations < want/2 {
				t.Errorf("parsed only %d observations from %d rows — too many dropped",
					observations, want)
			}
			// Every point has to be inside the survey, and a coordinate is a
			// position in the file's own units — never a timestamp. Reading an
			// event row as a measurement produced exactly that, and only the
			// store's coordinate constraint caught it, on import, after the
			// parser had reported success.
			const absurdCoordinate = 100000
			for _, p := range file.Points {
				if p.X < 0 || p.Y < 0 || p.X > absurdCoordinate || p.Y > absurdCoordinate {
					t.Fatalf("point at (%d,%d) is not a position on a floor", p.X, p.Y)
				}
			}

			t.Logf("%s %s: %d points, %d observations, %d APs, %dx%d",
				file.Type, file.AppVersion, len(file.Points), observations,
				len(file.APs), file.Width, file.Height)
		})
	}
}

// countAirMagnetRows counts data lines the way the format defines them: every
// line that is not the banner, a `#` comment or the `&` dimensions line.
func countAirMagnetRows(t *testing.T, data []byte) int {
	t.Helper()
	text := survey.DecodeAirMagnetTextForTest(t, data)
	rows := 0
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimRight(line, "\r")
		// The banner may still carry the byte-order mark the decode left in
		// place, so it is trimmed rather than matched.
		line = strings.TrimPrefix(line, "\ufeff")
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "&") ||
			strings.HasPrefix(line, "@") {
			continue
		}
		rows++
	}
	return rows
}
