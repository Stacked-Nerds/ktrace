package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/spf13/cobra"

	ktraceerrors "github.com/Stacked-Nerds/ktrace/pkg/errors"
	"github.com/Stacked-Nerds/ktrace/pkg/models"
)

const version = "0.3.0"

const (
	exitOK         = 0
	exitFindings   = 3
	exitUnknown    = 4
	exitUsage      = 2
	exitConnection = 5
)

var (
	kubeconfig    string
	kubeContext   string
	namespace     string
	jsonOutput    bool
	verbose       bool
	showCollected bool
	includeLogs   bool
	previousLogs  bool
	logTail       int64
	logSince      time.Duration
	timeout       time.Duration
	maxResources  int
	includeRaw    bool
	explainOnly   bool
	lastExitCode  int
)

// Execute runs the ktrace CLI.
func Execute() error {
	lastExitCode = exitOK
	ctx, stop := signal.NotifyContext(
		contextBackground(),
		os.Interrupt,
	)
	defer stop()

	if err := newRootCmd().ExecuteContext(ctx); err != nil {
		return err
	}
	if lastExitCode != exitOK {
		return &exitStatusError{code: lastExitCode}
	}
	return nil
}

var contextBackground = func() context.Context { return context.Background() }

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ktrace [resource-type] [name]",
		Short: "Understand the story behind your Kubernetes resources",
		Long: `ktrace collects related Kubernetes resources, builds a chronological
timeline, detects failure conditions, and explains the most likely root cause.

Examples:
  ktrace deployment frontend
  ktrace pod nginx -n production
  ktrace statefulset database -n production
  ktrace job migration -n production --previous-logs
  ktrace deployment frontend --json
  ktrace deployment frontend --explain
  ktrace deployment frontend -v --show-collected`,
		Version: version,
		Args:    validateArgs,
		RunE:    runTrace,
		SilenceUsage: true,
	}

	cmd.SetOut(os.Stdout)
	cmd.SetErr(os.Stderr)

	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "Kubernetes namespace")
	cmd.Flags().StringVar(&kubeconfig, "kubeconfig", "", "Path to kubeconfig file")
	cmd.Flags().StringVar(&kubeContext, "context", "", "Kubeconfig context name")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output stable redacted summary JSON")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Show detailed explanations and all recommendations")
	cmd.Flags().BoolVar(&showCollected, "show-collected", false, "Include collected resource counts")
	cmd.Flags().BoolVar(&includeLogs, "logs", false, "Include bounded current logs for failing containers")
	cmd.Flags().BoolVar(&previousLogs, "previous-logs", false, "Include bounded previous logs for restarted containers")
	cmd.Flags().Int64Var(&logTail, "log-tail", 100, "Maximum log lines per failing container")
	cmd.Flags().DurationVar(&logSince, "since", 30*time.Minute, "Only include logs newer than this duration")
	cmd.Flags().DurationVar(&timeout, "timeout", 30*time.Second, "Overall collection timeout")
	cmd.Flags().IntVar(&maxResources, "max-resources", 1000, "Maximum Kubernetes resources to retain")
	cmd.Flags().BoolVar(&includeRaw, "include-raw", false, "Include raw Kubernetes objects in JSON output")
	cmd.Flags().BoolVar(&explainOnly, "explain", false, "Show only diagnosis, evidence, and next actions")

	return cmd
}

func validateArgs(_ *cobra.Command, args []string) error {
	if len(args) != 2 {
		return ktraceerrors.InvalidArgs(
			fmt.Sprintf("expected resource type and name, got %d argument(s)", len(args)),
		)
	}
	return nil
}

func runTrace(cmd *cobra.Command, args []string) error {
	kind := args[0]
	name := args[1]
	if includeRaw && !jsonOutput {
		return handleError(ktraceerrors.InvalidArgs("--include-raw requires --json"))
	}
	if jsonOutput && explainOnly {
		return handleError(ktraceerrors.InvalidArgs("--json and --explain are mutually exclusive"))
	}
	if logTail <= 0 {
		return handleError(ktraceerrors.InvalidArgs("--log-tail must be greater than zero"))
	}
	if maxResources <= 0 {
		return handleError(ktraceerrors.InvalidArgs("--max-resources must be greater than zero"))
	}
	if timeout < 0 || logSince < 0 {
		return handleError(ktraceerrors.InvalidArgs("--timeout and --since cannot be negative"))
	}

	ns := namespace
	if ns == "" {
		defaultNS, err := resolveDefaultNamespace()
		if err != nil {
			return err
		}
		ns = defaultNS
	}

	result, err := traceContext(cmd.Context(), kind, name, ns)
	if err != nil {
		return handleError(err)
	}

	if jsonOutput {
		if err := writeJSON(cmd.OutOrStdout(), result, includeRaw); err != nil {
			return err
		}
	} else if explainOnly {
		if err := writeExplanation(cmd.OutOrStdout(), result); err != nil {
			return err
		}
	} else if err := writeReport(cmd.OutOrStdout(), result); err != nil {
		return err
	}

	switch result.Status {
	case models.StatusFailed, models.StatusDegraded:
		lastExitCode = exitFindings
	case models.StatusUnknown:
		lastExitCode = exitUnknown
	default:
		lastExitCode = exitOK
	}
	return nil
}

func handleError(err error) error {
	if ktraceerrors.IsInvalidArgs(err) || ktraceerrors.IsUnsupportedKind(err) {
		return &exitStatusError{code: exitUsage, cause: err}
	}
	return err
}

type exitStatusError struct {
	code  int
	cause error
}

func (e *exitStatusError) Error() string {
	if e.cause != nil {
		return e.cause.Error()
	}
	switch e.code {
	case exitFindings:
		return "trace completed with findings"
	case exitUnknown:
		return "trace completed with partial evidence"
	default:
		return "ktrace exited with a non-zero status"
	}
}

func (e *exitStatusError) Unwrap() error { return e.cause }

// ExitCode maps command outcomes to stable process exit codes.
func ExitCode(err error) int {
	if err == nil {
		return exitOK
	}
	var status *exitStatusError
	if errors.As(err, &status) {
		return status.code
	}
	if ktraceerrors.IsInvalidArgs(err) || ktraceerrors.IsUnsupportedKind(err) {
		return exitUsage
	}
	return exitConnection
}
