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
)
