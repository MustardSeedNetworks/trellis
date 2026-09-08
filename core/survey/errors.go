// SPDX-License-Identifier: BUSL-1.1

package survey

import "errors"

// Sentinel errors the manager returns so callers can classify failures with
// errors.Is rather than matching on message text. The API layer maps
// ErrSurveyNotFound / ErrFloorNotFound onto connect.CodeNotFound.
var (
	// ErrSurveyNotFound is returned when no survey exists for the given ID.
	ErrSurveyNotFound = errors.New("survey not found")
	// ErrFloorNotFound is returned when no floor exists for the given ID.
	ErrFloorNotFound = errors.New("floor not found")
	// ErrArchiveEntryTooLarge is returned when an entry in an imported archive
	// inflates past maxArchiveEntryBytes. The transport bounds the compressed
	// message; this bounds what one member is allowed to become.
	ErrArchiveEntryTooLarge = errors.New("archive entry exceeds the inflated size limit")
	// ErrPlanWouldStrandSamples is returned when a floor plan of different
	// dimensions is uploaded over a floor that already holds measurements. A
	// sample's position is a pixel coordinate on the plan it was taken
	// against, so a new pixel space silently moves every one of them.
	ErrPlanWouldStrandSamples = errors.New("replacing the floor plan would strand the measurements taken on it")
	// ErrLastFloor is returned when the only floor of a survey is deleted. A
	// survey collects onto a floor, so one without any has nowhere to put the
	// next reading.
	ErrLastFloor = errors.New("cannot delete the last floor of a survey")
	// ErrFloorNameEmpty is returned when a floor is created or renamed with a
	// blank name. Floors are listed and picked by name, and a blank one is a
	// row nothing can identify.
	ErrFloorNameEmpty = errors.New("floor name is required")
)
