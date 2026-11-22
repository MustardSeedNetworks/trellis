// SPDX-License-Identifier: BUSL-1.1

//go:build windows

package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/MustardSeedNetworks/foundation/pkg/securedir"
	"gopkg.in/natefinch/lumberjack.v2"
)

func TestServiceLogFileRotatesThroughSecuredir(t *testing.T) {
	t.Parallel()

	trust := currentUserTrust(t)
	dir := filepath.Join(t.TempDir(), "logs")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	protect(t, dir, trust)
	path := filepath.Join(dir, "trellisd.log")

	if err := createLogFileWith(path, trust); err != nil {
		t.Fatalf("createLogFileWith: %v", err)
	}
	logger := &lumberjack.Logger{Filename: path, MaxSize: logMaxSizeMB}
	if _, err := logger.Write([]byte("before\n")); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := logger.Rotate(); err != nil {
		t.Fatalf("rotate: %v", err)
	}
	if _, err := logger.Write([]byte("after\n")); err != nil {
		t.Fatalf("write after rotate: %v", err)
	}
	if err := logger.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	if got, err := os.ReadFile(path); err != nil || string(got) != "after\n" {
		t.Errorf("live log = %q, %v; want %q", got, err, "after\n")
	}
	backups, err := filepath.Glob(filepath.Join(dir, "trellisd-*.log"))
	if err != nil || len(backups) != 1 {
		t.Fatalf("rotated backups = %v, %v; want one", backups, err)
	}
	// The rotated file was created by lumberjack, not securedir; it must still
	// be something the service would verify, which is what a restart does.
	if err := securedir.Verify(dir, trust); err != nil {
		t.Errorf("Verify after rotation: %v", err)
	}
}

func TestCreateLogFileRefusesWhatSecuredirRefuses(t *testing.T) {
	t.Parallel()

	trust := currentUserTrust(t)
	dir := filepath.Join(t.TempDir(), "logs")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	protect(t, dir, trust)

	target := filepath.Join(t.TempDir(), "elsewhere.log")
	if err := os.WriteFile(target, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "linked.log")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("cannot create a symbolic link on this runner: %v", err)
	}

	tests := []struct {
		name string
		path string
		want error
	}{
		{name: "a link planted as the log file", path: link, want: securedir.ErrReparsePoint},
		{name: "a missing log directory", path: filepath.Join(dir, "absent", "trellisd.log"), want: os.ErrNotExist},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if err := createLogFileWith(tc.path, trust); !errors.Is(err, tc.want) {
				t.Fatalf("createLogFileWith() = %v, want %v", err, tc.want)
			}
		})
	}
}
