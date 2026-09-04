// SPDX-License-Identifier: BUSL-1.1

package api

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/MustardSeedNetworks/trellis/core/survey"
	"github.com/MustardSeedNetworks/trellis/core/wifi"
	surveyv1 "github.com/MustardSeedNetworks/trellis/gen/trellis/survey/v1"
	"github.com/MustardSeedNetworks/trellis/internal/capture"
)

// int32Of narrows a measurement for the wire.
//
// Every value it is used on is a physical radio quantity — dBm, a channel
// number, MHz, a count of BSSs at one point — that cannot approach int32 on any
// hardware. The bound is here because the compiler cannot know that, and a
// value that trips it means the sample is already corrupt, so saturating is
// closer to the truth than wrapping.
func int32Of(v int) int32 {
	switch {
	case v > math.MaxInt32:
		return math.MaxInt32
	case v < math.MinInt32:
		return math.MinInt32
	default:
		return int32(v)
	}
}

// permissionFix is the one remedy an operator can act on, so it travels with
// the error rather than being logged where nobody reading the API response sees
// it. What the remedy *is* differs per platform — a Location Services setting
// on macOS, CAP_NET_ADMIN on Linux — and the capture package is where that is
// known, so the text comes from there. Answering a Linux capability failure
// with macOS System Settings directions is worse than saying nothing.
func permissionFix() string {
	if capture.PermissionRemedy == "" {
		return ""
	}
	return ": " + capture.PermissionRemedy
}

// CreateSurvey opens a new, empty survey to walk.
func (h *SurveyServiceHandler) CreateSurvey(
	_ context.Context,
	req *connect.Request[surveyv1.CreateSurveyRequest],
) (*connect.Response[surveyv1.CreateSurveyResponse], error) {
	name := strings.TrimSpace(req.Msg.GetName())
	if name == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("name is required"))
	}

	// Passive is the only type this service can gather; see CreateSurveyRequest.
	created, err := h.manager.CreateSurvey(
		name, req.Msg.GetDescription(), req.Msg.GetInterface(), survey.TypePassive)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&surveyv1.CreateSurveyResponse{Survey: toSurveySummary(created)}), nil
}

// StartSurvey moves a survey into the state that accepts samples.
func (h *SurveyServiceHandler) StartSurvey(
	_ context.Context,
	req *connect.Request[surveyv1.StartSurveyRequest],
) (*connect.Response[surveyv1.StartSurveyResponse], error) {
	s, err := h.transition(req.Msg.GetId(), h.manager.StartSurvey)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&surveyv1.StartSurveyResponse{Survey: toSurveySummary(s)}), nil
}

// PauseSurvey stops a survey accepting samples without completing it.
func (h *SurveyServiceHandler) PauseSurvey(
	_ context.Context,
	req *connect.Request[surveyv1.PauseSurveyRequest],
) (*connect.Response[surveyv1.PauseSurveyResponse], error) {
	s, err := h.transition(req.Msg.GetId(), h.manager.PauseSurvey)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&surveyv1.PauseSurveyResponse{Survey: toSurveySummary(s)}), nil
}

// CompleteSurvey closes a survey to further samples.
func (h *SurveyServiceHandler) CompleteSurvey(
	_ context.Context,
	req *connect.Request[surveyv1.CompleteSurveyRequest],
) (*connect.Response[surveyv1.CompleteSurveyResponse], error) {
	s, err := h.transition(req.Msg.GetId(), h.manager.CompleteSurvey)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&surveyv1.CompleteSurveyResponse{Survey: toSurveySummary(s)}), nil
}

// CapturePoint scans the airspace and records it at a position on the survey's
// active floor.
func (h *SurveyServiceHandler) CapturePoint(
	ctx context.Context,
	req *connect.Request[surveyv1.CapturePointRequest],
) (*connect.Response[surveyv1.CapturePointResponse], error) {
	id := req.Msg.GetSurveyId()
	if id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("survey_id is required"))
	}

	sample, err := h.manager.CapturePoint(ctx, id, int(req.Msg.GetX()), int(req.Msg.GetY()))
	if err != nil {
		return nil, captureError(err)
	}

	reply := &surveyv1.CapturePointResponse{
		Networks:       make([]*surveyv1.ScannedNetwork, len(sample.Networks)),
		UniqueSsids:    int32Of(sample.UniqueSSIDs),
		UniqueBssids:   int32Of(sample.UniqueBSSIDs),
		ApCount_2_4Ghz: int32Of(sample.APCount2_4),
		ApCount_5Ghz:   int32Of(sample.APCount5),
		ApCount_6Ghz:   int32Of(sample.APCount6),
		CoChannelAps:   int32Of(sample.CoChannelAPs),
		AdjChannelAps:  int32Of(sample.AdjChannelAPs),
	}
	for i, n := range sample.Networks {
		reply.Networks[i] = scannedNetworkOf(n)
	}
	return connect.NewResponse(reply), nil
}

