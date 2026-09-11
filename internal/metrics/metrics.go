// Package metrics turns collected pull requests into daily series.
package metrics

import (
	"math"
	"sort"
	"time"

	"github.com/peterulsteen/shipped/internal/gh"
)

// Series is one named daily sequence aligned to Days.
type Series struct {
	Name   string    `json:"name"`
	Unit   string    `json:"unit"`
	Values []float64 `json:"values"`
}

// Report is everything the dashboard renders.
type Report struct {
	Days []time.Time

	OpenedPerDay    []float64
	MergedPerDay    []float64
	Backlog         []float64
	CycleP50        []float64
	CycleP90        []float64
	ReviewsGiven    []float64
	ReviewsGotten   []float64
	SrcLines        []float64
	RawLines        []float64
	MedianPRSize    []float64
	ReviewLatency   []float64
	FirstReviewWait []float64

	// Headline values for the current window.
	OpenedAvg, MergedAvg         float64
	OpenedPrev, MergedPrev       float64
	GivenAvg, GottenAvg          float64
	GivenPrev                    float64
	OpenNow                      float64
	MedianCycleHours             float64
	MedianSizeLines              float64
	SrcAvg, SrcPrev              float64
	GeneratedShare               float64
	AfterHoursShare              float64
	OldestOpenDays               float64
	TotalAuthored, TotalReviewed int
	Reciprocity                  float64

	// TopFirstReviewer is whoever most often reaches a PR first, and the share
	// they cover. A very high share paired with a near-zero FirstReviewWait is
	// the signature of automation -- which this metric cannot distinguish from a
	// diligent colleague, so the dashboard shows both numbers side by side
	// rather than implying that a human engaged.
	TopFirstReviewer      string
	TopFirstReviewerShare float64
}

// concentration returns the most frequent key and its share of the total.
func concentration(counts map[string]int) (string, float64) {
	var top string
	var best, total int
	for k, v := range counts {
		total += v
		if v > best {
			top, best = k, v
		}
	}
	if total == 0 {
		return "", 0
	}
	return top, float64(best) / float64(total) * 100
}

type options struct {
	window, rolling          int
	workdayStart, workdayEnd int
}

// Build computes every series over the trailing window.
func Build(now time.Time, login string, authored, reviewed []gh.PullRequest, window, rolling, workStart, workEnd int) *Report {
	o := options{window, rolling, workStart, workEnd}
	loc := now.Location()
	// Local midnight, not Truncate(24h): Truncate rounds to a UTC day, which west
	// of UTC labels every day as the one before and drops today's activity.
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	days := make([]time.Time, window)
	index := map[string]int{}
	for i := range days {
		d := today.AddDate(0, 0, -(window - 1 - i))
		days[i] = d
		index[d.Format(time.DateOnly)] = i
	}

	r := &Report{
		Days:          days,
		OpenedPerDay:  make([]float64, window),
		MergedPerDay:  make([]float64, window),
		Backlog:       make([]float64, window),
		ReviewsGiven:  make([]float64, window),
		ReviewsGotten: make([]float64, window),
		SrcLines:      make([]float64, window),
		RawLines:      make([]float64, window),
		TotalAuthored: len(authored),
		TotalReviewed: len(reviewed),
	}

	cycleByDay := make([][]float64, window)
	sizeByDay := make([][]float64, window)
	latencyByDay := make([][]float64, window)
	waitByDay := make([][]float64, window)
	firstReviewers := map[string]int{}
	var afterHours, totalCreated float64
	var allSizes []float64
	var allCycles []float64

	for _, pr := range authored {
		if i, ok := index[pr.CreatedAt.In(loc).Format(time.DateOnly)]; ok {
			r.OpenedPerDay[i]++
			totalCreated++
			if isAfterHours(pr.CreatedAt.In(loc), o) {
				afterHours++
			}
		}
		if pr.MergedAt != nil {
			if i, ok := index[pr.MergedAt.In(loc).Format(time.DateOnly)]; ok {
				r.MergedPerDay[i]++
				r.RawLines[i] += float64(pr.Additions + pr.Deletions)
				r.SrcLines[i] += float64(pr.SrcAdd + pr.SrcDel)
				h := pr.MergedAt.Sub(pr.CreatedAt).Hours()
				cycleByDay[i] = append(cycleByDay[i], h)
				allCycles = append(allCycles, h)
				size := float64(pr.SrcAdd + pr.SrcDel)
				sizeByDay[i] = append(sizeByDay[i], size)
				allSizes = append(allSizes, size)
			}
		}
		// A PR counts toward the backlog on every day it was open.
		end := pr.MergedAt
		if end == nil {
			end = pr.ClosedAt
		}
		for i, d := range days {
			if !pr.CreatedAt.After(endOfDay(d)) && (end == nil || end.After(endOfDay(d))) {
				r.Backlog[i]++
			}
		}
		for _, rev := range pr.Reviews {
			if rev.By == login {
				continue // your own comment on your own PR is not a review received
			}
			if i, ok := index[rev.At.In(loc).Format(time.DateOnly)]; ok {
				r.ReviewsGotten[i]++
			}
		}
		if first, ok := pr.FirstReviewBy(login); ok {
			firstReviewers[first.By]++
			if i, ok := index[pr.CreatedAt.In(loc).Format(time.DateOnly)]; ok {
				waitByDay[i] = append(waitByDay[i], first.At.Sub(pr.CreatedAt).Hours())
			}
		}
	}

	for _, pr := range reviewed {
		mine, ok := pr.MyReview(login)
		if !ok {
			continue
		}
		if i, ok := index[mine.At.In(loc).Format(time.DateOnly)]; ok {
			r.ReviewsGiven[i]++
			latencyByDay[i] = append(latencyByDay[i], mine.At.Sub(pr.CreatedAt).Hours())
		}
	}

	r.CycleP50 = trailingPercentile(cycleByDay, rolling, 0.5)
	r.CycleP90 = trailingPercentile(cycleByDay, rolling, 0.9)
	r.MedianPRSize = trailingPercentile(sizeByDay, rolling, 0.5)
	r.ReviewLatency = trailingPercentile(latencyByDay, rolling, 0.5)
	r.FirstReviewWait = trailingPercentile(waitByDay, rolling, 0.5)
	r.TopFirstReviewer, r.TopFirstReviewerShare = concentration(firstReviewers)

	r.OpenedPerDay = rollingMean(r.OpenedPerDay, rolling)
	r.MergedPerDay = rollingMean(r.MergedPerDay, rolling)
	givenRaw := append([]float64(nil), r.ReviewsGiven...)
	gottenRaw := append([]float64(nil), r.ReviewsGotten...)
	r.ReviewsGiven = rollingMean(r.ReviewsGiven, rolling)
	r.ReviewsGotten = rollingMean(r.ReviewsGotten, rolling)
	srcRaw := append([]float64(nil), r.SrcLines...)
	rawRaw := append([]float64(nil), r.RawLines...)
	r.SrcLines = rollingMean(r.SrcLines, rolling)
	r.RawLines = rollingMean(r.RawLines, rolling)

	r.OpenedAvg, r.OpenedPrev = tailMean(r.OpenedPerDay, rolling)
	r.MergedAvg, r.MergedPrev = tailMean(r.MergedPerDay, rolling)
	r.GivenAvg, r.GivenPrev = tailMean(givenRaw, rolling)
	r.GottenAvg, _ = tailMean(gottenRaw, rolling)
	r.SrcAvg, r.SrcPrev = tailMean(srcRaw, rolling)
	r.OpenNow = r.Backlog[len(r.Backlog)-1]
	r.MedianCycleHours = percentile(allCycles, 0.5)
	r.MedianSizeLines = percentile(allSizes, 0.5)

	if rawSum := sum(rawRaw[max(0, len(rawRaw)-rolling):]); rawSum > 0 {
		r.GeneratedShare = (1 - sum(srcRaw[max(0, len(srcRaw)-rolling):])/rawSum) * 100
	}
	if totalCreated > 0 {
		r.AfterHoursShare = afterHours / totalCreated * 100
	}
	if r.MergedAvg > 0 {
		r.Reciprocity = r.GivenAvg / r.MergedAvg
	}
	r.OldestOpenDays = oldestOpen(authored, now)
	return r
}

