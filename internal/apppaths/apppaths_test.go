// SPDX-License-Identifier: BUSL-1.1

package apppaths

import (
	"path/filepath"
	"testing"
)

func TestInBundle(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		exe  string
		want bool
	}{
		{
			name: "bundled executable",
			exe:  "/Applications/Trellis.app/Contents/MacOS/trellisd",
			want: true,
		},
		{
			name: "bundle in a user directory",
			exe:  "/Users/surveyor/dist/macos/Trellis.app/Contents/MacOS/trellisd",
			want: true,
		},
		{
			name: "bare binary from an archive",
			exe:  "/usr/local/bin/trellisd",
			want: false,
		},
		{
			name: "a directory merely named like a bundle",
			exe:  "/tmp/Trellis.app/trellisd",
			want: false,
		},
		{
			name: "Contents/MacOS outside a bundle",
			exe:  "/tmp/Contents/MacOS/trellisd",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := inBundle(filepath.FromSlash(tt.exe)); got != tt.want {
				t.Errorf("inBundle(%q) = %v, want %v", tt.exe, got, tt.want)
			}
		})
	}
}

func TestDataDirIsAbsolute(t *testing.T) {
	t.Parallel()

	// LaunchServices starts a bundled app with the working directory set to
	// "/", so a relative default would put the survey store in the root of the
	// filesystem and fail. Nothing about the default may depend on the cwd.
	dir, err := DataDir()
	if err != nil {
		t.Fatalf("DataDir: %v", err)
	}
	if !filepath.IsAbs(dir) {
		t.Errorf("DataDir() = %q, want an absolute path", dir)
	}
}
