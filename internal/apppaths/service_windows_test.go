// SPDX-License-Identifier: BUSL-1.1

//go:build windows

package apppaths

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestServiceLogFile(t *testing.T) {
	t.Parallel()

	got, err := serviceLogFile()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join("Trellis", "logs", "trellisd.log")
	if !filepath.IsAbs(got) || !strings.HasSuffix(got, want) {
		t.Fatalf("serviceLogFile() = %q, want an absolute path ending in %q", got, want)
	}
}

// A test process is not started by the Service Control Manager, so it logs
// the way any console process does.
func TestUnderServiceIsFalseInATest(t *testing.T) {
	t.Parallel()

	if underService() {
		t.Fatal("underService() = true in a test process")
	}
}
