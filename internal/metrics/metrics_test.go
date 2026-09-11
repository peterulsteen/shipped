package metrics

import (
	"math"
	"testing"
	"time"

	"github.com/peterulsteen/shipped/internal/gh"
)

// west is a zone behind UTC, where rounding "today" to a UTC day lands on the
// previous local date. Pinned so the test does not depend on the machine's zone.
var west = time.FixedZone("UTC-5", -5*3600)

func at(day, hour int) time.Time { return time.Date(2026, 9, day, hour, 0, 0, 0, west) }

func ptr(t time.Time) *time.Time { return &t }

func idx(t *testing.T, r *Report, date string) int {
	t.Helper()
	for i, d := range r.Days {
		if d.Format(time.DateOnly) == date {
			return i
		}
	}
	t.Fatalf("%s is not in the report window %s..%s", date,
		r.Days[0].Format(time.DateOnly), r.Days[len(r.Days)-1].Format(time.DateOnly))
	return -1
}

func total(v []float64) float64 {
	var s float64
	for _, x := range v {
		s += x
	}
	return s
}

// 10:00 at UTC-5 is 15:00 UTC; truncating that to a UTC day gives 19:00 the
// previous local evening, so the old code labelled today as yesterday.
func TestTodayIsTheLastDayWestOfUTC(t *testing.T) {
	now := at(11, 10)
	r := Build(now, "me", []gh.PullRequest{{Repo: "o/r", Number: 1, CreatedAt: at(11, 9)}}, nil, 10, 1, 8, 18)

	if got := r.Days[len(r.Days)-1].Format(time.DateOnly); got != "2026-09-11" {
		t.Fatalf("last day = %s, want 2026-09-11", got)
	}
	if got := r.OpenedPerDay[idx(t, r, "2026-09-11")]; got != 1 {
		t.Errorf("a PR opened this morning counted %v times today, want 1", got)
	}
}

// Reviews received are dated by when the review was submitted, not by when the
// PR merged, and a comment the author leaves on their own PR is not one.
func TestReviewsReceivedKeyOnSubmission(t *testing.T) {
	pr := gh.PullRequest{
		Repo: "o/r", Number: 2, CreatedAt: at(5, 10), MergedAt: ptr(at(11, 11)),
		Reviews: []gh.Review{{At: at(6, 10), By: "me"}, {At: at(8, 10), By: "alice"}},
	}
	r := Build(at(11, 12), "me", []gh.PullRequest{pr}, nil, 10, 1, 8, 18)

	if got := r.ReviewsGotten[idx(t, r, "2026-09-08")]; got != 1 {
		t.Errorf("review submitted Sep 8 counted %v on Sep 8, want 1", got)
	}
	if got := r.ReviewsGotten[idx(t, r, "2026-09-11")]; got != 0 {
		t.Errorf("merge day counted %v reviews received, want 0", got)
	}
	if got := total(r.ReviewsGotten); got != 1 {
		t.Errorf("total reviews received = %v, want 1 (own review excluded)", got)
	}
}

// Reviews given count the user's FIRST review of someone else's PR, and the
// latency runs from the PR opening to that review.
func TestReviewsGivenUseFirstReview(t *testing.T) {
	pr := gh.PullRequest{
		Repo: "o/r", Number: 3, Author: "bob", CreatedAt: at(7, 9),
		Reviews: []gh.Review{{At: at(7, 10), By: "carol"}, {At: at(9, 14), By: "me"}, {At: at(10, 9), By: "me"}},
	}
	r := Build(at(11, 12), "me", nil, []gh.PullRequest{pr}, 10, 1, 8, 18)

	if got := total(r.ReviewsGiven); got != 1 {
		t.Errorf("reviews given = %v, want 1 (second review of the same PR is not a new one)", got)
	}
	i := idx(t, r, "2026-09-09")
	if r.ReviewsGiven[i] != 1 {
		t.Errorf("first review landed Sep 9 but counted %v there", r.ReviewsGiven[i])
	}
	if got := r.ReviewLatency[i]; got != 53 {
		t.Errorf("latency = %vh, want 53h (Sep 7 09:00 to Sep 9 14:00)", got)
	}
}

func TestFirstReviewerConcentration(t *testing.T) {
	var prs []gh.PullRequest
	for n, first := range []string{"bot", "bot", "bot", "alice"} {
		prs = append(prs, gh.PullRequest{
			Repo: "o/r", Number: n, CreatedAt: at(8, 9),
			Reviews: []gh.Review{{At: at(8, 10), By: first}, {At: at(9, 10), By: "dave"}},
		})
	}
	r := Build(at(11, 12), "me", prs, nil, 10, 1, 8, 18)
	if r.TopFirstReviewer != "bot" || r.TopFirstReviewerShare != 75 {
		t.Errorf("top first reviewer = %q at %v%%, want bot at 75%%", r.TopFirstReviewer, r.TopFirstReviewerShare)
	}
}

// Percentiles interpolate between ranks, so the median of an even count is the
// mean of the middle two and a p90 of ten values is not simply the maximum.
func TestPercentileInterpolates(t *testing.T) {
	ten := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	cases := []struct {
		name string
		v    []float64
		q    float64
		want float64
	}{
		{"median of an even count", []float64{4, 1, 3, 2}, 0.5, 2.5},
		{"median of an odd count", []float64{5, 1, 3}, 0.5, 3},
		{"p90 of ten is not the max", ten, 0.9, 9.1},
		{"single value", []float64{7}, 0.9, 7},
		{"empty", nil, 0.5, 0},
	}
	for _, tc := range cases {
		if got := percentile(tc.v, tc.q); math.Abs(got-tc.want) > 1e-9 {
			t.Errorf("%s: percentile(%v, %v) = %v, want %v", tc.name, tc.v, tc.q, got, tc.want)
		}
	}
}
