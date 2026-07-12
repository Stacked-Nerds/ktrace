package console

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/Stacked-Nerds/ktrace/internal/explain"
	"github.com/Stacked-Nerds/ktrace/internal/redact"
	"github.com/Stacked-Nerds/ktrace/pkg/models"
	"github.com/Stacked-Nerds/ktrace/pkg/utils"
)

const sep = "━━━━━━━━━━━━━━━━━━━━━━━━━━"

// Options controls console rendering verbosity.
type Options struct {
	Verbose       bool
	MaxTimeline   int
	MaxFindings   int
	ShowCollected bool
}

// DefaultOptions returns sensible defaults for interactive use.
func DefaultOptions() Options {
	return Options{
		MaxTimeline:   20,
		MaxFindings:   10,
		ShowCollected: false,
	}
}

// Render writes a human-readable trace report.
func Render(w io.Writer, result *models.TraceResult, opts Options) error {
	if result == nil {
		return fmt.Errorf("nil trace result")
	}

	fmt.Fprintln(w, sep)
	fmt.Fprintf(w, "%s: %s\n", result.Root.Kind, result.Root.Name)
	if result.Root.Namespace != "" {
		fmt.Fprintf(w, "Namespace: %s\n", result.Root.Namespace)
	}
	fmt.Fprintf(w, "Status: %s\n", result.Status)
	if result.Partial {
		fmt.Fprintln(w, "Evidence: Partial")
	}
	fmt.Fprintln(w, sep)
	fmt.Fprintln(w)

	if len(result.Warnings) > 0 {
		fmt.Fprintln(w, "Warnings:")
		for _, warning := range result.Warnings {
			fmt.Fprintf(w, "  %s\n", safeText(warning))
		}
		fmt.Fprintln(w)
	}

	if len(result.Findings) > 0 {
		fmt.Fprintln(w, "Findings:")
		limit := opts.MaxFindings
		if limit <= 0 {
			limit = len(result.Findings)
		}
		for i, f := range result.Findings {
			if i >= limit {
				fmt.Fprintf(w, "  ... and %d more\n", len(result.Findings)-limit)
				break
			}
			icon := severityIcon(f.Severity)
			src := formatSource(f.Source)
			fmt.Fprintf(w, "  %s [%s] %s — %s\n", icon, f.Condition, src, safeText(f.Summary))
			if opts.Verbose && f.Explanation != "" {
				fmt.Fprintf(w, "      %s\n", utils.Truncate(safeText(f.Explanation), 120))
			}
		}
		fmt.Fprintln(w)
	}

	if len(result.Timeline) > 0 {
		fmt.Fprintln(w, "Timeline:")
		limit := opts.MaxTimeline
		if limit <= 0 {
			limit = len(result.Timeline)
		}
		start := len(result.Timeline) - limit
		if start < 0 {
			start = 0
		}
		for _, entry := range result.Timeline[start:] {
			ts := entry.Timestamp.Format("15:04")
			line := entry.Title
			if entry.Detail != "" {
				line = fmt.Sprintf("%s — %s", entry.Title, utils.Truncate(safeText(entry.Detail), 80))
			}
			fmt.Fprintf(w, "  %s  %s\n", ts, line)
		}
		fmt.Fprintln(w)
	}

	if result.RootCause != nil {
		fmt.Fprintln(w, sep)
		fmt.Fprintln(w, "Root Cause")
		fmt.Fprintf(w, "%s\n", safeText(result.RootCause.Summary))
		if result.Diagnosis != nil {
			fmt.Fprintf(w, "Confidence: %.0f%%\n", result.Diagnosis.Confidence*100)
		}
		if result.RootCause.Explanation != "" {
			fmt.Fprintf(w, "%s\n", utils.Truncate(safeText(result.RootCause.Explanation), 200))
		}
		if opts.Verbose && result.Diagnosis != nil && len(result.Diagnosis.EvidenceChain) > 0 {
			fmt.Fprintln(w, "Evidence chain:")
			for _, step := range result.Diagnosis.EvidenceChain {
				fmt.Fprintf(w, "  %s", step.Source.String())
				if step.Relation != "" {
					fmt.Fprintf(w, " --%s-->", step.Relation)
				}
				if step.Condition != "" {
					fmt.Fprintf(w, " %s", step.Condition)
				}
				fmt.Fprintln(w)
			}
		}
		fmt.Fprintln(w, sep)
		fmt.Fprintln(w, "Recommendation")
		recs := explain.Recommendations(result.Findings)
		limit := 5
		if opts.Verbose {
			limit = len(recs)
		}
		for i, rec := range recs {
			if i >= limit {
				fmt.Fprintf(w, "  ... and %d more commands\n", len(recs)-limit)
				break
			}
			fmt.Fprintf(w, "  %s\n", safeText(rec))
		}
		fmt.Fprintln(w)
	} else if result.Status == models.StatusHealthy {
		fmt.Fprintln(w, "No failure conditions detected.")
		fmt.Fprintln(w)
	} else if result.Status == models.StatusUnknown {
		fmt.Fprintln(w, "Unable to determine health because required evidence was incomplete.")
		fmt.Fprintln(w)
	}

	if result.Graph != nil && len(result.Graph.Logs) > 0 {
		fmt.Fprintln(w, "Failure Logs (redacted):")
		for _, log := range result.Graph.Logs {
			label := "current"
			if log.Previous {
				label = "previous"
			}
			fmt.Fprintf(w, "  %s/%s [%s]\n", log.Pod.Name, log.Container, label)
			fmt.Fprintf(w, "    %s\n", strings.ReplaceAll(utils.Truncate(safeText(log.Content), 1200), "\n", "\n    "))
			if log.Truncated {
				fmt.Fprintln(w, "    …[log truncated]")
			}
		}
		fmt.Fprintln(w)
	}

	if opts.ShowCollected || opts.Verbose {
		renderCollected(w, result.Graph)
	}

	return nil
}

func renderCollected(w io.Writer, graph *models.ResourceGraph) {
	if graph == nil {
		return
	}
	fmt.Fprintln(w, "Collected:")
	kinds := make([]string, 0, len(graph.Resources))
	for kind := range graph.Resources {
		kinds = append(kinds, kind)
	}
	sort.Strings(kinds)
	for _, kind := range kinds {
		if n := graph.Count(kind); n > 0 {
			fmt.Fprintf(w, "  %-22s %d\n", kind+":", n)
		}
	}
	fmt.Fprintf(w, "  %-22s %d\n", "Events:", len(graph.Events))
	fmt.Fprintln(w)
}

func formatSource(ref models.ResourceRef) string {
	if ref.Namespace != "" {
		return fmt.Sprintf("%s/%s", utils.NormalizeKind(ref.Kind), ref.Name)
	}
	return fmt.Sprintf("%s/%s", utils.NormalizeKind(ref.Kind), ref.Name)
}

func severityIcon(s models.Severity) string {
	switch s {
	case models.SeverityCritical:
		return "[CRIT]"
	case models.SeverityHigh:
		return "[HIGH]"
	case models.SeverityMedium:
		return "[MED]"
	default:
		return "[LOW]"
	}
}

func safeText(value string) string {
	value, _ = redact.Text(value)
	return value
}

// RenderCompact writes a one-line status summary.
func RenderCompact(w io.Writer, result *models.TraceResult) {
	if result == nil {
		return
	}
	msg := string(result.Status)
	if result.RootCause != nil {
		msg = result.RootCause.Summary
	}
	fmt.Fprintf(w, "%s/%s [%s] %s\n", strings.ToLower(result.Root.Kind), result.Root.Name, result.Status, msg)
}
