// SPDX-License-Identifier: BUSL-1.1

package survey

import (
	"fmt"
	"strings"

	"github.com/MustardSeedNetworks/trellis/core/wifi"
)

// ForBSSID is the floor as one access point covers it: every point that heard
// bssid, holding only that AP's reading there, so the heatmap and the dead-zone
// analysis — which read Networks[0] as the AP serving a point — answer about
// this AP instead of the strongest one.
//
// A point that did not hear the AP is left out rather than given a reading: the
// walk recorded no level for it, and any number put in its place would be
// invented. Active and throughput points are left out too; they measured the
// connection, not one AP's signal.
//
// The view is a copy; the floor and its samples are not modified.
func (f *Floor) ForBSSID(bssid string) (*Floor, error) {
	view := *f
	view.Samples = make([]*SamplePoint, 0, len(f.Samples))
	for _, sp := range f.Samples {
		ps := getPassiveSampleFromPoint(sp)
		if ps == nil {
			continue
		}
		var heard *wifi.ScannedNetwork
		for _, net := range ps.Networks {
			if strings.EqualFold(net.BSSID, bssid) && (heard == nil || net.Signal > heard.Signal) {
				heard = net
			}
		}
		if heard == nil {
			continue
		}
		only := &PassiveSample{Networks: []*wifi.ScannedNetwork{heard}}
		only.CalculateAggregations()
		point := *sp
		point.SampleData = only
		view.Samples = append(view.Samples, &point)
	}
	if len(view.Samples) == 0 {
		return nil, fmt.Errorf("%w: %s", ErrBSSIDNotHeard, bssid)
	}
	return &view, nil
}
