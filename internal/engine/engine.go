package engine

import (
	"time"

	"github.com/Stacked-Nerds/ktrace/internal/analyzer"
	"github.com/Stacked-Nerds/ktrace/internal/correlator"
	"github.com/Stacked-Nerds/ktrace/internal/explain"
	"github.com/Stacked-Nerds/ktrace/internal/timeline"
	"github.com/Stacked-Nerds/ktrace/pkg/models"
)

// Analyze runs the evidence-driven diagnostic pipeline on a resource graph.
func Analyze(graph *models.ResourceGraph) *models.TraceResult {
	if graph == nil {
		return nil
	}

	findings := analyzer.Analyze(graph)
	tl := timeline.Build(graph)
	edges := correlator.Correlate(graph)
	diagnosis := explain.Diagnose(graph.Root, findings, edges, tl)
	status := explain.Status(findings)
	if graph.Partial {
		status = models.StatusUnknown
	}

	return &models.TraceResult{
		Root:        graph.Root,
		Status:      status,
		Graph:       graph,
		Edges:       edges,
		Timeline:    tl,
		Findings:    findings,
		RootCause:   diagnosis.RootCause,
		Diagnosis:   diagnosis,
		Partial:     graph.Partial,
		Warnings:    append([]string(nil), graph.Warnings...),
		CollectedAt: time.Now().UTC(),
	}
}
