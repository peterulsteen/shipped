package render

import (
	"bytes"
	_ "embed"
	"fmt"
	"html/template"
	"math"
	"os"
	"path/filepath"
	"time"

	"github.com/peterulsteen/shipped/internal/metrics"
)

//go:embed page.html
var pageTemplate string

// Tile is one headline number.
type Tile struct {
	Label, Value, Note string
	Delta              *float64
}

type pageData struct {
	Login        string
	Range        string
	Built        string
	Authored     int
	Reviewed     int
	Tiles        []Tile
	Charts       []Chart
	ReviewCaveat string
}

// Page renders the whole dashboard to path.
func Page(path, login string, r *metrics.Report) error {
	tmpl, err := template.New("page").Funcs(template.FuncMap{
		"svg":    func(c Chart) template.HTML { return template.HTML(c.SVG()) },
		"legend": func(c Chart) template.HTML { return template.HTML(c.Legend()) },
		"table":  func(c Chart) template.HTML { return template.HTML(c.Table()) },
		"data":   func(c Chart) template.JS { return template.JS(c.Data()) },
		"pct":    func(v float64) string { return fmt.Sprintf("%.0f", math.Abs(v)) },
		"neg":    func(v float64) bool { return v < 0 },
	}).Parse(pageTemplate)
	if err != nil {
		return err
	}

	data := pageData{
		Login:    login,
		Range:    fmt.Sprintf("%s – %s", r.Days[0].Format("2 Jan"), r.Days[len(r.Days)-1].Format("2 Jan 2006")),
		Built:    time.Now().Format("2 Jan 15:04"),
		Authored: r.TotalAuthored,
		Reviewed: r.TotalReviewed,
		Tiles:    tiles(r),
		Charts:   charts(r),
	}
	if r.TopFirstReviewer != "" && r.TopFirstReviewerShare >= 50 {
		data.ReviewCaveat = fmt.Sprintf(
			"%s is first to review %.0f%% of your pull requests. GitHub reports every reviewer as a user, "+
				"so a high share paired with a very short wait may be automation rather than a colleague — "+
				"read the two together.",
			r.TopFirstReviewer, r.TopFirstReviewerShare)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0o644)
}

func delta(cur, prev float64) *float64 {
	if prev == 0 {
		return nil
	}
	d := (cur - prev) / prev * 100
	return &d
}

func tiles(r *metrics.Report) []Tile {
	return []Tile{
		{Label: "PRs opened / day", Value: fmtNum(r.OpenedAvg), Note: "rolling average", Delta: delta(r.OpenedAvg, r.OpenedPrev)},
		{Label: "PRs merged / day", Value: fmtNum(r.MergedAvg), Note: "rolling average", Delta: delta(r.MergedAvg, r.MergedPrev)},
		{Label: "Reviews given / day", Value: fmtNum(r.GivenAvg), Note: fmt.Sprintf("%.1fx your merge rate", r.Reciprocity), Delta: delta(r.GivenAvg, r.GivenPrev)},
		{Label: "Open PRs", Value: fmtNum(r.OpenNow), Note: fmt.Sprintf("oldest is %.0f days", r.OldestOpenDays)},
		{Label: "Median time to merge", Value: fmt.Sprintf("%.1fh", r.MedianCycleHours), Note: "open to merge"},
		{Label: "Median PR size", Value: fmt.Sprintf("%.0f", r.MedianSizeLines), Note: "authored lines, excl. generated"},
		{Label: "Authored lines / day", Value: fmtNum(r.SrcAvg), Note: fmt.Sprintf("%.0f%% of the diff was generated", r.GeneratedShare), Delta: delta(r.SrcAvg, r.SrcPrev)},
		{Label: "After hours", Value: fmt.Sprintf("%.0f%%", r.AfterHoursShare), Note: "PRs opened evenings & weekends"},
	}
}

func charts(r *metrics.Report) []Chart {
	return []Chart{
		{ID: "c1", Title: "Pull requests per day", Sub: "rolling average", Days: r.Days, Series: []metrics.Series{
			{Name: "Opened", Unit: " PRs", Values: r.OpenedPerDay},
			{Name: "Merged", Unit: " PRs", Values: r.MergedPerDay},
		}},
		{ID: "c2", Title: "Review, both directions", Sub: "the half most tools never show you", Days: r.Days, Series: []metrics.Series{
			{Name: "Given", Unit: " reviews", Values: r.ReviewsGiven},
			{Name: "Received", Unit: " reviews", Values: r.ReviewsGotten},
		}},
		{ID: "c3", Title: "Open pull requests", Sub: "still open at end of day", Days: r.Days, Fill: true, Series: []metrics.Series{
			{Name: "Open", Unit: " open", Values: r.Backlog},
		}},
		{ID: "c4", Title: "Time to merge", Sub: "hours from open to merge", Days: r.Days, Series: []metrics.Series{
			{Name: "Median", Unit: "h", Values: r.CycleP50},
			{Name: "90th percentile", Unit: "h", Values: r.CycleP90},
		}},
		{ID: "c5", Title: "Waiting on review", Sub: "hours from open to first review by someone else", Days: r.Days, Series: []metrics.Series{
			{Name: "Your PRs wait", Unit: "h", Values: r.FirstReviewWait},
			{Name: "You make others wait", Unit: "h", Values: r.ReviewLatency},
		}},
		{ID: "c6", Title: "Lines changed per day", Sub: "merged PRs, additions plus deletions", Days: r.Days, Series: []metrics.Series{
			{Name: "Authored", Unit: " lines", Values: r.SrcLines},
			{Name: "Raw (incl. generated)", Unit: " lines", Values: r.RawLines},
		}},
		{ID: "c7", Title: "Median PR size", Sub: "authored lines per merged PR — smaller reviews move faster", Days: r.Days, Fill: true, Series: []metrics.Series{
			{Name: "Median size", Unit: " lines", Values: r.MedianPRSize},
		}},
	}
}
