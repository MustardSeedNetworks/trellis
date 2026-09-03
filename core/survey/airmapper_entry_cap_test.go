// SPDX-License-Identifier: BUSL-1.1

package survey_test

import (
	"archive/zip"
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/MustardSeedNetworks/trellis/core/survey"
)

// A zip bomb is small on the wire and enormous inflated. The transport's cap
// sees only the wire size, so the parser has to stop the inflation itself; the
// assertion is that it does so with the sentinel and before the archive's
// other checks run.
func TestParseAirMapperFileRejectsAnEntryThatInflatesPastTheCap(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.CreateHeader(&zip.FileHeader{Name: "floorplan.png", Method: zip.Deflate})
	if err != nil {
		t.Fatalf("create entry: %v", err)
	}
	// 64 MiB + 1 of zeros deflates to well under 100 KiB.
	if _, err := io.CopyN(w, zeros{}, (64<<20)+1); err != nil {
		t.Fatalf("write entry: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close archive: %v", err)
	}
	if buf.Len() > 1<<20 {
		t.Fatalf("fixture is %d bytes on the wire; it must be small to prove the point", buf.Len())
	}

	_, err = survey.ParseAirMapperFile(buf.Bytes())
	if !errors.Is(err, survey.ErrArchiveEntryTooLarge) {
		t.Fatalf("ParseAirMapperFile = %v, want ErrArchiveEntryTooLarge", err)
	}
}

// zeros is an endless source of zero bytes.
type zeros struct{}

func (zeros) Read(p []byte) (int, error) {
	clear(p)
	return len(p), nil
}
