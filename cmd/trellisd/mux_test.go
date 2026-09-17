// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	surveyv1 "github.com/MustardSeedNetworks/trellis/gen/trellis/survey/v1"
	"github.com/MustardSeedNetworks/trellis/gen/trellis/survey/v1/surveyv1connect"
	"github.com/MustardSeedNetworks/trellis/internal/auth"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// The RPCs are read out of the compiled descriptor rather than listed here, so
// an RPC added to the proto is covered by this test the day it is added — which
// is the point: the composition was previously asserted by nothing at all, and
// a hand-written list would have gone stale the same way (#520).
func surveyProcedures(t *testing.T) []string {
	t.Helper()

	services := surveyv1.File_trellis_survey_v1_survey_proto.Services()
	if services.Len() != 1 {
		t.Fatalf("survey.proto declares %d services, want 1", services.Len())
	}
	service := services.Get(0)
	methods := service.Methods()
	if methods.Len() == 0 {
		t.Fatal("survey service declares no RPCs; this test would assert nothing")
	}

	procedures := make([]string, 0, methods.Len())
	for i := range methods.Len() {
		procedures = append(procedures, procedurePath(service, methods.Get(i)))
	}
	return procedures
}

func procedurePath(service protoreflect.ServiceDescriptor, method protoreflect.MethodDescriptor) string {
	return "/" + string(service.FullName()) + "/" + string(method.Name())
}

// callAnonymously makes the request an unauthenticated caller would: a Connect
// POST with a valid empty JSON message and no cookies or CSRF token. The body
// has to decode, because connect-go unmarshals before the unary interceptor
// runs and a malformed one would be refused as invalid_argument without auth
// ever being consulted.
func callAnonymously(t *testing.T, mux *http.ServeMux, procedure string) (int, string) {
	t.Helper()

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, procedure, strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	var body struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("%s: response %d is not Connect JSON: %v (%q)", procedure, rec.Code, err, rec.Body.String())
	}
	return rec.Code, body.Code
}

func gatedMux(t *testing.T) *http.ServeMux {
	t.Helper()

	credential, err := auth.NewCredential("surveyor", "correct-horse-battery-staple")
	if err != nil {
		t.Fatalf("NewCredential: %v", err)
	}
	gate, err := auth.NewGate(credential)
	if err != nil {
		t.Fatalf("NewGate: %v", err)
	}
	t.Cleanup(gate.Close)

	return newMux(muxDeps{
		survey: surveyv1connect.UnimplementedSurveyServiceHandler{},
		gate:   gate,
		ui:     http.NotFoundHandler(),
	})
}

// TestGatedMuxRefusesEveryRPCAnonymously is the composition test: it asserts
// the wiring, not the interceptor. The interceptor has its own tests and they
// passed while nothing checked that the survey service was actually served
// behind it.
func TestGatedMuxRefusesEveryRPCAnonymously(t *testing.T) {
	mux := gatedMux(t)

	for _, procedure := range surveyProcedures(t) {
		status, code := callAnonymously(t, mux, procedure)
		// The Connect code, not merely "not 200": a typo in the path (404), a
		// wrong content type (415) or an unwrapped handler (unimplemented) all
		// refuse the call too, and none of them is authentication.
		if status != http.StatusUnauthorized || code != "unauthenticated" {
			t.Errorf("%s anonymous = %d %q, want 401 \"unauthenticated\"", procedure, status, code)
		}
	}
}

// TestUngatedMuxServesEveryRPCAnonymously is the control. Without it the
// assertion above could hold for a reason that has nothing to do with the gate
// — every RPC of an Unimplemented service refuses the call as well. Reaching
// the handler is what "unimplemented" proves.
func TestUngatedMuxServesEveryRPCAnonymously(t *testing.T) {
	mux := newMux(muxDeps{
		survey: surveyv1connect.UnimplementedSurveyServiceHandler{},
		ui:     http.NotFoundHandler(),
	})

	for _, procedure := range surveyProcedures(t) {
		status, code := callAnonymously(t, mux, procedure)
		if code != "unimplemented" {
			t.Errorf("%s on the loopback daemon = %d %q, want \"unimplemented\"", procedure, status, code)
		}
	}
}

// The version endpoint is outside the gate by build-contract rule 4: a
// deployment check reads it with no credentials. /healthz is there for the same
// reason, and the login routes have to be reachable or nobody can log in.
func TestGatedMuxLeavesTheUnauthenticatedRoutesOpen(t *testing.T) {
	mux := gatedMux(t)

	for _, route := range []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/__version"},
		{http.MethodGet, "/healthz"},
	} {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), route.method, route.path, nil))
		if rec.Code != http.StatusOK {
			t.Errorf("anonymous %s %s = %d, want 200", route.method, route.path, rec.Code)
		}
	}

	// /auth/login is reachable without a session; it refuses these credentials
	// rather than the caller's lack of one.
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/auth/login", strings.NewReader(`{"username":"surveyor","password":"wrong"}`))
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("anonymous POST /auth/login with a bad password = %d, want 401", rec.Code)
	}
}
