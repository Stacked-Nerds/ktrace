package engine

import (
	"time"

	"github.com/Stacked-Nerds/ktrace/internal/analyzer"
	"github.com/Stacked-Nerds/ktrace/internal/correlator"
	"github.com/Stacked-Nerds/ktrace/internal/explain"
	"github.com/Stacked-Nerds/ktrace/internal/timeline"
	"github.com/Stacked-Nerds/ktrace/pkg/models"
)

// Analyze runs the full Phase 2 pipeline on a collected resource graph.
func Analyze(graph *models.ResourceGraph) *models.TraceResult {
	if graph == nil {
		return nil
	}

	findings := analyzer.Analyze(graph)
	tl := timeline.Build(graph)

	return &models.TraceResult{
		Root:        graph.Root,
		Status:      explain.Status(findings),
		Graph:       graph,
		Edges:       correlator.Correlate(graph),
		Timeline:    tl,
		Findings:    findings,
		RootCause:   explain.RootCause(findings),
		CollectedAt: time.Now().UTC(),
	}
}
