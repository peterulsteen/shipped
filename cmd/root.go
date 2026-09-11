// Package cmd wires the command line.
package cmd

import (
	"fmt"
	"os"
	"runtime/debug"

	"github.com/spf13/cobra"
)

// Version is set at build time by the release process.
var Version = "dev"

var root = &cobra.Command{
	Use:   "shipped",
	Short: "A personal mirror for your GitHub work — what you ship, and what you review",
	Long: `shipped collects your pull requests and the reviews you give others,
then renders a self-contained HTML dashboard. Nothing runs in the background,
no server, no account, no data leaves your machine.`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

// Execute runs the CLI.
func Execute() {
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func init() {
	root.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print the version",
		Run:   func(*cobra.Command, []string) { fmt.Println("shipped", version()) },
	})
}

// version prefers the release stamp, then the module version that `go install
// ...@vX.Y.Z` records, so a go-installed binary does not just say "dev".
func version() string {
	if Version != "dev" {
		return Version
	}
	if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
		return bi.Main.Version
	}
	return Version
}
