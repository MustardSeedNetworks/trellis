// SPDX-License-Identifier: BUSL-1.1

package api_test

// The PDF used to be analysed at the RSSI default whatever the operator chose
// on the Coverage page: the Go report options had a metric and threshold, the
// wire message did not (#509). These tests read the findings back out of the
// rendered PDF, because the mapping is only fixed if the document says so.

import (
	"bytes"
	"compress/zlib"
	"context"
	"io"
	"regexp"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"github.com/MustardSeedNetworks/trellis/core/wifi"
	surveyv1 "github.com/MustardSeedNetworks/trellis/gen/trellis/survey/v1"
	"github.com/MustardSeedNetworks/trellis/internal/api"
)

var (
	pdfStream = regexp.MustCompile(`(?s)stream\r?\n(.*?)\r?\nendstream`)
	pdfShown  = regexp.MustCompile(`\(((?:[^()\\]|\\.)*)\)\s*Tj`)
)

// reportText is every string the report's content streams print, in order.
func reportText(t *testing.T, pdf []byte) string {
	t.Helper()
	var text strings.Builder
	for _, m := range pdfStream.FindAllSubmatch(pdf, -1) {
		r, err := zlib.NewReader(bytes.NewReader(m[1]))
		if err != nil {
			continue // not a deflated stream
		}
		content, err := io.ReadAll(r)
		if err != nil {
			continue // image data, not text
		}
		for _, shown := range pdfShown.FindAllSubmatch(content, -1) {
			text.Write(shown[1])
			text.WriteByte('\n')
		}
	}
	return text.String()
}

// marginFloorSurvey walks a floor at -55 dBm with a 22 dB margin: clean
// against either metric's default, short of a 25 dB SNR threshold.
func marginFloorSurvey(t *testing.T) (*api.SurveyServiceHandler, string) {
	t.Helper()

	manager := mustManager(t, t.TempDir(), scriptedScanner{networks: []wifi.ScannedNetwork{
		{SSID: "margin", BSSID: "aa:bb:cc:00:00:0a", Signal: -55, NoiseFloor: -77, SNR: 22, Channel: 6, Frequency: 2437},
	}}, nil, nil, nil)
	handler := api.NewSurveyServiceHandler(manager)
	ctx := context.Background()

	created, err := handler.CreateSurvey(ctx,
		connect.NewRequest(&surveyv1.CreateSurveyRequest{Name: "margin", Interface: "en0"}))
	if err != nil {
		t.Fatalf("CreateSurvey: %v", err)
	}
	id := created.Msg.GetSurvey().GetId()
	if _, err := handler.StartSurvey(ctx,
		connect.NewRequest(&surveyv1.StartSurveyRequest{Id: id})); err != nil {
		t.Fatalf("StartSurvey: %v", err)
	}
	for _, at := range []struct{ x, y int32 }{{10, 10}, {60, 10}, {110, 10}, {160, 10}} {
		if _, err := handler.CapturePoint(ctx,
			connect.NewRequest(&surveyv1.CapturePointRequest{SurveyId: id, X: at.x, Y: at.y})); err != nil {
			t.Fatalf("CapturePoint: %v", err)
		}
	}
	return handler, id
}

func generateFindings(t *testing.T, options *surveyv1.ReportOptions) string {
	t.Helper()
	handler, id := marginFloorSurvey(t)
	resp, err := handler.GenerateReport(context.Background(),
		connect.NewRequest(&surveyv1.GenerateReportRequest{SurveyId: id, Options: options}))
	if err != nil {
		t.Fatalf("GenerateReport: %v", err)
	}
	return reportText(t, resp.Msg.GetPdf())
}

func TestGenerateReport_AnalysesAtTheRequestedMetricAndThreshold(t *testing.T) {
	t.Parallel()

	text := generateFindings(t, &surveyv1.ReportOptions{
		IncludeRecommendations: true, Metric: "snr", Threshold: 25,
	})

	if !strings.Contains(text, "a margin between 20 and 25 dB") {
		t.Errorf("the findings are not the SNR analysis at the requested 25 dB:\n%s", text)
	}
}

func TestGenerateReport_NoMetricOrThresholdIsTheRSSIDefault(t *testing.T) {
	t.Parallel()

	text := generateFindings(t, &surveyv1.ReportOptions{IncludeRecommendations: true})

	if !strings.Contains(text, "No significant dead zones detected.") {
		t.Errorf("an unset metric was not analysed as RSSI at its default:\n%s", text)
	}
	if strings.Contains(text, "noise") {
		t.Errorf("an unset metric produced SNR findings:\n%s", text)
	}
}

func TestGenerateReport_RefusesAMetricWithNoDeadZoneRule(t *testing.T) {
	t.Parallel()

	handler, id := marginFloorSurvey(t)
	_, err := handler.GenerateReport(context.Background(),
		connect.NewRequest(&surveyv1.GenerateReportRequest{
			SurveyId: id, Options: &surveyv1.ReportOptions{Metric: "throughput"},
		}))

	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("err = %v, want InvalidArgument", err)
	}
}
