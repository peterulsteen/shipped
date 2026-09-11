package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"time"

	"github.com/peterulsteen/shipped/internal/config"
	"github.com/peterulsteen/shipped/internal/gh"
	"github.com/peterulsteen/shipped/internal/metrics"
	"github.com/peterulsteen/shipped/internal/render"
	"github.com/peterulsteen/shipped/internal/store"
	"github.com/spf13/cobra"
)

// windowDays is how wide a slice of time each search covers. Narrow enough that
// an active account stays under GitHub's 1000-result cap per query.
const windowDays = 7

// backfillYears is how far back --backfill reaches: an ordinary account's
// working history, without an unbounded crawl.
const backfillYears = 3

var collectCmd = &cobra.Command{
	Use:   "collect",
	Short: "Fetch pull requests and reviews from GitHub",
	RunE: func(cmd *cobra.Command, _ []string) error {
		c, err := loadOrSetup()
		if err != nil {
			return err
		}
		applyScopeFlags(cmd, c)

		client, err := gh.New()
		if err != nil {
			return err
		}
		login := c.Author
		if login == "" {
			if login, err = client.Viewer(); err != nil {
				return err
			}
		}

		s, err := store.Load(c.DataFile())
		if err != nil {
			return err
		}

		since, err := resolveSince(cmd, s)
		if err != nil {
			return err
		}
		today := time.Now()
		scope := gh.Scope{Orgs: c.Orgs, Repos: c.Repos}
		cls := store.NewClassifier(c.GeneratedPaths, c.GeneratedSuffixes, c.BigFileLines)
		progress := func(msg string) { fmt.Fprintln(os.Stderr, msg) }

		fmt.Fprintf(os.Stderr, "%s: fetching %s..%s\n",
			login, since.Format(time.DateOnly), today.Format(time.DateOnly))

		total := 0
		for _, role := range []gh.Role{gh.Authored, gh.Reviewed} {
			fmt.Fprintf(os.Stderr, "\n%s:\n", roleLabel(role))
			for cursor := since; !cursor.After(today); cursor = cursor.AddDate(0, 0, windowDays) {
				end := cursor.AddDate(0, 0, windowDays-1)
				if end.After(today) {
					end = today
				}
				prs, err := client.Search(role, login, scope, cursor, end, cls, progress)
				if err != nil {
					return err
				}
				total += s.Merge(role, prs)
			}
		}

		now := time.Now()
		s.LastRun, s.Login = &now, login
		if err := s.Save(c.DataFile()); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "\nupserted %d · %d authored, %d reviewed · %s\n",
			total, len(s.Authored), len(s.Reviewed), c.DataFile())

		if auto, _ := cmd.Flags().GetBool("render"); auto {
			return renderDashboard(c, s, true)
		}
		return nil
	},
}

func roleLabel(r gh.Role) string {
	if r == gh.Reviewed {
		return "reviewed by you"
	}
	return "authored by you"
}

// resolveSince picks the start date: an explicit flag, a full backfill, the last
// run (with a day of overlap so a merge that landed since is picked up), or a
// sensible first-run window.
func resolveSince(cmd *cobra.Command, s *store.Store) (time.Time, error) {
	if v, _ := cmd.Flags().GetString("since"); v != "" {
		return time.Parse(time.DateOnly, v)
	}
	if full, _ := cmd.Flags().GetBool("backfill"); full {
		return time.Now().AddDate(-backfillYears, 0, 0), nil
	}
	if s.LastRun != nil {
		return s.LastRun.AddDate(0, 0, -1), nil
	}
	return time.Now().AddDate(0, 0, -90), nil
}

func loadOrSetup() (*config.Config, error) {
	if !config.Exists() {
		fmt.Fprintln(os.Stderr, "No config yet — running setup.")
		c, err := runSetup(os.Stdin, os.Stderr)
		if err != nil {
			return nil, err
		}
		if err := c.Save(); err != nil {
			return nil, err
		}
		fmt.Fprintf(os.Stderr, "\nWrote %s\n\n", config.File())
		return c, nil
	}
	return config.Load()
}

func applyScopeFlags(cmd *cobra.Command, c *config.Config) {
	if v, _ := cmd.Flags().GetStringSlice("org"); len(v) > 0 {
		c.Orgs = v
	}
	if v, _ := cmd.Flags().GetStringSlice("repo"); len(v) > 0 {
		c.Repos = v
	}
	if v, _ := cmd.Flags().GetString("author"); v != "" {
		c.Author = v
	}
}

func renderDashboard(c *config.Config, s *store.Store, openIt bool) error {
	authored := make([]gh.PullRequest, 0, len(s.Authored))
	for _, pr := range s.Authored {
		authored = append(authored, pr)
	}
	reviewed := make([]gh.PullRequest, 0, len(s.Reviewed))
	for _, pr := range s.Reviewed {
		reviewed = append(reviewed, pr)
	}
	login := s.Login
	if login == "" {
		login = c.Author
	}
	report := metrics.Build(time.Now(), login, authored, reviewed, c.WindowDays, c.RollingDays, c.WorkdayStart, c.WorkdayEnd)
	if err := render.Page(c.DashboardFile(), login, report); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "wrote %s\n", c.DashboardFile())
	if openIt {
		return openInBrowser(c.DashboardFile())
	}
	return nil
}

func openInBrowser(path string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", path)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", path)
	default:
		cmd = exec.Command("xdg-open", path)
	}
	return cmd.Start()
}

func init() {
	collectCmd.Flags().String("since", "", "collect from this date (YYYY-MM-DD)")
	collectCmd.Flags().Bool("backfill", false, "collect the full history")
	collectCmd.Flags().Bool("render", false, "render and open the dashboard when done")
	collectCmd.Flags().StringSlice("org", nil, "limit to these organizations")
	collectCmd.Flags().StringSlice("repo", nil, "limit to these repositories (owner/name)")
	collectCmd.Flags().String("author", "", "measure this user instead of you")
	root.AddCommand(collectCmd)
}
