// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"errors"
	"testing"
)

func TestCaptureModeName(t *testing.T) {
	tests := []struct {
		name string
		env  string
		err  error
		want string
	}{
		{"default is scan", "", nil, "scan"},
		{"explicit scan", "scan", nil, "scan"},
		{"monitor", "monitor", nil, "monitor"},
		{"failed backend is unavailable whatever was asked", "monitor", errors.New("no adapter"), "unavailable"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(captureModeEnv, tt.env)
			if got := captureModeName(tt.err); got != tt.want {
				t.Errorf("captureModeName = %q, want %q", got, tt.want)
			}
		})
	}
}
