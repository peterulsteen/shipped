// Package render turns a metrics report into a self-contained HTML page.
package render

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/peterulsteen/shipped/internal/metrics"
)

const (
	width   = 900
	height  = 250
	padTop  = 14
	padRt   = 16
	padBot  = 26
	padLeft = 52
)

type scale struct {
	X0, X1, Y0, Y1 float64
	N              int
	YMax           float64
}

// Chart is one plotted figure.
type Chart struct {
	ID, Title, Sub string
	Days           []time.Time
	Series         []metrics.Series
	Fill           bool
}

func newScale(n int, ymax float64) scale {
	return scale{X0: padLeft, X1: width - padRt, Y0: height - padBot, Y1: padTop, N: n, YMax: ymax}
}

func (s scale) at(i int, v float64) (float64, float64) {
	denom := float64(max(1, s.N-1))
	x := s.X0 + (s.X1-s.X0)*(float64(i)/denom)
	y := s.Y0 - (s.Y0-s.Y1)*(v/s.YMax)
	return round1(x), round1(y)
}

type tick struct {
	Value float64
	Label string
}

// axis picks gridlines at a step of 1, 2, 2.5 or 5 times a power of ten, at most
// four intervals, so every label states its gridline exactly. Splitting an
// arbitrary bound into quarters printed 6.25 as "6.2".
func axis(peak float64) (ymax float64, ticks []tick) {
	step := 1.0
	if peak > 0 {
		raw := peak / 4
		mag := math.Pow(10, math.Floor(math.Log10(raw)))
		step = 10 * mag
		for _, m := range []float64{1, 2, 2.5, 5} {
			if m*mag >= raw {
				step = m * mag
				break
			}
		}
	}
	n := int(math.Max(1, math.Ceil(peak/step-1e-9)))
	for k := 0; k <= n; k++ {
		v := math.Round(float64(k)*step*1e6) / 1e6
		ticks = append(ticks, tick{Value: v, Label: tickLabel(v)})
	}
	return ticks[n].Value, ticks
}

func tickLabel(v float64) string {
	if math.Abs(v) >= 1000 {
		return strconv.FormatFloat(v/1000, 'f', -1, 64) + "k"
	}
	return strconv.FormatFloat(v, 'f', -1, 64)
}

func fmtNum(v float64) string {
	switch {
	case math.Abs(v) >= 10000:
		return fmt.Sprintf("%.0fk", v/1000)
	case math.Abs(v) >= 1000:
		return fmt.Sprintf("%.1fk", v/1000)
	case v == math.Trunc(v):
		return fmt.Sprintf("%.0f", v)
	default:
		return fmt.Sprintf("%.1f", v)
	}
}

func round1(v float64) float64 { return math.Round(v*10) / 10 }

// SVG renders the chart, its legend, a data table, and the JSON its hover layer reads.
func (c Chart) SVG() string {
	peak := c.peak()
	ymax, ticks := axis(peak)
	sc := newScale(len(c.Days), ymax)

	var b strings.Builder
	for _, t := range ticks {
		_, y := sc.at(0, t.Value)
		fmt.Fprintf(&b, `<line class="grid" x1="%g" x2="%g" y1="%g" y2="%g"/>`, sc.X0, sc.X1, y, y)
		fmt.Fprintf(&b, `<text class="ytick" x="%g" y="%g">%s</text>`, sc.X0-8, y+3.5, t.Label)
	}

	step := max(1, len(c.Days)/6)
	for i := 0; i < len(c.Days); i += step {
		x, _ := sc.at(i, 0)
		fmt.Fprintf(&b, `<text class="xtick" x="%g" y="%d">%s</text>`,
			x, height-8, c.Days[i].Format("Jan 2"))
	}

	var placed []float64
	for k, s := range c.Series {
		colour := fmt.Sprintf("var(--series-%d)", k+1)
		if c.Fill && k == 0 {
			fmt.Fprintf(&b, `<path d="%s" fill="%s" fill-opacity=".14"/>`, areaPath(sc, s.Values), colour)
		}
		fmt.Fprintf(&b,
			`<path d="%s" fill="none" stroke="%s" stroke-width="2" stroke-linejoin="round" stroke-linecap="round"/>`,
			linePath(sc, s.Values), colour)

		last := len(s.Values) - 1
		ex, ey := sc.at(last, s.Values[last])
		ly := ey - 9
		for collides(placed, ly) {
			ly -= 13
		}
		placed = append(placed, ly)
		fmt.Fprintf(&b,
			`<circle cx="%g" cy="%g" r="3.5" fill="%s" stroke="var(--surface-1)" stroke-width="2"/>`+
				`<text class="endlab" x="%g" y="%g" fill="%s">%s</text>`,
			ex, ey, colour, ex-8, ly, colour, fmtNum(s.Values[last]))
	}

	fmt.Fprintf(&b,
		`<g class="hov" hidden><line class="cross" y1="%g" y2="%g"/></g>`+
			`<rect class="hit" x="%g" y="%g" width="%g" height="%g" fill="transparent"/>`,
		sc.Y1, sc.Y0, sc.X0, sc.Y1, sc.X1-sc.X0, sc.Y0-sc.Y1)
	return b.String()
}

func collides(placed []float64, y float64) bool {
	for _, p := range placed {
		if math.Abs(p-y) < 13 {
			return true
		}
	}
	return false
}

func linePath(sc scale, vals []float64) string {
	pts := make([]string, len(vals))
	for i, v := range vals {
		x, y := sc.at(i, v)
		pts[i] = fmt.Sprintf("%g,%g", x, y)
	}
	return "M" + strings.Join(pts, " L")
}

func areaPath(sc scale, vals []float64) string {
	first, _ := sc.at(0, 0)
	last, _ := sc.at(len(vals)-1, 0)
	return fmt.Sprintf("M%g,%g L%s L%g,%g Z", first, sc.Y0, strings.TrimPrefix(linePath(sc, vals), "M"), last, sc.Y0)
}

// Data is the JSON the hover layer reads.
func (c Chart) Data() string {
	days := make([]string, len(c.Days))
	for i, d := range c.Days {
		days[i] = d.Format("Mon 2 Jan")
	}
	ymax, _ := axis(c.peak())
	payload := map[string]any{
		"days":   days,
		"series": c.Series,
		"scale":  newScale(len(c.Days), ymax),
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return "{}"
	}
	return string(b)
}

// Legend renders swatches, omitted for a single series where the title names it.
func (c Chart) Legend() string {
	if len(c.Series) < 2 {
		return ""
	}
	var b strings.Builder
	for k, s := range c.Series {
		fmt.Fprintf(&b, `<span class="lg"><i style="background:var(--series-%d)"></i>%s</span>`, k+1, s.Name)
	}
	return b.String()
}

// Table renders the accessible data view behind every chart.
func (c Chart) Table() string {
	var b strings.Builder
	b.WriteString("<thead><tr><th>Day</th>")
	for _, s := range c.Series {
		fmt.Fprintf(&b, "<th>%s</th>", s.Name)
	}
	b.WriteString("</tr></thead><tbody>")
	for i, d := range c.Days {
		fmt.Fprintf(&b, "<tr><td>%s</td>", d.Format(time.DateOnly))
		for _, s := range c.Series {
			fmt.Fprintf(&b, "<td>%s</td>", fmtNum(s.Values[i]))
		}
		b.WriteString("</tr>")
	}
	b.WriteString("</tbody>")
	return b.String()
}

func (c Chart) peak() float64 {
	var peak float64
	for _, s := range c.Series {
		for _, v := range s.Values {
			peak = math.Max(peak, v)
		}
	}
	return peak
}
