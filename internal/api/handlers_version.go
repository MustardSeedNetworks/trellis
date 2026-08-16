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
// not a data endpoint.
func HandleBuildVersion(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(version.Info())
}
