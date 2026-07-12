package engine

import (
	"testing"

	"github.com/Stacked-Nerds/ktrace/pkg/models"
)

func TestAnalyzeMarksPartialGraphUnknown(t *testing.T) {
	graph := models.NewResourceGraph(models.ResourceRef{Kind: "Deployment", Name: "api", Namespace: "prod"})
	graph.Partial = true
	graph.Warnings = []string{"events forbidden"}

	result := Analyze(graph)
	if result.Status != models.StatusUnknown {
		t.Fatalf("status = %q, want Unknown", result.Status)
	}
	if !result.Partial || len(result.Warnings) != 1 {
		t.Fatalf("partial metadata not propagated: %#v", result)
	}
}

func TestAnalyzeBuildsDiagnosis(t *testing.T) {
	graph := models.NewResourceGraph(models.ResourceRef{Kind: "Deployment", Name: "api", Namespace: "prod"})
	result := Analyze(graph)
	if result.Diagnosis == nil {
		t.Fatal("expected non-nil diagnosis")
	}
}
