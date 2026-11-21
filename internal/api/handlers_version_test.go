// SPDX-License-Identifier: BUSL-1.1

package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/MustardSeedNetworks/trellis/internal/api"
)

func TestHandleBuildVersionServesReadMethods(t *testing.T) {
	for _, method := range []string{http.MethodGet, http.MethodHead} {
		t.Run(method, func(t *testing.T) {
			rec := httptest.NewRecorder()
			api.HandleBuildVersion(rec, httptest.NewRequestWithContext(t.Context(), method, "/__version", nil))

			if rec.Code != http.StatusOK {
				t.Fatalf("%s /__version = %d, want 200", method, rec.Code)
			}
			if got := rec.Header().Get("Content-Type"); got != "application/json" {
				t.Errorf("Content-Type = %q, want application/json", got)
			}
			if method != http.MethodGet {
				return
			}
			var info map[string]string
			if err := json.NewDecoder(rec.Body).Decode(&info); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			for _, key := range []string{"version", "commit", "buildTime", "uiBuildHash"} {
				if info[key] == "" {
					t.Errorf("body has no %q: %v", key, info)
				}
			}
		})
	}
}

func TestHandleBuildVersionRefusesOtherMethods(t *testing.T) {
	for _, method := range []string{
		http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodOptions,
	} {
		t.Run(method, func(t *testing.T) {
			rec := httptest.NewRecorder()
			api.HandleBuildVersion(rec, httptest.NewRequestWithContext(t.Context(), method, "/__version", nil))

			if rec.Code != http.StatusMethodNotAllowed {
				t.Fatalf("%s /__version = %d, want 405", method, rec.Code)
			}
			if got := rec.Header().Get("Allow"); got != "GET, HEAD" {
				t.Errorf("Allow = %q, want %q", got, "GET, HEAD")
			}
		})
	}
}
