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

	for _, fieldSelector := range fieldSelectors {
		list, err := client.Clientset.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{
			FieldSelector: fieldSelector,
		})
		if err != nil {
			return nil, fmt.Errorf("list events: %w", err)
		}

		for i := range list.Items {
			ev := &list.Items[i]
			key := string(ev.UID)
			if key == "" {
				key = fmt.Sprintf("%s/%s/%s", ev.InvolvedObject.Kind, ev.InvolvedObject.Name, ev.Reason)
			}
			if seen[key] {
				continue
			}
			seen[key] = true

			ts := ev.LastTimestamp.Time
			if ts.IsZero() {
				ts = ev.EventTime.Time
			}
			if ts.IsZero() {
				ts = ev.FirstTimestamp.Time
			}
			if ts.IsZero() {
				ts = time.Now()
			}

			timeline = append(timeline, models.TimelineEvent{
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
	}

	// Also list namespace-wide recent events when graph has multiple resources
	if graph != nil && len(graph.Resources) > 0 {
		list, err := client.Clientset.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("list namespace events: %w", err)
		}

		targetUIDs := graphUIDs(graph)
		for i := range list.Items {
			ev := &list.Items[i]
			if !matchesGraphEvent(ev, graph, targetUIDs) {
				continue
			}
			key := string(ev.UID)
			if seen[key] {
				continue
			}
			seen[key] = true

			ts := eventTimestamp(ev)
			timeline = append(timeline, models.TimelineEvent{
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
	}

	return timeline, nil
}

func eventTimestamp(ev *corev1.Event) time.Time {
	ts := ev.LastTimestamp.Time
	if ts.IsZero() {
		ts = ev.EventTime.Time
	}
	if ts.IsZero() {
		ts = ev.FirstTimestamp.Time
	}
	return ts
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
