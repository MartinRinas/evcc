package core

import (
	"testing"

	"github.com/evcc-io/evcc/core/types"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// dayPrices returns per Wh prices: a cheap night and an expensive rest of day
func dayPrices(cheap, expensive float32) []float32 {
	return lo.Times(marginSlots, func(i int) float32 {
		if i < marginSlots/4 {
			return cheap / 1e3
		}
		return expensive / 1e3
	})
}

func TestMarginQuantile(t *testing.T) {
	const rt = 0.81
	feedIn := lo.RepeatBy(marginSlots, func(int) float32 { return 0.08 / 1e3 })

	tc := []struct {
		name       string
		pn, pe     []float32
		solarFills bool
		lo, hi     float64
	}{
		{"winter: shortfall expensive, carry-over cheap", dayPrices(0.20, 0.35), feedIn, false, 0.65, 0.75},
		{"summer: solar fills the battery", dayPrices(0.20, 0.35), feedIn, true, 0.3, 0.45},
		{"flat tariff", dayPrices(0.30, 0.30), feedIn, false, 0, 0},
		{"no prices", nil, nil, false, 0, 0},
	}

	for _, tc := range tc {
		t.Run(tc.name, func(t *testing.T) {
			q := marginQuantile(tc.pn, tc.pe, rt, tc.solarFills)
			assert.GreaterOrEqual(t, q, tc.lo)
			assert.LessOrEqual(t, q, tc.hi)
		})
	}
}

func TestSolarMarginFactor(t *testing.T) {
	// 0.50, 0.55, ... 1.95: median 1.20
	ratios := lo.Times(30, func(i int) float64 { return 0.5 + 0.05*float64(i) })

	tc := []struct {
		name   string
		ratios []float64
		q      float64
		expect float64
	}{
		{"no trade-off", ratios, 0, 1},
		{"at median", ratios, 0.5, 1},
		{"P30", ratios, 0.7, 0.9 / 1.2},
		{"capped at P10", ratios, 0.99, 0.6 / 1.2},
		{"too few samples", ratios[:solarScaleMinSamples-1], 0.9, 1},
		{"no spread", lo.RepeatBy(20, func(int) float64 { return 0.8 }), 0.9, 1},
	}

	for _, tc := range tc {
		t.Run(tc.name, func(t *testing.T) {
			assert.InDelta(t, tc.expect, solarMarginFactor(tc.ratios, tc.q), 1e-9)
		})
	}
}

func TestApplySolarMargin(t *testing.T) {
	ft := []float32{100, 100, 100, 100, 100, 100}
	applySolarMargin(ft, 0.6, 4)
	assert.InDeltaSlice(t, []float32{100, 90, 80, 70, 60, 60}, ft, 1e-4)
}

func TestSolarFillsBattery(t *testing.T) {
	ft := []float32{0, 3000, 3000, 0}
	gt := []float32{500, 1000, 1000, 500}

	assert.True(t, solarFillsBattery(ft, gt, 4000))
	assert.False(t, solarFillsBattery(ft, gt, 4001))
}

func TestUsableBatteryCapacity(t *testing.T) {
	tc := []struct {
		name    string
		battery []types.Measurement
		expect  float64
	}{
		{"none", nil, 0},
		{"no capacity", []types.Measurement{{Soc: new(50.0)}}, 0},
		{"no soc", []types.Measurement{{Capacity: new(10.0)}}, 0},
		{"no limits", []types.Measurement{{Capacity: new(10.0), Soc: new(50.0)}}, 10e3},
		{"limits", []types.Measurement{{Capacity: new(10.0), Soc: new(50.0), MinSoc: new(20.0), MaxSoc: new(90.0)}}, 7e3},
		{"unset max soc", []types.Measurement{{Capacity: new(10.0), Soc: new(50.0), MinSoc: new(20.0), MaxSoc: new(0.0)}}, 8e3},
		{"multiple", []types.Measurement{
			{Capacity: new(10.0), Soc: new(50.0)},
			{Capacity: new(5.0), Soc: new(50.0)},
		}, 15e3},
	}

	for _, tc := range tc {
		t.Run(tc.name, func(t *testing.T) {
			require.InDelta(t, tc.expect, usableBatteryCapacity(tc.battery), 1e-6)
		})
	}
}
