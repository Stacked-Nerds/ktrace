package cli

import (
	"fmt"
	"io"

	"github.com/Stacked-Nerds/ktrace/internal/explain"
	"github.com/Stacked-Nerds/ktrace/internal/redact"
	"github.com/Stacked-Nerds/ktrace/pkg/models"
)

func writeExplanation(w io.Writer, result *models.TraceResult) error {
	if result == nil {
		return fmt.Errorf("nil trace result")
	}
	fmt.Fprintf(w, "%s/%s: %s\n", result.Root.Kind, result.Root.Name, result.Status)
	if result.Partial {
		fmt.Fprintln(w, "Evidence: partial")
	}
	if result.Diagnosis == nil || result.Diagnosis.RootCause == nil {
		fmt.Fprintln(w, "No failure conditions detected.")
		if result.Diagnosis != nil && len(result.Diagnosis.Context) > 0 {
			fmt.Fprintln(w, "Context:")
			for _, finding := range result.Diagnosis.Context {
				fmt.Fprintf(w, "  %s\n", safeExplanationText(finding.Summary))
			}
		}
		return nil
	}

	root := result.Diagnosis.RootCause
	summary := safeExplanationText(root.Summary)
	fmt.Fprintf(w, "Diagnosis: %s\n", summary)
	fmt.Fprintf(w, "Confidence: %.0f%%\n", result.Diagnosis.Confidence*100)

	if len(result.Diagnosis.EvidenceChain) > 0 {
		fmt.Fprintln(w, "Evidence:")
		for _, step := range result.Diagnosis.EvidenceChain {
			line := step.Source.String()
			if step.Relation != "" {
				line += " --" + step.Relation + "-->"
			}
			if step.Condition != "" {
				line += " " + step.Condition
			}
			line = safeExplanationText(line)
			fmt.Fprintf(w, "  %s\n", line)
		}
	}

	recommendations := explain.Recommendations(result.Findings)
	if len(recommendations) > 0 {
		fmt.Fprintln(w, "Next actions:")
		for _, recommendation := range recommendations {
			recommendation = safeExplanationText(recommendation)
			fmt.Fprintf(w, "  %s\n", recommendation)
		}
	}
	return nil
}

func safeExplanationText(value string) string {
	value, _ = redact.Text(value)
	return value
}
