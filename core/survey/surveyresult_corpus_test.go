package survey_test

// surveyresult_corpus_test.go validates the .SurveyResult reader against real
// AirMapper captures.
//
// The corpus is NOT committed. These are real site surveys of named third
// parties — their BSSIDs, SSIDs and floor plans — and this repository is
// public. Point the test at a directory of .amp files instead:
//
//	TRELLIS_AMP_CORPUS=~/Documents/AirMapper-Surveys go test ./core/survey/
//
// Without it the test skips, so CI stays green without ever seeing the data.
//
// The oracle is the archive's own `.serial` sidecar, which declares
// surveyPointCount. That is what makes this a real check rather than a
// plausibility argument: the reader must recover exactly the number of points
// the file says it contains, for every file, with no fixture to tune against.

import (
	"archive/zip"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MustardSeedNetworks/trellis/core/survey"
)

const corpusEnv = "TRELLIS_AMP_CORPUS"

type ampParts struct {
	declaredPoints int
	surveyResult   []byte
}

// readAMP pulls the declared point count and the measurement member out of an
// AirMapper archive.
func readAMP(t *testing.T, path string) ampParts {
	t.Helper()

	zr, err := zip.OpenReader(filepath.Clean(path))
	if err != nil {
		t.Fatalf("open %s: %v", filepath.Base(path), err)
	}
	defer func() { _ = zr.Close() }()

	var out ampParts
	for _, f := range zr.File {
		name := strings.ToLower(f.Name)
		rc, openErr := f.Open()
		if openErr != nil {
			t.Fatalf("open member %s: %v", f.Name, openErr)
		}
		data, readErr := io.ReadAll(rc)
		_ = rc.Close()
		if readErr != nil {
			t.Fatalf("read member %s: %v", f.Name, readErr)
		}
		switch {
		case strings.HasSuffix(name, ".serial"):
			var meta struct {
				SurveyPointCount int `json:"surveyPointCount"`
			}
			if json.Unmarshal(data, &meta) == nil {
				out.declaredPoints = meta.SurveyPointCount
			}
		case strings.HasSuffix(name, ".surveyresult"):
			out.surveyResult = data
		}
	}
	return out
}

func TestParseSurveyResultAgainstRealCaptures(t *testing.T) {
	dir := os.Getenv(corpusEnv)
	if dir == "" {
		t.Skipf("set %s to a directory of .amp files to run this", corpusEnv)
	}
	if strings.HasPrefix(dir, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			dir = filepath.Join(home, dir[2:])
		}
	}
	files, err := filepath.Glob(filepath.Join(dir, "*.amp"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no .amp files under %s (err=%v)", dir, err)
	}

	totalPoints, totalObs := 0, 0
	for _, path := range files {
		t.Run(filepath.Base(path), func(t *testing.T) {
			parts := readAMP(t, path)
			if parts.surveyResult == nil {
				t.Skip("archive carries no .SurveyResult member")
			}

			points, parseErr := survey.ParseSurveyResult(parts.surveyResult)
			if parseErr != nil {
				t.Fatalf("ParseSurveyResult: %v", parseErr)
			}

			// The file's own declared count is the oracle.
			if parts.declaredPoints > 0 && len(points) != parts.declaredPoints {
				t.Errorf("recovered %d points, the archive declares %d",
					len(points), parts.declaredPoints)
			}

			obs := 0
			for _, p := range points {
				obs += len(p.Networks)
				for _, n := range p.Networks {
					if n.Signal > 0 || n.Signal < -110 {
						t.Fatalf("signal %d dBm is outside a receiver's range", n.Signal)
					}
					if n.Channel < 0 || n.Channel > 233 {
						t.Errorf("channel %d is not an 802.11 channel", n.Channel)
					}
				}
			}
			totalPoints += len(points)
			totalObs += obs
			t.Logf("%d points, %d observations", len(points), obs)
		})
	}
	t.Logf("corpus totals: %d points, %d observations across %d files",
		totalPoints, totalObs, len(files))
}
