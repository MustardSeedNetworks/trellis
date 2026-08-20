// SPDX-License-Identifier: BUSL-1.1

package api

import (
	"io/fs"
	"net/http"
)

// UIHandlerForTest exposes the UI handler over an arbitrary filesystem, so the
// tests can assert on an exact build rather than on whatever the working tree
// happens to have compiled into the embed. The embedded ui/ holds only
// .gitkeep until Vite has run, and a test that passes against that proves
// nothing about what ships.
func UIHandlerForTest(fsys fs.FS) http.Handler { return uiHandler(fsys) }
