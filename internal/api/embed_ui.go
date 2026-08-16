// SPDX-License-Identifier: BUSL-1.1

package api

import (
	"embed"
	"io/fs"
	"net/http"
)

// uiFS embeds the built React frontend. Vite writes directly into ui/ (see
// ui/vite.config.ts's outDir) and Go embeds from there via go:embed — no
// copying, no syncing, per the fleet's Universal Build Contract. "all:" is
// required because the directory's only tracked file pre-build is the
// dot-prefixed .gitkeep, which a bare "ui/*" pattern would silently exclude
// and fail to compile against.
//
//go:embed all:ui
var uiFS embed.FS

// UIHandler serves the embedded frontend build from the root path. Returns
// an http.Handler backed by the ui/ subtree (dropping the "ui/" prefix so
// index.html serves at "/").
func UIHandler() (http.Handler, error) {
	sub, err := fs.Sub(uiFS, "ui")
	if err != nil {
		return nil, err
	}
	return http.FileServer(http.FS(sub)), nil
}
