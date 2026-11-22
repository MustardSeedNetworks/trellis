// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestOpenLogFileCreatesOwnerOnlyFileAndRotates(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the only Windows log is the service's; logfile_windows_test.go covers it")
	}
	path := filepath.Join(t.TempDir(), "logs", "trellisd.log")

	writer, err := openLogFile(path)
	if err != nil {
		t.Fatalf("openLogFile: %v", err)
	}
	if _, err := writer.Write([]byte("hello\n")); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	if writer.MaxSize != logMaxSizeMB || writer.MaxBackups != logMaxBackups || writer.MaxAge != logMaxAgeDays {
		t.Errorf("rotation bounds = %d MB, %d backups, %d days", writer.MaxSize, writer.MaxBackups, writer.MaxAge)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	if string(got) != "hello\n" {
		t.Errorf("log = %q, want %q", got, "hello\n")
	}
	for p, want := range map[string]os.FileMode{filepath.Dir(path): logDirPerm, path: logFilePerm} {
		info, err := os.Stat(p)
		if err != nil {
			t.Fatalf("stat %s: %v", p, err)
		}
		if info.Mode().Perm() != want {
			t.Errorf("%s mode = %o, want %o", p, info.Mode().Perm(), want)
		}
	}
}
