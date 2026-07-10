// SPDX-License-Identifier: BUSL-1.1

package wifi

import "time"

// Severity is the anomaly urgency. Lifted (data-only) from Seed's
// internal/anomaly.Severity: info < warning < error < critical.
type Severity string

// Anomaly severity levels, ordered least to most urgent.
const (
	SeverityInfo     Severity = "info"
	SeverityWarning  Severity = "warning"
	SeverityError    Severity = "error"
	SeverityCritical Severity = "critical"
)

// Category encodes the problem domain a detection belongs to. Lifted
// (data-only) from Seed's internal/anomaly.Category.
type Category string

// Anomaly categories.
const (
	CategorySecurity      Category = "security"
	CategoryRF            Category = "rf"
	CategoryRoaming       Category = "roaming"
	CategoryCapacity      Category = "capacity"
	CategoryStandards     Category = "standards"
	CategoryAuthorization Category = "authorization"
	CategoryNetHealth     Category = "nethealth"
)

// FollowUpKind selects how a follow-up narrows an ambiguous detection.
// Lifted (data-only) from Seed's internal/anomaly.FollowUpKind.
type FollowUpKind string

// Follow-up kinds.
const (
	FollowUpAuto   FollowUpKind = "auto"
	FollowUpPrompt FollowUpKind = "prompt"
)

// FollowUp narrows the diagnosis for an anomaly type. Lifted (data-only) from
// Seed's internal/anomaly.FollowUp.
type FollowUp struct {
	Kind       FollowUpKind `json:"kind"`
	Label      string       `json:"label"`
	Action     string       `json:"action"`
	Capability string       `json:"capability,omitempty"`
}

// SubjectKind classifies what an anomaly is about. Lifted (data-only) from
// Seed's internal/anomaly.SubjectKind.
type SubjectKind string

// Subject kinds.
const (
	SubjectSSID      SubjectKind = "ssid"
	SubjectBSSID     SubjectKind = "bssid"
	SubjectClient    SubjectKind = "client"
	SubjectChannel   SubjectKind = "channel"
	SubjectDevice    SubjectKind = "device"
	SubjectInterface SubjectKind = "interface"
	SubjectProbe     SubjectKind = "probe"
)

// SubjectRef points at the entity an anomaly concerns. Lifted (data-only)
// from Seed's internal/anomaly.SubjectRef.
type SubjectRef struct {
	Kind SubjectKind `json:"kind"`
	ID   string      `json:"id"`
}

// Anomaly is the projected, JSON-serializable view of a detected anomaly: the
// catalog copy (title/description/recommendation/standards) merged with the
// instance evidence and lifecycle (firstSeen/lastSeen/count). Lifted
// (data-only) from Seed's internal/anomaly.Anomaly — Wire tags match Seed's
// so downstream tooling (reports, fixtures) round-trips unchanged.
//
// The rule engine that produces these values (internal/anomaly +
// internal/wifi/anomaly in Seed, ~3700 LOC of catalog/detector/engine) has
// not been ported to Trellis yet; see core/survey.AnomalyDetector.
type Anomaly struct {
	DefKey         string            `json:"defKey"`
	Category       Category          `json:"category"`
	Severity       Severity          `json:"severity"`
	Subject        SubjectRef        `json:"subject"`
	Title          string            `json:"title"`
	Description    string            `json:"description"`
	Impact         string            `json:"impact,omitempty"`
	Recommendation string            `json:"recommendation"`
	Standards      []string          `json:"standards,omitempty"`
	Evidence       map[string]string `json:"evidence,omitempty"`
	FollowUps      []FollowUp        `json:"followUps,omitempty"`
	FirstSeen      time.Time         `json:"firstSeen"`
	LastSeen       time.Time         `json:"lastSeen"`
	Count          int               `json:"count"`
}

// Wi-Fi anomaly definition keys. Lifted from Seed's
// internal/wifi/anomaly.Def* constants (identifiers only — the catalog
// copy/detection rules that produce these are not ported yet).
const (
	DefOpenNetwork             = "wifi-open-network"
	DefWEPInUse                = "wifi-wep-in-use"
	DefWPSEnabled              = "wifi-wps-enabled"
	DefPMFNotRequired          = "wifi-pmf-not-required"
	DefSecurityMismatch        = "wifi-security-mismatch"
	DefEvilTwin                = "wifi-evil-twin"
	DefCoChannelContention     = "wifi-co-channel-contention"
	DefAdjacentChannelOverlap  = "wifi-adjacent-channel-overlap"
	DefHiddenSSID              = "wifi-hidden-ssid"
	DefCountryConflict         = "wifi-country-conflict"
	DefStandardMismatch        = "wifi-standard-mismatch"
	DefWPA3TransitionDowngrade = "wifi-wpa3-transition-downgrade"
	DefDefaultSSIDName         = "wifi-default-ssid-name"
	DefSSIDSprawl              = "wifi-ssid-sprawl"
	DefInconsistentRoaming     = "wifi-inconsistent-roaming"
	DefRegulatoryViolation     = "wifi-regulatory-violation"
	DefBSSLoadSaturation       = "wifi-bss-load-saturation"
	DefWideChannel24GHz        = "wifi-wide-channel-2ghz"
	DefChannelWidthMismatch    = "wifi-channel-width-mismatch"
	DefDeauthFlood             = "wifi-deauth-flood"
	DefRogueAPOnLAN            = "wifi-rogue-ap-on-lan"
)
