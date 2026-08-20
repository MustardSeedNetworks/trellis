// SPDX-License-Identifier: BUSL-1.1

package api

import (
	"embed"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"path"
	"strings"
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

// indexFile is the single page every client route resolves to.
const indexFile = "index.html"

// UIHandler serves the embedded frontend build from the root path.
func UIHandler() (http.Handler, error) {
	sub, err := fs.Sub(uiFS, "ui")
	if err != nil {
		return nil, err
	}
	return uiHandler(sub), nil
}

// uiHandler serves a built single-page app out of fsys.
//
// It replaces a bare http.FileServer, which got three things wrong. Routes
// that exist only in the client router — /coverage, /import — have no file
// behind them, so every way of arriving at a page that is not a click
// answered 404: a bookmark, a refresh, a pasted link, a restored tab. A
// request for a directory answered with an index of the build, listing the
// asset names of a product that has no reason to publish them. And an
// unbuilt binary answered "/" with a listing of .gitkeep, which looks like a
// server that works.
//
// What stays a 404 is deliberate: a missing *asset*. A bundle that answers
// 200 with an HTML body is how an app ships a white screen and no error.
func uiHandler(fsys fs.FS) http.Handler {
	files := http.FileServer(http.FS(fsys))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isFile(fsys, r.URL.Path) {
			files.ServeHTTP(w, r)
			return
		}
		// An asset the build does not contain is missing, not a route.
		if path.Ext(path.Base(r.URL.Path)) != "" {
			http.NotFound(w, r)
			return
		}
		serveIndex(w, r, fsys)
	})
}

// isFile reports whether the path names a regular file in the build.
// Directories are excluded on purpose: serving one lists the build.
func isFile(fsys fs.FS, urlPath string) bool {
	name := strings.TrimPrefix(path.Clean(urlPath), "/")
	if name == "" || name == "." {
		return false
	}
	info, err := fs.Stat(fsys, name)
	return err == nil && !info.IsDir()
}

// serveIndex hands the client router its page.
func serveIndex(w http.ResponseWriter, r *http.Request, fsys fs.FS) {
	index, err := fsys.Open(indexFile)
	if err != nil {
		// No index means no build. Saying so beats listing the directory
		// that would have held it.
		if errors.Is(err, fs.ErrNotExist) {
			http.Error(w, "UI is not built into this binary", http.StatusNotFound)
			return
		}
		http.Error(w, "cannot read UI", http.StatusInternalServerError)
		return
	}
	defer func() { _ = index.Close() }()

	readSeeker, ok := index.(io.ReadSeeker)
	if !ok {
		http.Error(w, "cannot read UI", http.StatusInternalServerError)
		return
	}
	info, err := index.Stat()
	if err != nil {
		http.Error(w, "cannot read UI", http.StatusInternalServerError)
		return
	}
	// ServeContent sets the type from the name and handles conditional
	// requests; the SPA shell is small and changes with every build.
	http.ServeContent(w, r, indexFile, info.ModTime(), readSeeker)
}
