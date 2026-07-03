package collector

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/api/errors"

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
		fieldSelectors = []string{buildFieldSelector(graph.Root.Kind, graph.Root.Name, graph.Root.Namespace)}
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
				if isOptionalNamespaceEventError(ns, namespace, err) {
					continue
				}
				return nil, fmt.Errorf("list events in namespace %q: %w", ns, err)
			}
			for i := range list.Items {
				ev := &list.Items[i]
				if graph != nil && !matchesGraphEvent(ev, graph, graphUIDs(graph)) {
					continue
				}
				appendEvent(&timeline, seen, ev)
			}
		}

		if graph != nil && len(graph.Resources) > 0 {
			list, err := client.Clientset.CoreV1().Events(ns).List(ctx, metav1.ListOptions{})
			if err != nil {
				if isOptionalNamespaceEventError(ns, namespace, err) {
					continue
				}
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

// isOptionalNamespaceEventError reports whether an events list failure in the
// default namespace can be skipped (supplemental cluster-scoped event lookup).
func isOptionalNamespaceEventError(ns, workloadNamespace string, err error) bool {
	if ns == workloadNamespace || ns != metav1.NamespaceDefault {
		return false
	}
	return errors.IsForbidden(err) || errors.IsNotFound(err) || errors.IsUnauthorized(err)
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

func buildFieldSelector(kind, name, namespace string) string {
	if namespace != "" {
		return fmt.Sprintf("involvedObject.kind=%s,involvedObject.name=%s,involvedObject.namespace=%s", kind, name, namespace)
	}
	return fmt.Sprintf("involvedObject.kind=%s,involvedObject.name=%s", kind, name)
}

func buildInvolvedObjectSelectors(graph *models.ResourceGraph) []string {
	if graph == nil {
		return nil
	}
	seen := make(map[string]bool)
	selectors := make([]string, 0)
	for _, resources := range graph.Resources {
		for _, r := range resources {
			sel := buildFieldSelector(r.Ref.Kind, r.Ref.Name, r.Ref.Namespace)
			if seen[sel] {
				continue
			}
			seen[sel] = true
			selectors = append(selectors, sel)
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
			if resourceMatchesInvolvedObject(r.Ref, ev.InvolvedObject) {
				return true
			}
		}
	}
	return resourceMatchesInvolvedObject(graph.Root, ev.InvolvedObject)
}

func resourceMatchesInvolvedObject(ref models.ResourceRef, obj corev1.ObjectReference) bool {
	if ref.Kind != obj.Kind || ref.Name != obj.Name {
		return false
	}
	return normalizeResourceNamespace(ref.Kind, ref.Namespace) ==
		normalizeInvolvedObjectNamespace(obj.Kind, obj.Namespace)
}

// normalizeInvolvedObjectNamespace maps API event involvedObject namespaces to a
// canonical form. Kubernetes often omits namespace on events for default-namespace
// resources; cluster-scoped object events may use an empty or "default" namespace.
func normalizeInvolvedObjectNamespace(kind, namespace string) string {
	if isClusterScopedKind(kind) {
		if namespace == metav1.NamespaceDefault {
			return ""
		}
		return namespace
	}
	if namespace == "" {
		return metav1.NamespaceDefault
	}
	return namespace
}

func normalizeResourceNamespace(kind, namespace string) string {
	if isClusterScopedKind(kind) {
		return ""
	}
	return namespace
}

func isClusterScopedKind(kind string) bool {
	switch kind {
	case "Node", "PersistentVolume", "Namespace", "StorageClass":
		return true
	default:
		return false
	}
}
