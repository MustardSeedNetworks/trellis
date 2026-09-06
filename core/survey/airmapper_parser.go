package survey

// This file implements AirMapper .amp file parsing.

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

// Unit conversion and default constants for AirMapper parsing.
const (
	// feetToMeters is the conversion factor from feet to meters (1 foot = 0.3048 meters).
	feetToMeters = 0.3048

	// defaultPropagationMeters is the default signal propagation radius in meters when not specified.
	defaultPropagationMeters = 10
)

// AirMapperFile represents a parsed .amp file from NetAlly AirMapper.
type AirMapperFile struct {
	Serial            *SerialMetadata `json:"serial"`
	FloorPlan         []byte          `json:"floorPlanData"` // Raw JPEG/PNG data
	FloorPlanFilename string          `json:"floorPlanFilename"`
	// SurveyResult is the raw measurement member, decoded by
	// ParseSurveyResult. Nil when the archive carries none.
	SurveyResult []byte `json:"-"`
}

// SerialMetadata contains metadata from the .serial JSON file in an AirMapper archive.
type SerialMetadata struct {
	FileName          string         `json:"fileName"`
	FloorPlanScalePpf float64        `json:"floorPlanScalePpf"` // pixels per foot
	Propagation       float64        `json:"propagation"`
	PropagationUnit   string         `json:"propagationUnit"`
	SurveyPointCount  int            `json:"surveyPointCount"`
	SurveyItemsCount  int            `json:"surveyItemsCount"`
	Locations         *LocationsData `json:"locations,omitempty"`
	InsitesLimits     []InsitesLimit `json:"insitesLimits,omitempty"`
	Views             []ViewConfig   `json:"views,omitempty"`
}

// LocationsData contains AP and client location data.
type LocationsData struct {
	APLocations     []APLocationData     `json:"aps,omitempty"`
	ClientLocations []ClientLocationData `json:"clients,omitempty"`
}

// APLocationData represents a placed AP location from AirMapper.
type APLocationData struct {
	BSSID   string  `json:"bssid"`
	X       float64 `json:"x"`
	Y       float64 `json:"y"`
	Label   string  `json:"label,omitempty"`
	Channel int     `json:"channel,omitempty"`
	Band    string  `json:"band,omitempty"`
}

// ClientLocationData represents a client location from AirMapper.
type ClientLocationData struct {
	MAC   string  `json:"mac"`
	X     float64 `json:"x"`
	Y     float64 `json:"y"`
	Label string  `json:"label,omitempty"`
}

// InsitesLimit represents a pass/fail criterion from AirMapper.
type InsitesLimit struct {
	Option  string  `json:"option"`
	Name    string  `json:"name,omitempty"`
	Limit   float64 `json:"limit"`
	Suffix  string  `json:"suffix"`
	Enabled bool    `json:"enabled"`
	Mode    string  `json:"mode"`         // "passive", "active"
	AP      int     `json:"ap,omitempty"` // AP index for nth-signal tests
}

// ViewConfig represents a view configuration from AirMapper.
type ViewConfig struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Enabled bool   `json:"enabled"`
}

// AirMapperCalibration contains scale and propagation settings.
type AirMapperCalibration struct {
	ScaleM       float64 `json:"scaleM"`       // meters per pixel
	PropagationM float64 `json:"propagationM"` // signal propagation radius in meters
}

// AirMapperImportResult is the result of parsing an AirMapper file.
type AirMapperImportResult struct {
	FloorPlanImage    string               `json:"floorPlanImage"` // Base64 data URL
	FloorPlanFilename string               `json:"floorPlanFilename"`
	Calibration       AirMapperCalibration `json:"calibration"`
	APLocations       []APLocationData     `json:"apLocations,omitempty"`
	ClientLocations   []ClientLocationData `json:"clientLocations,omitempty"`
	PassFailCriteria  []InsitesLimit       `json:"passFailCriteria,omitempty"`
	SurveyPointCount  int                  `json:"surveyPointCount"`
	SurveyItemsCount  int                  `json:"surveyItemsCount"`
	Warnings          []string             `json:"warnings,omitempty"`
}

// ParseAirMapperFile parses an AirMapper .amp archive file.
func ParseAirMapperFile(data []byte) (*AirMapperFile, error) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("invalid zip archive: %w", err)
	}

	result := &AirMapperFile{}
	var floorPlanFound bool

	for _, file := range reader.File {
		name := file.Name
		ext := strings.ToLower(filepath.Ext(name))

		switch {
		case strings.HasSuffix(name, ".serial"):
			serial, parseErr := parseSerialFile(file)
			if parseErr != nil {
				return nil, fmt.Errorf("failed to parse .serial file: %w", parseErr)
			}
			result.Serial = serial

		case ext == ".jpg" || ext == ".jpeg" || ext == ".png":
			imgData, readErr := readZipFile(file)
			if readErr != nil {
				return nil, fmt.Errorf("failed to read floor plan image: %w", readErr)
			}
			result.FloorPlan = imgData
			result.FloorPlanFilename = filepath.Base(name)
			floorPlanFound = true

		}

		if strings.HasSuffix(strings.ToLower(name), ".surveyresult") {
			payload, readErr := readZipFile(file)
			if readErr != nil {
				return nil, fmt.Errorf("read .SurveyResult: %w", readErr)
			}
			result.SurveyResult = payload
		}
	}

	// .serial is optional. It carries the extras — AP and client placements,
	// the pixels-per-foot calibration, the pass/fail criteria — and an archive
	// exported from Link-Live, which is how most surveys leave an AirCheck or
	// an EtherScope, has none. The survey itself is in .SurveyResult and the
	// plan is the image, both of which those archives do carry; refusing them
	// for a missing extra turned away eight of the 58 archives in the reference
	// corpus.

	if !floorPlanFound {
		return nil, errors.New("no floor plan image found in archive")
	}

	return result, nil
}

