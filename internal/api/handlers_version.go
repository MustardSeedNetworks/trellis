// SPDX-License-Identifier: BUSL-1.1

package api

import (
	"encoding/json"
	"net/http"

	"github.com/MustardSeedNetworks/trellis/internal/version"
)

// HandleBuildVersion serves GET /__version with build metadata for
// deployment validation, matching the /__version contract shared across
// seed, stem, and niac. No auth: it is a deployment introspection surface,
// not a data endpoint. Any method other than GET or HEAD is refused with 405.
func HandleBuildVersion(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(version.Info())
}
