package cli

import (
	"context"
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
	return traceContext(context.Background(), kind, name, ns)
}

func traceContext(ctx context.Context, kind, name, ns string) (*models.TraceResult, error) {
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	client, err := newClientFn(kubernetes.Options{
		Kubeconfig: kubeconfig,
		Context:    kubeContext,
		Namespace:  ns,
	})
	if err != nil {
		return nil, err
	}

	orch := newOrchestratorFn(client)
	orch.SetMaxResources(maxResources)
	graph, err := orch.Collect(ctx, kind, name, ns)
	if err != nil {
		return nil, err
	}
	collector.CollectFailureLogs(ctx, client, graph, collector.LogOptions{
		Current:      includeLogs,
		Previous:     previousLogs,
		TailLines:    logTail,
		SinceSeconds: int64(logSince.Seconds()),
	})

	return analyzeFn(graph), nil
}

func writeReport(w io.Writer, result *models.TraceResult) error {
	opts := console.DefaultOptions()
	opts.Verbose = verbose
	opts.ShowCollected = showCollected
	return console.Render(w, result, opts)
}
