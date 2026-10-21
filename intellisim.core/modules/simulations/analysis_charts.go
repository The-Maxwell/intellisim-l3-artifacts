package simulations

import (
	"bytes"
	"math"
	"sort"
	"sync"

	"github.com/golang/freetype/truetype"
	chart "github.com/wcharczuk/go-chart/v2"

	"intelli.sim.core/assets"
	datasims "intelli.sim.core/data/simulations"
)

var (
	chartFontOnce sync.Once
	chartFont     *truetype.Font
)

// chartTTF returns the Cyrillic-capable chart font parsed once from the same
// embedded DejaVuSans the PDF body uses. go-chart's built-in Roboto has no
// Cyrillic glyphs, so uk archetype/property labels would render as tofu.
// Returns nil on parse failure, letting go-chart fall back to Roboto.
func chartTTF() *truetype.Font {
	chartFontOnce.Do(func() {
		if f, err := truetype.Parse(assets.DejaVuSans); err == nil {
			chartFont = f
		}
	})
	return chartFont
}

// paddedRange returns an explicit Y range when the data is flat (min == max).
// go-chart does not reject a zero y-range delta (only a zero x-range one) and
// renders a blank plot instead: Translate divides by the zero delta, so every
// point maps to NaN and no line, ticks, or legend values are drawn. Constant
// series are the norm for small runs (stable population, flat properties), so
// pad the range to keep the line visible. Returns nil when the range is sane.
func paddedRange(ys ...[]float64) *chart.ContinuousRange {
	min, max := math.Inf(1), math.Inf(-1)
	for _, series := range ys {
		for _, v := range series {
			min = math.Min(min, v)
			max = math.Max(max, v)
		}
	}
	if math.IsInf(min, 1) || min != max {
		return nil
	}
	pad := math.Max(1, math.Abs(min)*0.1)
	return &chart.ContinuousRange{Min: min - pad, Max: max + pad}
}

// renderPopulationChart renders a population-over-time line chart (one series per
// archetype) to PNG bytes. names maps archetype UIDs to display names for the
// legend. Returns ("", nil-png) with a human-readable reason when the chart
// cannot be produced.
func renderPopulationChart(points []datasims.PopulationPoint, names map[string]string) ([]byte, string) {
	if len(points) == 0 {
		return nil, "no population data points"
	}
	if len(points) < 2 {
		return nil, "fewer than 2 population data points"
	}
	// Collect archetype keys in stable (sorted) order so series, legend, and
	// auto-assigned colors are deterministic across renders of the same data.
	keySet := map[string]struct{}{}
	for _, p := range points {
		for k := range p.Count {
			keySet[k] = struct{}{}
		}
	}
	keys := make([]string, 0, len(keySet))
	for k := range keySet {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var series []chart.Series
	var allYs [][]float64
	for _, k := range keys {
		xs := make([]float64, 0, len(points))
		ys := make([]float64, 0, len(points))
		for _, p := range points {
			xs = append(xs, float64(p.Step))
			ys = append(ys, float64(p.Count[k]))
		}
		label := names[k]
		if label == "" {
			label = k
		}
		series = append(series, chart.ContinuousSeries{Name: label, XValues: xs, YValues: ys})
		allYs = append(allYs, ys)
	}
	graph := chart.Chart{
		Width: 900, Height: 360,
		Font:   chartTTF(),
		XAxis:  chart.XAxis{Name: "Step"},
		YAxis:  chart.YAxis{Name: "Count"},
		Series: series,
	}
	if r := paddedRange(allYs...); r != nil {
		graph.YAxis.Range = r
	}
	graph.Elements = []chart.Renderable{chart.Legend(&graph)}
	var buf bytes.Buffer
	if err := graph.Render(chart.PNG, &buf); err != nil {
		return nil, "chart render failed: " + err.Error()
	}
	return buf.Bytes(), ""
}

// renderPropertyChart renders one numeric property's average-over-time to PNG
// bytes. Returns a human-readable skip reason when the chart cannot be produced.
func renderPropertyChart(ps PropertySeries) ([]byte, string) {
	if len(ps.Points) == 0 {
		return nil, "no data points"
	}
	if len(ps.Points) < 2 {
		return nil, "fewer than 2 data points"
	}
	xs := make([]float64, 0, len(ps.Points))
	ys := make([]float64, 0, len(ps.Points))
	for _, p := range ps.Points {
		xs = append(xs, float64(p.Step))
		ys = append(ys, toFloat(p.Value))
	}
	graph := chart.Chart{
		Width: 900, Height: 320,
		Font:   chartTTF(),
		XAxis:  chart.XAxis{Name: "Step"},
		YAxis:  chart.YAxis{Name: ps.Name},
		Series: []chart.Series{chart.ContinuousSeries{Name: ps.Name, XValues: xs, YValues: ys}},
	}
	if r := paddedRange(ys); r != nil {
		graph.YAxis.Range = r
	}
	var buf bytes.Buffer
	if err := graph.Render(chart.PNG, &buf); err != nil {
		return nil, "chart render failed: " + err.Error()
	}
	return buf.Bytes(), ""
}

func toFloat(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case float32:
		return float64(n)
	case int:
		return float64(n)
	case int64:
		return float64(n)
	case uint64:
		return float64(n)
	default:
		return 0
	}
}
