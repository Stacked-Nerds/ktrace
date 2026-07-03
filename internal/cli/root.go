package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	ktraceerrors "github.com/Stacked-Nerds/ktrace/pkg/errors"
)

const version = "0.1.0"

var (
	kubeconfig  string
	kubeContext string
	namespace   string
	jsonOutput  bool
)

// Execute runs the ktrace CLI.
func Execute() error {
	return newRootCmd().Execute()
}

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ktrace [resource-type] [name]",
		Short: "Understand the story behind your Kubernetes resources",
		Long: `ktrace collects related Kubernetes resources, builds a chronological
timeline, and explains why resources are failing.

Examples:
  ktrace deployment frontend
  ktrace pod nginx -n production
  ktrace namespace production
  ktrace deployment frontend --json`,
		Version: version,
		Args:    cobra.ExactArgs(2),
		RunE:    runTrace,
	}

	cmd.SetOut(os.Stdout)
	cmd.SetErr(os.Stderr)

	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "Kubernetes namespace")
	cmd.Flags().StringVar(&kubeconfig, "kubeconfig", "", "Path to kubeconfig file")
	cmd.Flags().StringVar(&kubeContext, "context", "", "Kubeconfig context name")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output collected resource graph as JSON")

	return cmd
}

func runTrace(cmd *cobra.Command, args []string) error {
	kind := args[0]
	name := args[1]

	ns := namespace
	if ns == "" {
		defaultNS, err := resolveDefaultNamespace()
		if err != nil {
			return err
		}
		ns = defaultNS
	}

	graph, err := collect(kind, name, ns)
	if err != nil {
		return handleError(err)
	}

	if jsonOutput {
		return writeJSON(cmd.OutOrStdout(), graph)
	}
	return writeSummary(cmd.OutOrStdout(), graph)
}

func handleError(err error) error {
	if ktraceerrors.IsInvalidArgs(err) || ktraceerrors.IsUnsupportedKind(err) {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(2)
	}
	return err
}
