package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/peterulsteen/shipped/internal/config"
	"github.com/peterulsteen/shipped/internal/gh"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Set up the config file",
	Long: `Discovers your GitHub login and asks which organizations to measure.
Every answer is optional -- with no organizations, shipped measures all of your
GitHub activity.`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		force, _ := cmd.Flags().GetBool("force")
		if config.Exists() && !force {
			return fmt.Errorf("%s already exists (use --force to overwrite)", config.File())
		}
		c, err := runSetup(os.Stdin, os.Stdout)
		if err != nil {
			return err
		}
		if err := c.Save(); err != nil {
			return err
		}
		fmt.Printf("\nWrote %s\n\nNext:  shipped collect\n", config.File())
		return nil
	},
}

// runSetup prompts for the few things that cannot be discovered.
func runSetup(in *os.File, out *os.File) (*config.Config, error) {
	c, err := config.Load()
	if err != nil {
		return nil, err
	}

	client, err := gh.New()
	if err != nil {
		return nil, err
	}
	login, err := client.Viewer()
	if err != nil {
		return nil, err
	}
	c.Author = login
	fmt.Fprintf(out, "Measuring GitHub user: %s\n\n", login)

	fmt.Fprint(out, "Limit to specific organizations? Comma-separated, or blank for all of GitHub.\n> ")
	reader := bufio.NewReader(in)
	line, _ := reader.ReadString('\n')
	for _, o := range strings.Split(strings.TrimSpace(line), ",") {
		if o = strings.TrimSpace(o); o != "" {
			c.Orgs = append(c.Orgs, o)
		}
	}

	if len(c.Orgs) == 0 {
		fmt.Fprintln(out, "\nScope: all of GitHub.")
	} else {
		fmt.Fprintf(out, "\nScope: %s\n", strings.Join(c.Orgs, ", "))
	}
	return c, nil
}

func init() {
	initCmd.Flags().Bool("force", false, "overwrite an existing config")
	root.AddCommand(initCmd)
}
