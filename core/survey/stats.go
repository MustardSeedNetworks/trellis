package survey

import (
	"math"
	"sort"

	"github.com/MustardSeedNetworks/trellis/core/wifi"
)

// SurveyStats holds calculated statistics for a survey.
//
//nolint:revive // Renaming to Stats would be a breaking API change
type SurveyStats struct {
	TotalSamples     int
	AvgRSSI          int
	MinRSSI          int
	MaxRSSI          int
	CoverageScore    float64
	WeakAreas        int
	DeadZones        int
	ExcellentPercent float64
	GoodPercent      float64
	FairPercent      float64
	PoorPercent      float64
	DeadPercent      float64
}

// measuredPoints drops the points where a measurement was attempted and
// produced nothing. They are stored so the map can show where a survey tried
// and failed, but they are not measurements: counted as samples they divide
// every percentage by a number nobody measured.
func measuredPoints(points []*SamplePoint) []*SamplePoint {
	out := points
	for i, p := range points {
		if p.Failed == nil {
			continue
		}
		// Copy only once a failed point is actually seen — a walk with no
		// failures, which is nearly all of them, keeps its own slice.
		out = make([]*SamplePoint, 0, len(points))
		out = append(out, points[:i]...)
		for _, rest := range points[i:] {
			if rest.Failed == nil {
				out = append(out, rest)
			}
		}
		break
	}
	return out
}

func calculateSurveyStats(points []*SamplePoint) SurveyStats {
	samples := measuredPoints(points)
	stats := SurveyStats{
		TotalSamples: len(samples),
		MinRSSI:      0,
		MaxRSSI:      minRSSIValue,
	}

	if len(samples) == 0 {
		return stats
	}

	var sumRSSI int
	var excellent, good, fair, poor, dead int

	for _, sample := range samples {
		ps := getPassiveSampleFromPoint(sample)
		if ps == nil || len(ps.Networks) == 0 {
			continue
		}

		// Use best RSSI from networks in the passive sample
		bestRSSI := minRSSIValue
		for _, net := range ps.Networks {
			if net.Signal > bestRSSI {
				bestRSSI = net.Signal
			}
		}

		sumRSSI += bestRSSI
		if bestRSSI > stats.MaxRSSI {
			stats.MaxRSSI = bestRSSI
		}
		if bestRSSI < stats.MinRSSI || stats.MinRSSI == 0 {
			stats.MinRSSI = bestRSSI
		}

		switch {
		case bestRSSI > ExcellentSignal:
			excellent++
		case bestRSSI > GoodSignal:
			good++
		case bestRSSI > FairSignal:
			fair++
		case bestRSSI > PoorSignal:
			poor++
		default:
			dead++
		}
	}

	if len(samples) > 0 {
		stats.AvgRSSI = sumRSSI / len(samples)
		total := float64(len(samples))
		stats.ExcellentPercent = float64(excellent) / total * percentMultiplier
		stats.GoodPercent = float64(good) / total * percentMultiplier
		stats.FairPercent = float64(fair) / total * percentMultiplier
		stats.PoorPercent = float64(poor) / total * percentMultiplier
		stats.DeadPercent = float64(dead) / total * percentMultiplier
		stats.CoverageScore = stats.ExcellentPercent + stats.GoodPercent + (stats.FairPercent * fairCoverageWeight)
		stats.WeakAreas = poor
		stats.DeadZones = dead
	}

	return stats
}

func calculateFloorStats(samples []*SamplePoint) SurveyStats {
	return calculateSurveyStats(samples)
}

// ChannelInfo represents Wi-Fi channel usage information.
type ChannelInfo struct {
	Channel int
	Count   int
}

func getChannelUsage(samples []*SamplePoint) []ChannelInfo {
	channelMap := make(map[int]int)

	for _, sample := range samples {
		ps := getPassiveSampleFromPoint(sample)
		if ps == nil {
			continue
		}
		for _, net := range ps.Networks {
			if net.Channel > 0 {
				channelMap[net.Channel]++
			}
		}
	}

	channels := make([]ChannelInfo, 0, len(channelMap))
	for ch, count := range channelMap {
		channels = append(channels, ChannelInfo{Channel: ch, Count: count})
	}

	sort.Slice(channels, func(i, j int) bool {
		return channels[i].Count > channels[j].Count
	})

	// Limit to top channels
	if len(channels) > topChannelsLimit {
		channels = channels[:topChannelsLimit]
	}

	return channels
}

// RecommendationPriority indicates the urgency of a recommendation.
type RecommendationPriority int

// Recommendation priority levels.
const (
	PriorityLow RecommendationPriority = iota
	PriorityMedium
	PriorityHigh
)

// Recommendation represents an improvement suggestion.
type Recommendation struct {
	Text     string
	Priority RecommendationPriority
}

// APSighting is one access point's record across a floor's walk: what it
// announced and how it was heard.
type APSighting struct {
	BSSID      string
	SSID       string
	Band       string
	Channel    int
	Width      int // MHz; 0 when the scan did not report it
	Samples    int // points at which this BSSID was heard
	BestRSSI   int
	MedianRSSI int
}

// APInventory lists every BSSID heard across samples, most-heard first.
//
// A BSSID counts once per point, at its strongest reading there, so Samples is
// "how much of the walk heard it" rather than how many scan entries named it.
// SSID, band, channel and width come from the strongest reading overall: an AP
// that moved channel mid-walk is reported where it was loudest.
func APInventory(samples []*SamplePoint) []APSighting {
	type tally struct {
		strongest *wifi.ScannedNetwork
		readings  []int
	}
	byBSSID := make(map[string]*tally)

	for _, sample := range samples {
		ps := getPassiveSampleFromPoint(sample)
		if ps == nil {
			continue
		}
		best := make(map[string]*wifi.ScannedNetwork, len(ps.Networks))
		for _, net := range ps.Networks {
			if net.BSSID == "" {
				continue
			}
			if cur, ok := best[net.BSSID]; !ok || net.Signal > cur.Signal {
				best[net.BSSID] = net
			}
		}
		for bssid, net := range best {
			t, ok := byBSSID[bssid]
			if !ok {
				t = &tally{strongest: net}
				byBSSID[bssid] = t
			} else if net.Signal > t.strongest.Signal {
				t.strongest = net
			}
			t.readings = append(t.readings, net.Signal)
		}
	}

	aps := make([]APSighting, 0, len(byBSSID))
	for bssid, t := range byBSSID {
		sort.Ints(t.readings)
		n := t.strongest
		aps = append(aps, APSighting{
			BSSID:      bssid,
			SSID:       n.SSID,
			Band:       bandLabel(n.Frequency, n.Channel),
			Channel:    n.Channel,
			Width:      n.ChannelWidth,
			Samples:    len(t.readings),
			BestRSSI:   n.Signal,
			MedianRSSI: medianInt(t.readings),
		})
	}
	sort.Slice(aps, func(i, j int) bool {
		if aps[i].Samples != aps[j].Samples {
			return aps[i].Samples > aps[j].Samples
		}
		return aps[i].BSSID < aps[j].BSSID
	})
	return aps
}

// medianInt is the median of sorted, rounded half away from zero when the
// count is even.
func medianInt(sorted []int) int {
	mid := len(sorted) / 2
	if len(sorted)%2 == 1 {
		return sorted[mid]
	}
	return int(math.Round(float64(sorted[mid-1]+sorted[mid]) / 2))
}