// parseSerialFile parses the .serial JSON file from an AirMapper archive.
func parseSerialFile(file *zip.File) (*SerialMetadata, error) {
	data, err := readZipFile(file)
	if err != nil {
		return nil, err
	}

	var serial SerialMetadata
	if unmarshalErr := json.Unmarshal(data, &serial); unmarshalErr != nil {
		return nil, fmt.Errorf("invalid JSON in .serial file: %w", unmarshalErr)
	}

	return &serial, nil
}

// maxArchiveEntryBytes caps what a single archive member may inflate to. The
// daemon already caps the compressed request at 64 MiB; a member is allowed
// the same, which no real floor plan or .SurveyResult approaches, while a
// crafted archive that is small on the wire and huge inflated is stopped here
// rather than in the allocator.
const maxArchiveEntryBytes = 64 << 20

// readZipFile reads one member of the archive, refusing it once it inflates
// past maxArchiveEntryBytes. The header's own size claim is not consulted: a
// hostile archive can write whatever it likes there.
func readZipFile(file *zip.File) ([]byte, error) {
	rc, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer func() { _ = rc.Close() }()

	// One byte past the cap is read so overflow is observable: a reader limited
	// to exactly the cap returns a full buffer for both "fits" and "too big".
	data, err := io.ReadAll(io.LimitReader(rc, maxArchiveEntryBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxArchiveEntryBytes {
		return nil, fmt.Errorf("%w: %s inflates past %d bytes", ErrArchiveEntryTooLarge, file.Name, maxArchiveEntryBytes)
	}
	return data, nil
}

// ToImportResult converts an AirMapperFile to an AirMapperImportResult.
func (a *AirMapperFile) ToImportResult() (*AirMapperImportResult, error) {
	result := &AirMapperImportResult{
		Warnings: make([]string, 0),
	}

	// Convert floor plan to base64 data URL
	if len(a.FloorPlan) > 0 {
		// Detect image type from the file signature
		mimeType := "image/jpeg"
		if len(a.FloorPlan) > 8 && string(a.FloorPlan[:8]) == "\x89PNG\r\n\x1a\n" {
			mimeType = "image/png"
		}

		result.FloorPlanImage = fmt.Sprintf("data:%s;base64,%s",
			mimeType, base64.StdEncoding.EncodeToString(a.FloorPlan))
		result.FloorPlanFilename = a.FloorPlanFilename
	} else {
		result.Warnings = append(result.Warnings, "No floor plan image found")
	}

	if a.Serial == nil {
		// Everything below reads .serial. Without it the archive states no
		// scale, and the scale stays unknown rather than defaulted: a made-up
		// metres-per-pixel would put invented distances on every dead-zone
		// radius in the report. The two-point calibration is how an operator
		// supplies it.
		result.Warnings = append(result.Warnings,
			"This archive carries no calibration, placements or pass/fail criteria. "+
				"Calibrate the floor plan to measure distances.")
		return result, nil
	}

	// Convert scale from pixels per foot to meters per pixel
	if a.Serial.FloorPlanScalePpf > 0 {
		// ppf = pixels per foot
		// We need meters per pixel = 1 / (ppf * 3.28084) = feet_per_pixel * feetToMeters
		feetPerPixel := 1.0 / a.Serial.FloorPlanScalePpf
		result.Calibration.ScaleM = feetPerPixel * feetToMeters
	} else {
		// Unknown, not defaulted — see the nil-Serial branch above.
		result.Warnings = append(result.Warnings,
			"No scale calibration in this archive. Calibrate the floor plan to measure distances.")
	}

	// Convert propagation from feet to meters
	if a.Serial.Propagation > 0 {
		switch a.Serial.PropagationUnit {
		case "ft", "":
			result.Calibration.PropagationM = a.Serial.Propagation * feetToMeters
		case "m":
			result.Calibration.PropagationM = a.Serial.Propagation
		default:
			result.Calibration.PropagationM = a.Serial.Propagation * feetToMeters // Default to feet
			result.Warnings = append(
				result.Warnings,
				fmt.Sprintf(
					"Unknown propagation unit: %s, assuming feet",
					a.Serial.PropagationUnit,
				),
			)
		}
	} else {
		result.Calibration.PropagationM = defaultPropagationMeters
	}

	// Copy locations if available
	if a.Serial.Locations != nil {
		result.APLocations = a.Serial.Locations.APLocations
		result.ClientLocations = a.Serial.Locations.ClientLocations
	}

	// Copy pass/fail criteria
	result.PassFailCriteria = a.Serial.InsitesLimits

	// Copy counts
	result.SurveyPointCount = a.Serial.SurveyPointCount
	result.SurveyItemsCount = a.Serial.SurveyItemsCount

	return result, nil
}
