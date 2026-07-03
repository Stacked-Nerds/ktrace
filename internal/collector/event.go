package collector

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/Stacked-Nerds/ktrace/internal/kubernetes"
	"github.com/Stacked-Nerds/ktrace/pkg/models"
)

type eventCollector struct{}

func (c *eventCollector) Kind() string { return "Event" }

func (c *eventCollector) Collect(ctx context.Context, client *kubernetes.Client, ref models.ResourceRef) ([]models.CollectedResource, error) {
	events, err := c.collectForGraph(ctx, client, ref.Namespace, nil)
	if err != nil {
		return nil, err
	}
	_ = events
	return nil, nil
}

func (c *eventCollector) collectForGraph(ctx context.Context, client *kubernetes.Client, namespace string, graph *models.ResourceGraph) ([]models.TimelineEvent, error) {
	fieldSelectors := buildInvolvedObjectSelectors(graph)
	if len(fieldSelectors) == 0 && graph != nil && graph.Root.Name != "" {
		fieldSelectors = []string{fmt.Sprintf("involvedObject.name=%s", graph.Root.Name)}
	}

	seen := make(map[string]bool)
	timeline := make([]models.TimelineEvent, 0)

	namespaces := eventNamespaces(namespace, graph)
	for _, ns := range namespaces {
		for _, fieldSelector := range fieldSelectors {
			list, err := client.Clientset.CoreV1().Events(ns).List(ctx, metav1.ListOptions{
				FieldSelector: fieldSelector,
			})
			if err != nil {
				return nil, fmt.Errorf("list events in namespace %q: %w", ns, err)
			}
			appendEvents(&timeline, seen, list.Items)
		}

		if graph != nil && len(graph.Resources) > 0 {
			list, err := client.Clientset.CoreV1().Events(ns).List(ctx, metav1.ListOptions{})
			if err != nil {
				return nil, fmt.Errorf("list namespace events in %q: %w", ns, err)
			}

			targetUIDs := graphUIDs(graph)
			for i := range list.Items {
				ev := &list.Items[i]
				if !matchesGraphEvent(ev, graph, targetUIDs) {
					continue
				}
				appendEvent(&timeline, seen, ev)
			}
		}
	}

	return timeline, nil
}

// eventNamespaces returns namespaces to search for events. Cluster-scoped object
// events (Node, PersistentVolume) are stored in the default namespace.
func eventNamespaces(workloadNamespace string, graph *models.ResourceGraph) []string {
	namespaces := []string{workloadNamespace}
	if graph == nil {
		return namespaces
	}

	needsDefault := graph.Count("Node") > 0 || graph.Count("PersistentVolume") > 0
	if needsDefault && workloadNamespace != metav1.NamespaceDefault {
		namespaces = append(namespaces, metav1.NamespaceDefault)
	}
	return namespaces
}

func appendEvents(timeline *[]models.TimelineEvent, seen map[string]bool, events []corev1.Event) {
	for i := range events {
		appendEvent(timeline, seen, &events[i])
	}
}

func appendEvent(timeline *[]models.TimelineEvent, seen map[string]bool, ev *corev1.Event) {
	key := string(ev.UID)
	if key == "" {
		key = fmt.Sprintf("%s/%s/%s/%s", ev.InvolvedObject.Kind, ev.InvolvedObject.Name, ev.Reason, ev.Message)
	}
	if seen[key] {
		return
	}
	seen[key] = true

	ts := eventTimestamp(ev)
	if ts.IsZero() {
		return
	}

	*timeline = append(*timeline, models.TimelineEvent{
		Timestamp: ts,
		Source: models.ResourceRef{
			Kind:      ev.InvolvedObject.Kind,
			Name:      ev.InvolvedObject.Name,
			Namespace: ev.InvolvedObject.Namespace,
			UID:       string(ev.InvolvedObject.UID),
		},
		Type:    ev.Type,
		Reason:  ev.Reason,
		Message: ev.Message,
		Count:   ev.Count,
	})
}

func eventTimestamp(ev *corev1.Event) time.Time {
	if !ev.LastTimestamp.IsZero() {
		return ev.LastTimestamp.Time
	}
	if !ev.EventTime.IsZero() {
		return ev.EventTime.Time
	}
	if !ev.FirstTimestamp.IsZero() {
		return ev.FirstTimestamp.Time
	}
	return ev.CreationTimestamp.Time
}

func buildInvolvedObjectSelectors(graph *models.ResourceGraph) []string {
	if graph == nil {
		return nil
	}
	selectors := make([]string, 0)
	for _, resources := range graph.Resources {
		for _, r := range resources {
			if r.Ref.Name != "" {
				selectors = append(selectors, fmt.Sprintf("involvedObject.name=%s", r.Ref.Name))
			}
		}
	}
	return selectors
}

func graphUIDs(graph *models.ResourceGraph) map[string]bool {
	uids := make(map[string]bool)
	for _, resources := range graph.Resources {
		for _, r := range resources {
			if r.Metadata.UID != "" {
				uids[r.Metadata.UID] = true
			}
		}
	}
	return uids
}

func matchesGraphEvent(ev *corev1.Event, graph *models.ResourceGraph, uids map[string]bool) bool {
	if string(ev.InvolvedObject.UID) != "" && uids[string(ev.InvolvedObject.UID)] {
		return true
	}
	for _, resources := range graph.Resources {
		for _, r := range resources {
			if r.Ref.Kind == ev.InvolvedObject.Kind && r.Ref.Name == ev.InvolvedObject.Name {
				return true
			}
		}
	}
	if graph.Root.Kind == ev.InvolvedObject.Kind && graph.Root.Name == ev.InvolvedObject.Name {
		return true
	}
	return false
}
