// SPDX-License-Identifier: BUSL-1.1

package survey_test

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"hash/crc32"
	"image"
	"math"
	"strings"
	"testing"
)

// pngDeclaring builds a PNG whose IHDR declares a w*h RGBA canvas over an IDAT
// holding a single zeroed scanline. This is the shape of a decompression bomb:
// the file clears any byte-based cap and image/png allocates w*h*4 on the
// header alone, before it reads far enough to find the pixel data ends early.
//
// The scanline count is deliberately one and not h. Nothing these tests
// exercise reads pixel data — every guard refuses on DecodeConfig, which parses
// IHDR and stops — so streaming h scanlines through zlib only bought 1.6 GB of
// writes for the 20000x20000 case, and -race instruments every one of them
// (523 s in a single test against 13 s without, #463).
//
// Dimensions are uint32 because that is what a PNG header holds; taking them
// as int would put an unchecked narrowing conversion in every call.
func pngDeclaring(t *testing.T, w, h uint32) []byte {
	t.Helper()

	chunk := func(kind string, data []byte) []byte {
		var b bytes.Buffer
		n := uint64(len(data))
		if n > math.MaxUint32 {
			t.Fatalf("chunk %q is %d bytes, over what a PNG length field holds", kind, n)
		}
		_ = binary.Write(&b, binary.BigEndian, uint32(n))
		body := append([]byte(kind), data...)
		b.Write(body)
		_ = binary.Write(&b, binary.BigEndian, crc32.ChecksumIEEE(body))
		return b.Bytes()
	}

	var ihdr bytes.Buffer
	_ = binary.Write(&ihdr, binary.BigEndian, w)
	_ = binary.Write(&ihdr, binary.BigEndian, h)
	ihdr.Write([]byte{8, 6, 0, 0, 0}) // 8-bit RGBA, no interlace

	var idat bytes.Buffer
	zw := zlib.NewWriter(&idat)
	scanline := make([]byte, 1+int(w)*4) // leading filter byte, then zeroed pixels
	if _, err := zw.Write(scanline); err != nil {
		t.Fatalf("compress scanline: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zlib writer: %v", err)
	}

	var png bytes.Buffer
	png.Write([]byte("\x89PNG\r\n\x1a\n"))
	png.Write(chunk("IHDR", ihdr.Bytes()))
	png.Write(chunk("IDAT", idat.Bytes()))
	png.Write(chunk("IEND", nil))
	return png.Bytes()
}

// The premise the pixel bound exists for: the 32 MiB byte cap cannot see this
// coming, because a PNG's compressed size is unrelated to its decoded size.
func TestAnOversizedCanvasPassesTheByteCap(t *testing.T) {
	t.Parallel()

	b := pngDeclaring(t, 20000, 20000)

	// Well under maxFloorPlanBytes (32 MiB), so the byte cap admits it.
	if len(b) > 4<<20 {
		t.Fatalf("crafted plan is %d bytes; it must be far under the 32 MiB byte cap "+
			"or this proves nothing", len(b))
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(b))
	if err != nil {
		t.Fatalf("crafted plan does not decode as an image: %v", err)
	}
	if cfg.Width != 20000 || cfg.Height != 20000 {
		t.Fatalf("crafted plan declares %dx%d, want 20000x20000", cfg.Width, cfg.Height)
	}
	t.Logf("%.2f MiB declaring %d pixels — decoding would allocate %.2f GiB",
		float64(len(b))/(1<<20), cfg.Width*cfg.Height,
		float64(cfg.Width*cfg.Height*4)/(1<<30))
}

func TestSetFloorPlanRefusesAnOversizedCanvas(t *testing.T) {
	t.Parallel()

	mgr, id, floorID := planSurvey(t)

	err := mgr.SetFloorPlan(id, floorID, pngDeclaring(t, 20000, 20000))
	if err == nil {
		t.Fatal("stored a 400-megapixel floor plan; want a refusal")
	}
	// The message carries both numbers, as the byte cap's does, or an operator
	// cannot tell whether their own plan is near the limit.
	if !strings.Contains(err.Error(), "400000000") || !strings.Contains(err.Error(), "pixel") {
		t.Fatalf("refusal does not name the actual pixel count: %v", err)
	}
}

// The largest real plan across the 58-archive AirMapper corpus is 4096x3168
// (12.98 MP) and the median is 1.36 MP, so a genuine plan must be nowhere near
// the bound. A limit that refuses real work is worse than the bomb.
func TestSetFloorPlanAcceptsTheLargestRealCorpusPlan(t *testing.T) {
	t.Parallel()

	mgr, id, floorID := planSurvey(t)

	if err := mgr.SetFloorPlan(id, floorID, pngDeclaring(t, 4096, 3168)); err != nil {
		t.Fatalf("refused a 4096x3168 plan, the largest real one in the corpus: %v", err)
	}
}
