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
// surveyPointCount: the reader must recover exactly the number of points the
// file says it contains, with no fixture to tune against.
//
// Not every archive has one. An export from Link-Live carries the measurements
// and the plan and nothing else (#335), and one archive carries a sidecar
// written for a different survey, which the parser now drops. Those archives
// are still checked — the decoded rows have to be internally consistent and to
// yield measurements — they just have no declared count to be checked against.

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MustardSeedNetworks/trellis/core/survey"
)

const corpusEnv = "TRELLIS_AMP_CORPUS"

type ampParts struct {
	// declaredPoints is the sidecar's surveyPointCount, or -1 when the archive
	// has no sidecar the parser accepts and so declares nothing.
	declaredPoints int
	surveyResult   []byte
}

// corpus opens the directory named by the env var as a filesystem rooted at
// itself and lists the files under it whose extension matches ext, walking
// rather than joining a name taken from the environment onto a path.
//
// Confinement here is real, not lexical: dir is opened once with
// os.OpenRoot (Go 1.24+), whose FS refuses to follow a symlink or a ".."
// name out of the root, which is the shape gosec's own G703 autofix
// suggests in place of filepath.Clean (Clean only normalises a path
// lexically; it does not confine one — see pathtraversal.go's sanitizer
// list, which is deliberately just filepath.Base/Rel and path.Base).
// Shared by ampCorpus here and svdCorpus in airmagnet_corpus_test.go.
func corpus(t *testing.T, env, ext string) (fs.FS, []string) {
	t.Helper()
	dir := os.Getenv(env)
	if dir == "" {
		t.Skipf("set %s to a directory of %s files to run this", env, ext)
	}
	if strings.HasPrefix(dir, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			dir = filepath.Join(home, dir[2:])
		}
	}
	r, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatalf("open corpus root %s: %v", dir, err)
	}
	t.Cleanup(func() { _ = r.Close() })
	root := r.FS()
	var files []string
	err = fs.WalkDir(root, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.EqualFold(filepath.Ext(path), ext) {
			files = append(files, path)
		}
		return nil
	})
	if err != nil || len(files) == 0 {
		t.Fatalf("no %s files under %s (err=%v)", ext, dir, err)
	}
	return root, files
}

// ampCorpus is corpus scoped to TRELLIS_AMP_CORPUS and .amp files.
func ampCorpus(t *testing.T) (fs.FS, []string) {
	return corpus(t, corpusEnv, ".amp")
}

// readAMP pulls the declared point count and the measurement member out of an
// AirMapper archive named within root, the filesystem ampCorpus rooted at the
// corpus directory. It goes through ParseAirMapperFile rather than reading the
// members itself, so "does this archive declare a count" is answered by the
// same code the daemon runs — including its refusal of a sidecar written for
// another survey's floor plan.
func readAMP(t *testing.T, root fs.FS, name string) ampParts {
	t.Helper()

	data, err := fs.ReadFile(root, name)
	if err != nil {
		t.Fatalf("read %s: %v", filepath.Base(name), err)
	}
	file, err := survey.ParseAirMapperFile(data)
	if err != nil {
		t.Fatalf("ParseAirMapperFile %s: %v", filepath.Base(name), err)
	}
	out := ampParts{declaredPoints: -1, surveyResult: file.SurveyResult}
	if file.Serial != nil {
		out.declaredPoints = file.Serial.SurveyPointCount
	}
	if file.ForeignSerial != "" {
		t.Logf("sidecar dropped: %s", file.ForeignSerial)
	}
	return out
}

func TestParseSurveyResultAgainstRealCaptures(t *testing.T) {
	root, files := ampCorpus(t)

	totalPoints, totalObs, totalActive := 0, 0, 0
	for _, path := range files {
		t.Run(filepath.Base(path), func(t *testing.T) {
			parts := readAMP(t, root, path)
			// Skipping here turned a corpus of unreadable archives into a
			// passing run. Every .amp in the corpus carries measurements.
			if parts.surveyResult == nil {
				t.Fatal("archive carries no .SurveyResult member")
			}

			points, parseErr := survey.ParseSurveyResult(parts.surveyResult)
			if parseErr != nil {
				t.Fatalf("ParseSurveyResult: %v", parseErr)
			}

			// A sidecar that declares a count is the oracle: the reader must
			// recover exactly that many. A sidecar that declares zero is a
			// malformed one and still fails — only a genuinely absent sidecar
			// (declaredPoints < 0) leaves the count unchecked, and the decoded
			// rows below carry the check in that case.
			switch {
			case parts.declaredPoints == 0:
				t.Fatal("archive declares 0 survey points; a sidecar that declares a count must declare a real one")
			case parts.declaredPoints > 0 && len(points) != parts.declaredPoints:
				t.Errorf("recovered %d points, the archive declares %d",
					len(points), parts.declaredPoints)
			case parts.declaredPoints < 0 && len(points) == 0:
				t.Error("archive declares no point count and decoded no points: nothing checked it at all")
			}

			obs, active := 0, 0
			for _, p := range points {
				obs += len(p.Networks)
				if p.Active != nil {
					active++
					if p.Active.RSSI > 0 || p.Active.RSSI < -110 {
						t.Fatalf("active RSSI %d dBm is outside a receiver's range", p.Active.RSSI)
					}
				}
				for _, n := range p.Networks {
					if n.Signal > 0 || n.Signal < -110 {
						t.Fatalf("signal %d dBm is outside a receiver's range", n.Signal)
					}
					if n.Channel < 0 || n.Channel > 233 {
						t.Errorf("channel %d is not an 802.11 channel", n.Channel)
					}
				}
			}
			// Every point must yield something. A point with neither passive
			// observations nor an active association means a field this reader
			// does not know about, which is the failure mode that made a
			// 245-point active survey look empty.
			measured := 0
			for _, p := range points {
				if len(p.Networks) > 0 || p.Active != nil {
					measured++
				}
			}
			if measured == 0 && len(points) > 0 {
				t.Errorf("%d points yielded no measurements of either kind", len(points))
			}

			totalPoints += len(points)
			totalObs += obs
			totalActive += active
			t.Logf("%d points, %d passive observations, %d active associations",
				len(points), obs, active)
		})
	}
	t.Logf("corpus totals: %d points, %d passive observations, %d active associations across %d files",
		totalPoints, totalObs, totalActive, len(files))

	// Without these the whole corpus could decode to nothing and still pass.
	if totalPoints == 0 {
		t.Error("corpus yielded no survey points at all")
	}
	if totalObs+totalActive == 0 {
		t.Error("corpus yielded no measurements of either kind")
	}
}