// Scan reads the airspace and stores nothing. The live analysis view polls it.
func (h *SurveyServiceHandler) Scan(
	ctx context.Context,
	_ *connect.Request[surveyv1.ScanRequest],
) (*connect.Response[surveyv1.ScanResponse], error) {
	networks, err := h.manager.Scan(ctx)
	if err != nil {
		return nil, captureError(err)
	}

	reply := &surveyv1.ScanResponse{
		Networks:  make([]*surveyv1.ScannedNetwork, len(networks)),
		ScannedAt: timestamppb.New(time.Now().UTC()),
	}
	for i := range networks {
		reply.Networks[i] = scannedNetworkOf(&networks[i])
	}
	return connect.NewResponse(reply), nil
}

// transition applies a state change by ID and returns the survey as it stands
// afterwards, so a caller sees the new status without a second round trip.
func (h *SurveyServiceHandler) transition(
	id string,
	apply func(string) error,
) (*survey.Survey, error) {
	if id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id is required"))
	}
	if err := apply(id); err != nil {
		if errors.Is(err, survey.ErrSurveyNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		// A refused transition is the caller asking for something the survey's
		// current state does not allow, not a server fault.
		return nil, connect.NewError(connect.CodeFailedPrecondition, err)
	}
	return h.manager.GetSurvey(id)
}

// captureError maps a capture failure onto a code that tells the caller whose
// problem it is.
func captureError(err error) error {
	switch {
	case errors.Is(err, survey.ErrSurveyNotFound):
		return connect.NewError(connect.CodeNotFound, err)

	case errors.Is(err, capture.ErrPermission):
		// The one failure an operator fixes by doing something. Burying it in
		// CodeInternal would hide the remedy behind "internal error".
		return connect.NewError(connect.CodeFailedPrecondition,
			fmt.Errorf("%w%s", err, permissionFix()))

	case errors.Is(err, capture.ErrUnsupported), errors.Is(err, survey.ErrNoScanner):
		return connect.NewError(connect.CodeUnimplemented, err)

	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return connect.NewError(connect.CodeCanceled, err)

	default:
		// Everything else is either a refused state transition (survey not in
		// progress, no active floor) or a radio failure. Both are conditions of
		// the request, not server faults.
		return connect.NewError(connect.CodeFailedPrecondition, err)
	}
}

// scannedNetworkOf mirrors core/wifi.ScannedNetwork onto the wire type.
func scannedNetworkOf(n *wifi.ScannedNetwork) *surveyv1.ScannedNetwork {
	return &surveyv1.ScannedNetwork{
		Ssid:            n.SSID,
		Bssid:           n.BSSID,
		SignalDbm:       int32Of(n.Signal),
		Channel:         int32Of(n.Channel),
		FrequencyMhz:    int32Of(n.Frequency),
		Security:        n.Security,
		ChannelWidthMhz: int32Of(n.ChannelWidth),
		NoiseFloorDbm:   int32Of(n.NoiseFloor),
		SnrDb:           int32Of(n.SNR),
		HtMode:          n.HTMode,
		IsDfs:           n.IsDFS,
		Associated:      n.Associated,
		// Left nil when the AP advertised no BSS Load element: 0% is a real
		// reading for an idle channel, so a client has to be able to tell the
		// two apart.
		ChannelUtilizationPercent: utilizationOf(n.ChannelUtilization),
	}
}

// utilizationOf narrows an optional utilisation reading for the wire.
func utilizationOf(percent *int) *int32 {
	if percent == nil {
		return nil
	}
	narrowed := int32Of(*percent)
	return &narrowed
}

// ListSamples returns the points stored on a survey's active floor, in
// capture order.
func (h *SurveyServiceHandler) ListSamples(
	_ context.Context,
	req *connect.Request[surveyv1.ListSamplesRequest],
) (*connect.Response[surveyv1.ListSamplesResponse], error) {
	id := req.Msg.GetSurveyId()
	if id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("survey_id is required"))
	}

	svy, err := h.manager.GetSurvey(id)
	if err != nil {
		return nil, notFoundOrInternal(err)
	}

	reply := &surveyv1.ListSamplesResponse{}
	floor := svy.GetActiveFloor()
	if floor == nil {
		return connect.NewResponse(reply), nil
	}

	reply.Samples = make([]*surveyv1.SurveySample, 0, len(floor.Samples))
	for _, sp := range floor.Samples {
		reply.Samples = append(reply.Samples, surveySampleOf(sp))
	}
	return connect.NewResponse(reply), nil
}

// surveySampleOf reduces a stored point to its drawable facts.
//
// The strongest signal is found by scanning rather than read from Networks[0]:
// a passive sample is sorted strongest-first when it is captured, but a point
// loaded from the store has been through JSON, and the wire type should not
// depend on that ordering having survived.
func surveySampleOf(sp *survey.SamplePoint) *surveyv1.SurveySample {
	out := &surveyv1.SurveySample{
		X:          int32Of(sp.X),
		Y:          int32Of(sp.Y),
		CapturedAt: timestamppb.New(sp.Timestamp),
	}

	var passive *survey.PassiveSample
	switch data := sp.SampleData.(type) {
	case *survey.PassiveSample:
		passive = data
	case survey.PassiveSample:
		passive = &data
	default:
		return out
	}

	out.NetworkCount = int32Of(len(passive.Networks))
	for i, n := range passive.Networks {
		if i == 0 || n.Signal > int(out.GetStrongestDbm()) {
			out.StrongestDbm = proto.Int32(int32Of(n.Signal))
		}
	}
	return out
}
