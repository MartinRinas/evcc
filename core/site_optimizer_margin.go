package core

import (
	"slices"

	"github.com/evcc-io/evcc/core/types"
	optimizer "github.com/evcc-io/optimizer/client"
	"github.com/samber/lo"
)

// Solar forecast margin: the optimizer plans with a solar forecast below the expected
// one when buying insurance energy in cheap slots is worth more than it risks.
// See core/optimizer.md for the reasoning.

const (
	// marginSlots is the period the cost trade-off is evaluated over (24h of 15min slots)
	marginSlots = 96

	// marginMaxQuantile caps the planning percentile at P10: 30 daily ratios cannot
	// tell rarer days apart
	marginMaxQuantile = 0.9
)

// marginQuantile returns the share of days the planned solar energy should not
// fall short of. It balances the cost per Wh of planning too little, importing
// later instead of storing at the cheapest price, against planning too much:
// only round trip losses while the energy carries over, or the displaced solar
// surplus when solar fills the battery anyway. Prices are per Wh, rt is the
// battery round trip efficiency. 0 means no trade-off worth a margin.
func marginQuantile(pn, pe []float32, rt float64, solarFills bool) float64 {
	n := min(len(pn), len(pe), marginSlots)
	if n == 0 || rt <= 0 {
		return 0
	}

	prices := slices.Sorted(slices.Values(pn[:n]))
	cheap := float64(prices[0])

	// a shortfall is imported in the expensive half of the day, the slots the battery covers
	later := float64(lo.Mean(prices[n/2:]))

	under := max(0, later-cheap/rt)

	over := cheap * (1/rt - 1)
	if solarFills {
		over = cheap/rt - float64(lo.Mean(pe[:n]))
	}
	over = max(0, over)

	if under+over == 0 {
		return 0
	}
	return under / (under + over)
}

// solarMarginFactor returns the factor applied to the expected solar forecast:
// the (1-q) percentile of the daily produced/forecasted ratios relative to their
// median. Relative to the median it only covers the spread of the forecast error,
// bias stays with the solar adjustment. A q at or below 0.5 or too little history
// means no margin, so the margin never plans more solar than today.
func solarMarginFactor(ratios []float64, q float64) float64 {
	if q <= 0.5 {
		return 1
	}

	median, ok := percentileOf(ratios, 0.5, solarScaleMinSamples)
	if !ok || median <= 0 {
		return 1
	}

	low, _ := percentileOf(ratios, 1-min(q, marginMaxQuantile), solarScaleMinSamples)

	return min(1, low/median)
}

// applySolarMargin scales the solar forecast by factor, fading in over the
// first slots like the measured value blending: the current slot is close to
// what is measured, the forecast error grows with the distance.
func applySolarMargin(ft []float32, factor float64, decaySlots int) {
	for i := range ft {
		w := min(1, float64(i)/float64(decaySlots))
		ft[i] *= float32(1 - w*(1-factor))
	}
}

// solarFillsBattery reports whether the solar surplus over home demand in the
// evaluated period exceeds the usable home battery capacity (Wh)
func solarFillsBattery(ft, gt []float32, capacity float64) bool {
	var surplus float64
	for i := range min(len(ft), len(gt), marginSlots) {
		surplus += max(0, float64(ft[i]-gt[i]))
	}
	return surplus >= capacity
}

// usableBatteryCapacity returns the home battery capacity in Wh between the soc
// limits, counting the batteries the optimizer models
func usableBatteryCapacity(battery []types.Measurement) float64 {
	var res float64

	for _, b := range battery {
		if b.Capacity == nil || *b.Capacity == 0 || b.Soc == nil {
			continue
		}

		share := 1.0
		if b.MinSoc != nil && b.MaxSoc != nil {
			maxSoc := *b.MaxSoc
			if maxSoc == 0 {
				maxSoc = 100 // unset means no upper limit
			}
			share = max(0, maxSoc-*b.MinSoc) / 100
		}

		res += *b.Capacity * 1e3 * share
	}

	return res
}

// solarMargin reduces the request's solar forecast by the margin the current
// prices and the installation's forecast accuracy make worthwhile. Without a
// home battery there is nothing to store insurance energy in.
func (site *Site) solarMargin(ts optimizer.TimeSeries, battery []types.Measurement) {
	capacity := usableBatteryCapacity(battery)
	if capacity == 0 || lo.Max(ts.Ft) == 0 {
		return
	}

	fills := solarFillsBattery(ts.Ft, ts.Gt, capacity)
	q := marginQuantile(ts.PN, ts.PE, eta*eta, fills)
	if q <= 0.5 {
		site.log.DEBUG.Printf("optimizer: no solar margin (q=%.2f, solar fills battery: %v)", q, fills)
		return
	}

	ratios, err := site.solarRatiosCached()
	if err != nil {
		return
	}

	factor := solarMarginFactor(ratios, q)
	site.log.DEBUG.Printf("optimizer: solar margin %.0f%% (q=%.2f, solar fills battery: %v, %d days)", (1-factor)*100, q, fills, len(ratios))

	if factor < 1 {
		applySolarMargin(ts.Ft, factor, optimizerDecaySlots)
	}
}