func isAfterHours(t time.Time, o options) bool {
	if t.Weekday() == time.Saturday || t.Weekday() == time.Sunday {
		return true
	}
	h := t.Hour()
	return h < o.workdayStart || h >= o.workdayEnd
}

func endOfDay(d time.Time) time.Time {
	return time.Date(d.Year(), d.Month(), d.Day(), 23, 59, 59, 0, d.Location())
}

func oldestOpen(prs []gh.PullRequest, now time.Time) float64 {
	var oldest float64
	for _, pr := range prs {
		if pr.MergedAt == nil && pr.ClosedAt == nil {
			if age := now.Sub(pr.CreatedAt).Hours() / 24; age > oldest {
				oldest = age
			}
		}
	}
	return oldest
}

func rollingMean(v []float64, n int) []float64 {
	out := make([]float64, len(v))
	for i := range v {
		lo := max(0, i-n+1)
		window := v[lo : i+1]
		out[i] = round(sum(window)/float64(len(window)), 2)
	}
	return out
}

func trailingPercentile(byDay [][]float64, n int, q float64) []float64 {
	out := make([]float64, len(byDay))
	for i := range byDay {
		lo := max(0, i-n+1)
		var pool []float64
		for _, day := range byDay[lo : i+1] {
			pool = append(pool, day...)
		}
		out[i] = round(percentile(pool, q), 1)
	}
	return out
}

func percentile(v []float64, q float64) float64 {
	if len(v) == 0 {
		return 0
	}
	s := append([]float64(nil), v...)
	sort.Float64s(s)
	// Linear interpolation between ranks: the median of an even count is the
	// mean of the middle two, and a p90 of a small sample is not just its max.
	pos := q * float64(len(s)-1)
	lo, hi := int(math.Floor(pos)), int(math.Ceil(pos))
	return s[lo] + (s[hi]-s[lo])*(pos-float64(lo))
}

// tailMean returns the mean of the last n values and of the n before those, so
// a headline can show a change without a second pass over the data.
func tailMean(v []float64, n int) (current, previous float64) {
	if len(v) == 0 {
		return 0, 0
	}
	cur := v[max(0, len(v)-n):]
	current = sum(cur) / float64(len(cur))
	if len(v) >= 2*n {
		prev := v[len(v)-2*n : len(v)-n]
		previous = sum(prev) / float64(len(prev))
	}
	return current, previous
}

func sum(v []float64) float64 {
	var t float64
	for _, x := range v {
		t += x
	}
	return t
}

func round(v float64, places int) float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	p := math.Pow(10, float64(places))
	return math.Round(v*p) / p
}
