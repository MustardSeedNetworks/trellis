// SPDX-License-Identifier: BUSL-1.1

package api_test

import (
	"context"
	"testing"

	"connectrpc.com/connect/v2"
	"google.golang.org/protobuf/proto"

	"github.com/MustardSeedNetworks/trellis/core/survey"
	surveyv1 "github.com/MustardSeedNetworks/trellis/gen/trellis/survey/v1"
	"github.com/MustardSeedNetworks/trellis/internal/api"
)

// plannedFloor returns a handler over a survey whose one floor has an 800x600
// plan and no placements, and the directory its store lives in.
func plannedFloor(t *testing.T) (handler *api.SurveyServiceHandler, dir, surveyID, floorID string) {
	t.Helper()

	dir = t.TempDir()
	mgr := mustManager(t, dir, nil, nil, nil, nil)
	svy, err := mgr.CreateSurvey("Plan only", "", "en0", survey.TypePassive)
	if err != nil {
		t.Fatalf("CreateSurvey: %v", err)
	}
	floorID = svy.Floors[0].ID
	if err := mgr.SetFloorPlan(svy.ID, floorID, planPNG(t, 800, 600)); err != nil {
		t.Fatalf("SetFloorPlan: %v", err)
	}
	return api.NewSurveyServiceHandler(mgr), dir, svy.ID, floorID
}

func createAP(
	t *testing.T, h *api.SurveyServiceHandler, surveyID, floorID string, edit *surveyv1.APPlacementEdit,
) *surveyv1.APPlacement {
	t.Helper()
	resp, err := h.CreateAPPlacement(context.Background(), connect.NewRequest(&surveyv1.CreateAPPlacementRequest{
		SurveyId: surveyID, FloorId: floorID, Placement: edit,
	}))
	if err != nil {
		t.Fatalf("CreateAPPlacement: %v", err)
	}
	return resp.Msg.GetPlacement()
}

func listAPs(t *testing.T, h *api.SurveyServiceHandler, surveyID, floorID string) []*surveyv1.APPlacement {
	t.Helper()
	resp, err := h.ListAPPlacements(context.Background(), connect.NewRequest(&surveyv1.ListAPPlacementsRequest{
		SurveyId: surveyID, FloorId: floorID,
	}))
	if err != nil {
		t.Fatalf("ListAPPlacements: %v", err)
	}
	return resp.Msg.GetPlacements()
}

func TestCreateAPPlacementPlacesAnAPOnTheFloor(t *testing.T) {
	t.Parallel()
	h, _, surveyID, floorID := plannedFloor(t)

	created := createAP(t, h, surveyID, floorID, &surveyv1.APPlacementEdit{
		X: 120, Y: 340, Label: "AP-lobby", TxPowerDbm: proto.Float64(14), Band: "5 GHz",
	})
	if created.GetId() == "" || created.GetImported() {
		t.Fatalf("created = %v, want a new ID and imported=false", created)
	}

	listed := listAPs(t, h, surveyID, floorID)
	if len(listed) != 1 || !proto.Equal(listed[0], created) {
		t.Fatalf("listed = %v, want exactly %v", listed, created)
	}
}

// A power the operator did not state stays unstated rather than becoming
// 0 dBm, which the prediction would draw as an AP 10 dB quieter than assumed.
func TestCreateAPPlacementWithoutAPowerLeavesItUnstated(t *testing.T) {
	t.Parallel()
	h, _, surveyID, floorID := plannedFloor(t)

	created := createAP(t, h, surveyID, floorID, &surveyv1.APPlacementEdit{X: 1, Y: 1})
	if created.TxPowerDbm != nil {
		t.Fatalf("tx_power_dbm = %v, want absent", created.GetTxPowerDbm())
	}
}

func TestUpdateAPPlacementMovesItAndChangesItsSettings(t *testing.T) {
	t.Parallel()
	h, _, surveyID, floorID := plannedFloor(t)
	created := createAP(t, h, surveyID, floorID, &surveyv1.APPlacementEdit{
		X: 10, Y: 10, Label: "AP-1", TxPowerDbm: proto.Float64(20), Band: "2.4 GHz",
	})

	resp, err := h.UpdateAPPlacement(context.Background(), connect.NewRequest(&surveyv1.UpdateAPPlacementRequest{
		SurveyId: surveyID, FloorId: floorID, PlacementId: created.GetId(),
		Placement: &surveyv1.APPlacementEdit{X: 799, Y: 599, Label: "AP-1 moved", Band: "6 GHz"},
	}))
	if err != nil {
		t.Fatalf("UpdateAPPlacement: %v", err)
	}
	want := &surveyv1.APPlacement{Id: created.GetId(), X: 799, Y: 599, Label: "AP-1 moved", Band: "6 GHz"}
	if got := resp.Msg.GetPlacement(); !proto.Equal(got, want) {
		t.Fatalf("updated = %v, want %v", got, want)
	}
	if listed := listAPs(t, h, surveyID, floorID); len(listed) != 1 || !proto.Equal(listed[0], want) {
		t.Fatalf("listed = %v, want exactly %v", listed, want)
	}
}

