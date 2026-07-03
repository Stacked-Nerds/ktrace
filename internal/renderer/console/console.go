package console

import (
	"fmt"
	"io"
	"strings"

	"github.com/Stacked-Nerds/ktrace/internal/explain"
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
	fmt.Fprintln(w, sep)
	fmt.Fprintln(w)

	if len(result.Findings) > 0 {
		fmt.Fprintln(w, "Critical Issues:")
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
			fmt.Fprintf(w, "  %s [%s] %s — %s\n", icon, f.Condition, src, f.Summary)
			if opts.Verbose && f.Explanation != "" {
				fmt.Fprintf(w, "      %s\n", utils.Truncate(f.Explanation, 120))
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
				line = fmt.Sprintf("%s — %s", entry.Title, utils.Truncate(entry.Detail, 80))
			}
			fmt.Fprintf(w, "  %s  %s\n", ts, line)
		}
		fmt.Fprintln(w)
	}

	if result.RootCause != nil {
		fmt.Fprintln(w, sep)
		fmt.Fprintln(w, "Root Cause")
		fmt.Fprintf(w, "%s\n", result.RootCause.Summary)
		if result.RootCause.Explanation != "" {
			fmt.Fprintf(w, "%s\n", utils.Truncate(result.RootCause.Explanation, 200))
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
			fmt.Fprintf(w, "  %s\n", rec)
		}
		fmt.Fprintln(w)
	} else if result.Status == models.StatusHealthy {
		fmt.Fprintln(w, "No failure conditions detected.")
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
	kinds := []string{"Deployment", "ReplicaSet", "Pod", "PersistentVolumeClaim", "PersistentVolume", "Node", "Service"}
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
