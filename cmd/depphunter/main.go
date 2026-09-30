// Command depphunter opens an interactive map of the code base in the current directory.
package main

import (
	"context"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/sarumaj/depphunter-cli/internal/app"
	"github.com/sarumaj/depphunter-cli/internal/config"
	"github.com/sarumaj/depphunter-cli/internal/findings"
	"github.com/sarumaj/depphunter-cli/internal/graph"
	"github.com/sarumaj/depphunter-cli/internal/latest"
)

// version is set at release builds: -ldflags "-X main.version=v1.2.3".
var version = "dev"

// Implements: REQ-CLI-008, REQ-CLI-011
func main() {
	log.SetFlags(0)
	log.SetPrefix("depphunter: ")
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	err := newCommand().ExecuteContext(ctx)
	stop()
	if err != nil {
		// A failure is not part of the output anybody asked for, wherever the log
		// was going by then (logOutput).
		log.SetOutput(os.Stderr)
		log.Println(err)
		os.Exit(1)
	}
}

// logOutput is where depphunter's own log goes: stdout, like any other output of a
// command - except where stdout is already carrying an export with no file to go to,
// since "analyzed …" in the middle of a JSON graph is no use to anyone.
//
// Implements: REQ-CLI-009, REQ-CLI-010
func logOutput(settings config.Config) io.Writer {
	if settings.Export != "" && settings.Output == "" {
		return os.Stderr
	}
	return os.Stdout
}

// newCommand is the depphunter command: flags are declared by the config package,
// which layers them over the config files and environment (viper).
//
// Implements: REQ-CLI-001, REQ-CLI-002, REQ-CLI-003, REQ-CLI-005, REQ-CLI-006, REQ-CLI-007, REQ-CLI-008
func newCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "depphunter [path]",
		Short: "Browse a code base as an interactive isometric map",
		Long: `Opens an interactive map of the code base at path (default: the current
directory) in the browser, or writes the dependency graph with --export.

Settings come from, in increasing precedence: defaults, the user config
(<user config dir>/depphunter/config.yaml), the project config
(<path>/` + config.ProjectFile + ` or --config), DEPPHUNTER_* environment
variables, and flags.`,
		Example: `  depphunter                          # the current directory
  depphunter ~/src/app --watch        # keep the map in sync while you edit
  depphunter --findings trivy.json    # put what a scanner reported on the map
  depphunter --export html -o map.html`,
		Args:          cobra.MaximumNArgs(1),
		Version:       version,
		SilenceUsage:  true, // errors are about the input, not the syntax
		SilenceErrors: true, // main logs them
		RunE: func(command *cobra.Command, arguments []string) error {
			userDirectory := ""
			if d, err := os.UserConfigDir(); err == nil {
				userDirectory = filepath.Join(d, "depphunter")
			}
			settings, err := config.Load(command.Flags(), arguments, userDirectory)
			if err != nil {
				return err
			}
			log.SetOutput(logOutput(settings))
			return app.Run(command.Context(), settings)
		},
	}
	command.SetVersionTemplate("depphunter {{.Version}}\n")
	command.Flags().SortFlags = false
	config.RegisterFlags(command.Flags())
	return command
}

// The cache lifetimes, documents, pinned and newLatest forward to the code that moved
// out of main, for the tests of this package that still use them by their old names.
const (
	findingsCacheTTL = app.FindingsCacheTTL
	linkCacheTTL     = app.LinkCacheTTL
)

func documents(g *graph.Graph) []string { return findings.Documents(g) }

func pinned(g *graph.Graph, private func(ecosystem, name string) bool) []findings.Package {
	return findings.Pinned(g, private)
}

func newLatest[T any](function func(T)) *latest.Runner[T] { return latest.New(function) }
