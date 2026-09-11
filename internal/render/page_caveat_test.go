package render

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/peterulsteen/shipped/internal/metrics"
)

func render(t *testing.T, r *metrics.Report) string {
	t.Helper()
	out := filepath.Join(t.TempDir(), "dashboard.html")
	if err := Page(out, "someone", r); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// The first-reviewer caveat must reach the page at or above half, and stay off
// below it. It moved from its own field into the notes list in v0.1.3, which no
// test covered at the time.
func TestFirstReviewerCaveatRenders(t *testing.T) {
	z := []float64{0, 0}
	r := &metrics.Report{
		Days:         []time.Time{time.Now().AddDate(0, 0, -1), time.Now()},
		OpenedPerDay: z, MergedPerDay: z, Backlog: z, CycleP50: z, CycleP90: z,
		ReviewsGiven: z, ReviewsGotten: z, SrcLines: z, RawLines: z, MedianPRSize: z,
		ReviewLatency: z, FirstReviewWait: z,
		TopFirstReviewer: "review-bot", TopFirstReviewerShare: 62,
	}
	if !strings.Contains(render(t, r), "review-bot is first to review 62%") {
		t.Error("a 62% first reviewer produced no caveat")
	}
	r.TopFirstReviewerShare = 40
	if strings.Contains(render(t, r), "is first to review") {
		t.Error("the caveat rendered for a 40% first reviewer, below the 50% threshold")
	}
}
