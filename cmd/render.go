package cmd

import (
	"fmt"

	"github.com/peterulsteen/shipped/internal/config"
	"github.com/peterulsteen/shipped/internal/store"
	"github.com/spf13/cobra"
)

var renderCmd = &cobra.Command{
	Use:   "render",
	Short: "Build the dashboard from already-collected data",
	RunE: func(cmd *cobra.Command, _ []string) error {
		c, s, err := loadData()
		if err != nil {
			return err
		}
		if v, _ := cmd.Flags().GetInt("window"); v > 0 {
			c.WindowDays = v
		}
		open, _ := cmd.Flags().GetBool("open")
		return renderDashboard(c, s, open)
	},
}

var openCmd = &cobra.Command{
	Use:   "open",
	Short: "Render and open the dashboard in your browser",
	RunE: func(*cobra.Command, []string) error {
		c, s, err := loadData()
		if err != nil {
			return err
		}
		return renderDashboard(c, s, true)
	},
}

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Show where configuration and data live",
	RunE: func(*cobra.Command, []string) error {
		c, err := config.Load()
		if err != nil {
			return err
		}
		fmt.Printf("config     %s\n", config.File())
		fmt.Printf("data       %s\n", c.DataFile())
		fmt.Printf("dashboard  %s\n", c.DashboardFile())
		fmt.Printf("\nauthor     %s\n", orDefault(c.Author, "(the authenticated user)"))
		fmt.Printf("orgs       %s\n", orDefault(join(c.Orgs), "(all of GitHub)"))
		fmt.Printf("repos      %s\n", orDefault(join(c.Repos), "(all)"))
		return nil
	},
}

func loadData() (*config.Config, *store.Store, error) {
	c, err := config.Load()
	if err != nil {
		return nil, nil, err
	}
	s, err := store.Load(c.DataFile())
	if err != nil {
		return nil, nil, err
	}
	if len(s.Authored) == 0 && len(s.Reviewed) == 0 {
		return nil, nil, fmt.Errorf("no data yet — run `shipped collect` first")
	}
	return c, s, nil
}

func orDefault(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}

func join(v []string) string {
	out := ""
	for i, s := range v {
		if i > 0 {
			out += ", "
		}
		out += s
	}
	return out
}

func init() {
	renderCmd.Flags().Int("window", 0, "days to display (default from config)")
	renderCmd.Flags().Bool("open", false, "open in the browser when done")
	root.AddCommand(renderCmd, openCmd, configCmd)
}
