// SPDX-License-Identifier: BUSL-1.1

package survey

import "testing"

// DecodeAirMagnetTextForTest exposes the UTF-16 decode to the corpus test,
// which counts rows in the same file the parser read and must decode it the
// same way to be an independent count rather than a second guess at it.
func DecodeAirMagnetTextForTest(t *testing.T, data []byte) string {
	t.Helper()
	text, err := decodeAirMagnetText(data)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	return text
}
