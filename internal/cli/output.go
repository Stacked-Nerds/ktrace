package cli

import (
	"context"
	"fmt"
	"io"

	"github.com/Stacked-Nerds/ktrace/internal/collector"
	"github.com/Stacked-Nerds/ktrace/internal/kubernetes"
	"github.com/Stacked-Nerds/ktrace/pkg/models"
	"github.com/Stacked-Nerds/ktrace/pkg/utils"
)

var (
	newClientFn = func(opts kubernetes.Options) (*kubernetes.Client, error) {
		return kubernetes.New(opts)
	}
	newOrchestratorFn = func(client *kubernetes.Client) *collector.Orchestrator {
		return collector.NewOrchestrator(client)
	}
)

func resolveDefaultNamespace() (string, error) {
	return kubernetes.DefaultNamespace(kubernetes.Options{
		Kubeconfig: kubeconfig,
		Context:    kubeContext,
	})
}

func collect(kind, name, ns string) (*models.ResourceGraph, error) {
	client, err := newClientFn(kubernetes.Options{
		Kubeconfig: kubeconfig,
		Context:    kubeContext,
		Namespace:  ns,
	})
	if err != nil {
		return nil, fmt.Errorf("connect to cluster: %w", err)
	}

	orch := newOrchestratorFn(client)
	return orch.Collect(context.Background(), kind, name, ns)
}

func writeSummary(w io.Writer, graph *models.ResourceGraph) error {
	sep := "━━━━━━━━━━━━━━━━━━━━━━━━━━"

	fmt.Fprintln(w, sep)
	fmt.Fprintf(w, "%s: %s\n", graph.Root.Kind, graph.Root.Name)
	if graph.Root.Namespace != "" {
		fmt.Fprintf(w, "Namespace: %s\n", graph.Root.Namespace)
	}
	fmt.Fprintln(w, sep)
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Collected:")
	fmt.Fprintf(w, "  ReplicaSets:  %d\n", graph.Count("ReplicaSet"))
	fmt.Fprintf(w, "  Pods:         %d\n", graph.Count("Pod"))
	fmt.Fprintf(w, "  Events:       %d\n", len(graph.Events))
	fmt.Fprintf(w, "  PVCs:         %d\n", graph.Count("PersistentVolumeClaim"))
	fmt.Fprintf(w, "  PVs:          %d\n", graph.Count("PersistentVolume"))
	fmt.Fprintf(w, "  Nodes:        %d\n", graph.Count("Node"))
	fmt.Fprintf(w, "  Services:     %d\n", graph.Count("Service"))
	fmt.Fprintf(w, "  Deployments:  %d\n", graph.Count("Deployment"))
	fmt.Fprintln(w)

	recent := recentEvents(graph.Events, 5)
	if len(recent) > 0 {
		fmt.Fprintln(w, "Recent Events:")
		for _, ev := range recent {
			ts := ev.Timestamp.Format("15:04")
			source := fmt.Sprintf("%s/%s", utils.NormalizeKind(ev.Source.Kind), ev.Source.Name)
			msg := utils.Truncate(ev.Message, 60)
			fmt.Fprintf(w, "  %s  %-7s  %-18s  %s  %s\n", ts, ev.Type, ev.Reason, source, msg)
		}
		fmt.Fprintln(w)
	}

	fmt.Fprintln(w, "(Timeline and root cause analysis coming in Phase 2)")
	return nil
}

func recentEvents(events []models.TimelineEvent, n int) []models.TimelineEvent {
	if len(events) == 0 {
		return nil
	}
	// events are sorted ascending; show most recent
	start := len(events) - n
	if start < 0 {
		start = 0
	}
	out := make([]models.TimelineEvent, len(events[start:]))
	copy(out, events[start:])
	// reverse for display (most recent first)
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out
}
