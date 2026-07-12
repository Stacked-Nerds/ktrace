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
	seen := make(map[string]bool)
	timeline := make([]models.TimelineEvent, 0)

	namespaces := eventNamespaces(namespace, graph)
	for _, ns := range namespaces {
		if graph != nil && len(graph.Resources) > 0 {
			targeted, supported, err := listTargetedGraphEvents(ctx, client, ns, graph)
			if err != nil {
				if isOptionalNamespaceEventError(ns, namespace, err) {
					continue
				}
				return nil, fmt.Errorf("list targeted events in %q: %w", ns, err)
			}
			if supported {
				targetUIDs := graphUIDs(graph)
				for i := range targeted {
					if matchesGraphEvent(&targeted[i], graph, targetUIDs) {
						appendEvent(&timeline, seen, &targeted[i])
					}
				}
				continue
			}

			targetUIDs := graphUIDs(graph)
			items, err := listAllEvents(ctx, client, ns, metav1.ListOptions{})
			if err != nil {
				if isOptionalNamespaceEventError(ns, namespace, err) {
					continue
				}
				return nil, fmt.Errorf("list namespace events in %q: %w", ns, err)
			}
			for i := range items {
				ev := &items[i]
				if !matchesGraphEvent(ev, graph, targetUIDs) {
					continue
				}
				appendEvent(&timeline, seen, ev)
			}
			continue
		}

		// Minimal graph: use targeted field selectors for the root resource.
		selectors := []string{}
		if graph != nil && graph.Root.Name != "" {
			selectors = []string{buildFieldSelector(graph.Root.Kind, graph.Root.Name, graph.Root.Namespace)}
		}
		for _, fieldSelector := range selectors {
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
				appendEvent(&timeline, seen, &list.Items[i])
			}
		}
	}

	return timeline, nil
}

const maxTargetedEventQueries = 25

// listTargetedGraphEvents uses involvedObject.uid field selectors on small
// graphs. It reports supported=false when a server does not support the
// selector or the graph is large enough that one paginated list is cheaper.
func listTargetedGraphEvents(
	ctx context.Context,
	client *kubernetes.Client,
	namespace string,
	graph *models.ResourceGraph,
) ([]corev1.Event, bool, error) {
	refs := eventRefsForNamespace(namespace, graph)
	if len(refs) == 0 {
		return nil, true, nil
	}
	if len(refs) > maxTargetedEventQueries {
		return nil, false, nil
	}

	all := make([]corev1.Event, 0)
	for _, ref := range refs {
		selector := buildFieldSelector(ref.Kind, ref.Name, ref.Namespace)
		if ref.UID != "" {
			selector = "involvedObject.uid=" + ref.UID
		}
		items, err := listAllEvents(ctx, client, namespace, metav1.ListOptions{
			FieldSelector: selector,
		})
		if err != nil {
			if errors.IsBadRequest(err) || errors.IsInvalid(err) {
				return nil, false, nil
			}
			return nil, false, err
		}
		all = append(all, items...)
	}
	return all, true, nil
}

func eventRefsForNamespace(namespace string, graph *models.ResourceGraph) []models.ResourceRef {
	refs := make([]models.ResourceRef, 0)
	seen := make(map[string]bool)
	add := func(ref models.ResourceRef) {
		eventNamespace := normalizeInvolvedObjectNamespace(ref.Kind, ref.Namespace)
		if isClusterScopedKind(ref.Kind) {
			eventNamespace = metav1.NamespaceDefault
		}
		if eventNamespace != namespace {
			return
		}
		key := ref.String()
		if seen[key] {
			return
		}
		seen[key] = true
		refs = append(refs, ref)
	}
	add(graph.Root)
	for _, resources := range graph.Resources {
		for _, resource := range resources {
			add(resource.Ref)
		}
	}
	return refs
}

// listAllEvents pages through all events in a namespace.
func listAllEvents(ctx context.Context, client *kubernetes.Client, ns string, opts metav1.ListOptions) ([]corev1.Event, error) {
	all := make([]corev1.Event, 0)
	for {
		list, err := client.Clientset.CoreV1().Events(ns).List(ctx, opts)
		if err != nil {
			return nil, err
		}
		all = append(all, list.Items...)
		if list.Continue == "" {
			break
		}
		opts.Continue = list.Continue
	}
	return all, nil
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

	needsDefault := graph.Count("Node") > 0 ||
		graph.Count("PersistentVolume") > 0 ||
		graph.Count("Namespace") > 0 ||
		graph.Count("StorageClass") > 0
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
