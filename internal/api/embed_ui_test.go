// SPDX-License-Identifier: BUSL-1.1

package api_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/MustardSeedNetworks/trellis/internal/api"
)

const indexBody = "<!doctype html><div id=\"root\"></div>"

func builtUI() fstest.MapFS {
	return fstest.MapFS{
		"index.html":         {Data: []byte(indexBody)},
		"assets/app-abc.js":  {Data: []byte("console.log(1)")},
		"assets/app-abc.css": {Data: []byte(".a{}")},
	}
}

// TestUIHandlerServesClientRoutes proves a page can be reached by URL and not
// only by clicking the rail.
//
// The handler was a bare http.FileServer, so every route that exists only in
// the client router answered 404: a bookmark, a refresh, a pasted link and a
// restored tab all failed on pages that worked when navigated to. /import had
// behaved this way since it shipped; /coverage joined it.
func TestUIHandlerServesClientRoutes(t *testing.T) {
	handler := api.UIHandlerForTest(builtUI())

	for _, urlPath := range []string{"/", "/coverage", "/import", "/surveys/abc123", "/floors"} {
		t.Run(urlPath, func(t *testing.T) {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, urlPath, nil))

			if rec.Code != http.StatusOK {
				t.Fatalf("GET %s = %d, want 200", urlPath, rec.Code)
			}
			if rec.Body.String() != indexBody {
				t.Errorf("GET %s served %q, want the app shell", urlPath, rec.Body.String())
			}
		})
	}
}

// A bundle that answers 200 with an HTML body is how an app ships a white
// screen and no error, so a missing asset has to stay a 404 even though every
// other unknown path now resolves to the shell.
func TestUIHandlerKeeps404ForMissingAssets(t *testing.T) {
	handler := api.UIHandlerForTest(builtUI())

	for _, urlPath := range []string{"/assets/gone.js", "/index.css", "/favicon.ico"} {
		t.Run(urlPath, func(t *testing.T) {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, urlPath, nil))

			if rec.Code != http.StatusNotFound {
				t.Errorf("GET %s = %d, want 404", urlPath, rec.Code)
			}
		})
	}
}

func TestUIHandlerServesRealAssets(t *testing.T) {
	handler := api.UIHandlerForTest(builtUI())

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/assets/app-abc.js", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /assets/app-abc.js = %d, want 200", rec.Code)
	}
	if rec.Body.String() != "console.log(1)" {
		t.Errorf("served %q, want the asset's own bytes", rec.Body.String())
	}
}

// A directory request used to answer with an index of the build, publishing
// the asset names of a product that has no reason to publish them.
func TestUIHandlerNeverListsTheBuild(t *testing.T) {
	handler := api.UIHandlerForTest(builtUI())

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/assets/", nil))

	if body := rec.Body.String(); body != indexBody {
		t.Errorf("GET /assets/ served %q, want the app shell rather than a listing", body)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("GET /assets/ = %d, want the shell's 200", rec.Code)
	}
}

// An unbuilt binary used to answer "/" with a listing containing .gitkeep,
// which reads as a server that works.
func TestUIHandlerSaysWhenTheUIIsMissing(t *testing.T) {
	handler := api.UIHandlerForTest(fstest.MapFS{".gitkeep": {Data: []byte{}}})

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("GET / on an unbuilt binary = %d, want 404", rec.Code)
	}
	if body := rec.Body.String(); body == "" {
		t.Error("expected a message saying the UI is not built in")
	}
}
