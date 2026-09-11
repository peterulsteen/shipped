package render

import (
	"strings"
	"testing"

	"github.com/peterulsteen/shipped/internal/metrics"
)

func TestTruncationNotes(t *testing.T) {
	if n := truncationNotes(&metrics.Report{}); len(n) != 0 {
		t.Errorf("nothing truncated but got notes: %q", n)
	}
	n := truncationNotes(&metrics.Report{FilesCappedPRs: 4, FilesCappedLines: 3200, WindowRawLines: 256000, ReviewsCappedPRs: 1})
	joined := strings.Join(n, " | ")
	for _, want := range []string{"4 merged pull requests", "up to 3.2k", "1.2% of this window", "1 pull request had more than 100 reviews"} {
		if !strings.Contains(joined, want) {
			t.Errorf("notes %q lack %q", joined, want)
		}
	}
}
