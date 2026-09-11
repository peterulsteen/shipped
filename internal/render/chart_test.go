package render

import (
	"math"
	"strconv"
	"strings"
	"testing"
)

func parseLabel(t *testing.T, s string) float64 {
	t.Helper()
	mult := 1.0
	if strings.HasSuffix(s, "k") {
		s, mult = strings.TrimSuffix(s, "k"), 1000
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		t.Fatalf("label %q is not a number", s)
	}
	return v * mult
}

// Every axis label must state its gridline's value exactly.
func TestAxisLabelsAreExact(t *testing.T) {
	for _, peak := range []float64{0, 0.3, 0.9, 11.5, 20, 20.3, 25, 63, 1700, 23456} {
		ymax, ticks := axis(peak)
		if ymax < peak {
			t.Errorf("peak %v: axis tops out at %v, below the data", peak, ymax)
		}
		if n := len(ticks) - 1; n < 1 || n > 4 {
			t.Errorf("peak %v: %d intervals, want 1-4", peak, n)
		}
		if ticks[0].Value != 0 {
			t.Errorf("peak %v: first tick %v, want 0", peak, ticks[0].Value)
		}
		for _, tk := range ticks {
			if got := parseLabel(t, tk.Label); math.Abs(got-tk.Value) > 1e-9 {
				t.Errorf("peak %v: label %q reads as %v but the gridline is at %v", peak, tk.Label, got, tk.Value)
			}
		}
	}
}

func TestAxisPicksRoundSteps(t *testing.T) {
	cases := map[float64]string{
		20:   "0 5 10 15 20",
		20.3: "0 10 20 30",
		0.9:  "0 0.25 0.5 0.75 1",
		1700: "0 500 1k 1.5k 2k",
	}
	for peak, want := range cases {
		_, ticks := axis(peak)
		labels := make([]string, len(ticks))
		for i, tk := range ticks {
			labels[i] = tk.Label
		}
		if got := strings.Join(labels, " "); got != want {
			t.Errorf("axis(%v) = %q, want %q", peak, got, want)
		}
	}
}
