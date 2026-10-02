// SPDX-License-Identifier: BUSL-1.1

package api

import (
	"testing"

	"connectrpc.com/connect"
	surveyv1 "github.com/MustardSeedNetworks/trellis/gen/trellis/survey/v1"
)

// TestReportOptions covers the one place a report's contents are decided.
//
// The handler used to pass DefaultReportOptions() unconditionally, so the five
// options core/survey reads were unreachable. Now that they cross the wire,
// two behaviours matter and pull in opposite directions: a request that says
// nothing should still get the defaults, and a request that does say something
// must be taken literally — proto3 cannot tell an unset bool from false, so
// "helpfully" re-enabling a section would silently override the operator.
func TestReportOptions(t *testing.T) {
	t.Parallel()

	t.Run("no options means the engine defaults", func(t *testing.T) {
		t.Parallel()
		got, err := reportOptions(nil)
		if err != nil {
			t.Fatalf("reportOptions(nil): %v", err)
		}

		if !got.IncludeExecutiveSummary || !got.IncludeRecommendations || !got.IncludeHeatmaps {
			t.Errorf("defaults dropped a section that was on: %+v", got)
		}
		if got.IncludeRawData {
			t.Error("raw data is off by default; the appendix is long")
		}
	})

	t.Run("sections the caller turned off stay off", func(t *testing.T) {
		t.Parallel()
		got, err := reportOptions(&surveyv1.ReportOptions{
			IncludeExecutiveSummary: true,
			IncludeRecommendations:  false,
			IncludeHeatmaps:         false,
			IncludeRawData:          true,
		})
		if err != nil {
			t.Fatalf("reportOptions: %v", err)
		}

		if got.IncludeRecommendations || got.IncludeHeatmaps {
			t.Errorf("a section the operator turned off came back on: %+v", got)
		}
		if !got.IncludeExecutiveSummary || !got.IncludeRawData {
			t.Errorf("a section the operator turned on was dropped: %+v", got)
		}
	})

	t.Run("the company name reaches the cover page", func(t *testing.T) {
		t.Parallel()
		got, err := reportOptions(&surveyv1.ReportOptions{CompanyName: "Mustard Seed Networks"})
		if err != nil {
			t.Fatalf("reportOptions: %v", err)
		}

		if got.CompanyName != "Mustard Seed Networks" {
			t.Errorf("CompanyName = %q, want it carried through", got.CompanyName)
		}
	})

	t.Run("a metric with no dead-zone rule is refused", func(t *testing.T) {
		t.Parallel()
		_, err := reportOptions(&surveyv1.ReportOptions{Metric: "throughput"})

		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("err = %v, want InvalidArgument", err)
		}
	})
}