func TestDeleteAPPlacementReturnsTheRemainingOnes(t *testing.T) {
	t.Parallel()
	h, _, surveyID, floorID := plannedFloor(t)
	gone := createAP(t, h, surveyID, floorID, &surveyv1.APPlacementEdit{X: 10, Y: 10, Label: "gone"})
	kept := createAP(t, h, surveyID, floorID, &surveyv1.APPlacementEdit{X: 20, Y: 20, Label: "kept"})

	resp, err := h.DeleteAPPlacement(context.Background(), connect.NewRequest(&surveyv1.DeleteAPPlacementRequest{
		SurveyId: surveyID, FloorId: floorID, PlacementId: gone.GetId(),
	}))
	if err != nil {
		t.Fatalf("DeleteAPPlacement: %v", err)
	}
	if remaining := resp.Msg.GetPlacements(); len(remaining) != 1 || !proto.Equal(remaining[0], kept) {
		t.Fatalf("remaining = %v, want exactly %v", remaining, kept)
	}

	_, err = h.DeleteAPPlacement(context.Background(), connect.NewRequest(&surveyv1.DeleteAPPlacementRequest{
		SurveyId: surveyID, FloorId: floorID, PlacementId: gone.GetId(),
	}))
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("deleting it twice = %v, want NotFound", err)
	}
}

// A placement written through the API is in the store, not only in memory.
func TestAPPlacementWrittenThroughTheAPISurvivesAStoreReopen(t *testing.T) {
	t.Parallel()
	h, dir, surveyID, floorID := plannedFloor(t)
	created := createAP(t, h, surveyID, floorID, &surveyv1.APPlacementEdit{
		X: 400, Y: 300, Label: "AP-centre", TxPowerDbm: proto.Float64(17.5), Band: "5 GHz",
	})

	// The first manager's store closes at cleanup; a second one opening the
	// same directory is what a daemon restart reads.
	reopened := mustManager(t, dir, nil, nil, nil, nil)
	if err := reopened.LoadSurveys(); err != nil {
		t.Fatalf("LoadSurveys: %v", err)
	}
	listed := listAPs(t, api.NewSurveyServiceHandler(reopened), surveyID, floorID)
	if len(listed) != 1 || !proto.Equal(listed[0], created) {
		t.Fatalf("after reopen = %v, want exactly %v", listed, created)
	}
}

func TestAPPlacementWritesAreRefused(t *testing.T) {
	t.Parallel()
	h, _, surveyID, floorID := plannedFloor(t)
	existing := createAP(t, h, surveyID, floorID, &surveyv1.APPlacementEdit{X: 10, Y: 10})

	unplanned := mustManager(t, t.TempDir(), nil, nil, nil, nil)
	bare, err := unplanned.CreateSurvey("No plan", "", "en0", survey.TypePassive)
	if err != nil {
		t.Fatalf("CreateSurvey: %v", err)
	}
	bareHandler := api.NewSurveyServiceHandler(unplanned)

	for _, tc := range []struct {
		name string
		call func() error
		want connect.Code
	}{
		{"off the plan's right edge", func() error {
			return createErr(h, surveyID, floorID, &surveyv1.APPlacementEdit{X: 800, Y: 10})
		}, connect.CodeInvalidArgument},
		{"negative position", func() error {
			return createErr(h, surveyID, floorID, &surveyv1.APPlacementEdit{X: 10, Y: -1})
		}, connect.CodeInvalidArgument},
		{"unknown band", func() error {
			return createErr(h, surveyID, floorID, &surveyv1.APPlacementEdit{X: 1, Y: 1, Band: "60 GHz"})
		}, connect.CodeInvalidArgument},
		{"impossible power", func() error {
			return createErr(h, surveyID, floorID, &surveyv1.APPlacementEdit{X: 1, Y: 1, TxPowerDbm: proto.Float64(100)})
		}, connect.CodeInvalidArgument},
		{"floor with no plan", func() error {
			return createErr(bareHandler, bare.ID, bare.Floors[0].ID, &surveyv1.APPlacementEdit{X: 1, Y: 1})
		}, connect.CodeFailedPrecondition},
		{"unknown floor", func() error {
			return createErr(h, surveyID, "no-such-floor", &surveyv1.APPlacementEdit{X: 1, Y: 1})
		}, connect.CodeNotFound},
		{"move off the plan", func() error {
			_, err := h.UpdateAPPlacement(context.Background(), connect.NewRequest(&surveyv1.UpdateAPPlacementRequest{
				SurveyId: surveyID, FloorId: floorID, PlacementId: existing.GetId(),
				Placement: &surveyv1.APPlacementEdit{X: 10, Y: 600},
			}))
			return err
		}, connect.CodeInvalidArgument},
		{"update an unknown placement", func() error {
			_, err := h.UpdateAPPlacement(context.Background(), connect.NewRequest(&surveyv1.UpdateAPPlacementRequest{
				SurveyId: surveyID, FloorId: floorID, PlacementId: "no-such-ap",
				Placement: &surveyv1.APPlacementEdit{X: 1, Y: 1},
			}))
			return err
		}, connect.CodeNotFound},
		{"update without a placement ID", func() error {
			_, err := h.UpdateAPPlacement(context.Background(), connect.NewRequest(&surveyv1.UpdateAPPlacementRequest{
				SurveyId: surveyID, FloorId: floorID, Placement: &surveyv1.APPlacementEdit{X: 1, Y: 1},
			}))
			return err
		}, connect.CodeInvalidArgument},
	} {
		if got := connect.CodeOf(tc.call()); got != tc.want {
			t.Errorf("%s = %v, want %v", tc.name, got, tc.want)
		}
	}

	// A refused write changes nothing.
	if listed := listAPs(t, h, surveyID, floorID); len(listed) != 1 || !proto.Equal(listed[0], existing) {
		t.Errorf("after the refusals = %v, want exactly %v", listed, existing)
	}
}

func createErr(h *api.SurveyServiceHandler, surveyID, floorID string, edit *surveyv1.APPlacementEdit) error {
	_, err := h.CreateAPPlacement(context.Background(), connect.NewRequest(&surveyv1.CreateAPPlacementRequest{
		SurveyId: surveyID, FloorId: floorID, Placement: edit,
	}))
	return err
}
