// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestHandleArgs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		args       []string
		wantDone   bool
		wantCode   int
		wantStdout string
		wantStderr string
	}{
		{name: "no arguments serves", args: nil, wantDone: false, wantCode: exitOK},
		{name: "--version prints and exits", args: []string{"--version"}, wantDone: true, wantCode: exitOK, wantStdout: "trellisd "},
		{name: "-version prints and exits", args: []string{"-version"}, wantDone: true, wantCode: exitOK, wantStdout: "trellisd "},
		{name: "--help prints usage and exits", args: []string{"--help"}, wantDone: true, wantCode: exitOK, wantStdout: "Usage: trellisd"},
		{name: "-h prints usage and exits", args: []string{"-h"}, wantDone: true, wantCode: exitOK, wantStdout: "TRELLIS_ADDR"},
		{name: "unknown flag is refused", args: []string{"--port=9000"}, wantDone: true, wantCode: exitUsage, wantStderr: "flag provided but not defined"},
		{name: "positional argument is refused", args: []string{"serve"}, wantDone: true, wantCode: exitUsage, wantStderr: `unexpected argument "serve"`},
		{name: "argument after --version is refused", args: []string{"--version", "extra"}, wantDone: true, wantCode: exitUsage, wantStderr: `unexpected argument "extra"`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var stdout, stderr bytes.Buffer

			code, done := handleArgs(tc.args, &stdout, &stderr)

			if done != tc.wantDone || code != tc.wantCode {
				t.Fatalf("handleArgs(%q) = (%d, %v), want (%d, %v)", tc.args, code, done, tc.wantCode, tc.wantDone)
			}
			if !strings.Contains(stdout.String(), tc.wantStdout) {
				t.Errorf("stdout = %q, want it to contain %q", stdout.String(), tc.wantStdout)
			}
			if !strings.Contains(stderr.String(), tc.wantStderr) {
				t.Errorf("stderr = %q, want it to contain %q", stderr.String(), tc.wantStderr)
			}
			if !tc.wantDone && stdout.Len()+stderr.Len() > 0 {
				t.Errorf("serving path wrote output: stdout %q, stderr %q", stdout.String(), stderr.String())
			}
		})
	}
}
