package simulations

import (
	"testing"

	"github.com/stretchr/testify/require"

	datasims "intelli.sim.core/data/simulations"
)

// Constant series produced visually-empty charts: go-chart accepts a zero
// y-range delta and Translate then maps every point to NaN, drawing axes but
// no line/ticks/legend values. paddedRange must widen flat data.
func TestPaddedRange(t *testing.T) {
	r := paddedRange([]float64{6, 6, 6})
	require.NotNil(t, r)
	require.Less(t, r.Min, 6.0)
	require.Greater(t, r.Max, 6.0)

	require.Nil(t, paddedRange([]float64{1, 2, 3}), "varying data needs no explicit range")
	require.Nil(t, paddedRange(nil), "no data → leave range alone")

	// Multi-series: flat only when ALL series share one constant value.
	require.Nil(t, paddedRange([]float64{5, 5}, []float64{7, 7}))
	r = paddedRange([]float64{0, 0}, []float64{0, 0})
	require.NotNil(t, r, "all-zero series must still get a visible range")
	require.Less(t, r.Min, r.Max)
}

func TestRenderPopulationChart(t *testing.T) {
	names := map[string]string{"wolf_uid": "Wolf"}

	_, reason := renderPopulationChart(nil, names)
	require.NotEmpty(t, reason)
	_, reason = renderPopulationChart([]datasims.PopulationPoint{{Step: 0, Count: map[string]int{"wolf_uid": 3}}}, names)
	require.NotEmpty(t, reason)

	flat := make([]datasims.PopulationPoint, 0, 10)
	for s := int64(0); s < 10; s++ {
		flat = append(flat, datasims.PopulationPoint{Step: s, Count: map[string]int{"wolf_uid": 6, "unmapped": 4}})
	}
	png, reason := renderPopulationChart(flat, names)
	require.Empty(t, reason)
	require.NotEmpty(t, png, "flat population must still render a visible chart")
}

func TestRenderPropertyChart(t *testing.T) {
	ps := PropertySeries{Name: "energy"}
	_, reason := renderPropertyChart(ps)
	require.NotEmpty(t, reason)

	for s := int64(0); s < 10; s++ {
		ps.Points = append(ps.Points, datasims.TimeSeriesPoint{Step: s, Value: 50.0})
	}
	png, reason := renderPropertyChart(ps)
	require.Empty(t, reason)
	require.NotEmpty(t, png, "flat property must still render a visible chart")
}

// Cyrillic labels require the embedded DejaVu font (go-chart's Roboto has no
// Cyrillic glyphs). The parse must succeed so charts never fall back silently.
func TestChartTTFParses(t *testing.T) {
	require.NotNil(t, chartTTF())
}
