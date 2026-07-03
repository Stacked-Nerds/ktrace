package cli

import (
	"context"
	"fmt"
	"io"

	"github.com/Stacked-Nerds/ktrace/internal/collector"
	"github.com/Stacked-Nerds/ktrace/internal/engine"
	"github.com/Stacked-Nerds/ktrace/internal/kubernetes"
	"github.com/Stacked-Nerds/ktrace/internal/renderer/console"
	"github.com/Stacked-Nerds/ktrace/pkg/models"
)

var (
	newClientFn = func(opts kubernetes.Options) (*kubernetes.Client, error) {
		return kubernetes.New(opts)
	}
	newOrchestratorFn = func(client *kubernetes.Client) *collector.Orchestrator {
		return collector.NewOrchestrator(client)
	}
	analyzeFn = engine.Analyze
)

func resolveDefaultNamespace() (string, error) {
	return kubernetes.DefaultNamespace(kubernetes.Options{
		Kubeconfig: kubeconfig,
		Context:    kubeContext,
	})
}

func trace(kind, name, ns string) (*models.TraceResult, error) {
	client, err := newClientFn(kubernetes.Options{
		Kubeconfig: kubeconfig,
		Context:    kubeContext,
		Namespace:  ns,
	})
	if err != nil {
		return nil, fmt.Errorf("connect to cluster: %w", err)
	}

	orch := newOrchestratorFn(client)
	graph, err := orch.Collect(context.Background(), kind, name, ns)
	if err != nil {
		return nil, err
	}

	return analyzeFn(graph), nil
}

func writeReport(w io.Writer, result *models.TraceResult) error {
	opts := console.DefaultOptions()
	opts.Verbose = verbose
	opts.ShowCollected = showCollected
	return console.Render(w, result, opts)
}
