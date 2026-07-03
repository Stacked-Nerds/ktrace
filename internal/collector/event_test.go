package collector

import (
	"context"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/Stacked-Nerds/ktrace/internal/kubernetes"
	"github.com/Stacked-Nerds/ktrace/pkg/models"
)

func TestEventCollectorSortAndDedupe(t *testing.T) {
	now := time.Now()
	events := []*corev1.Event{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "ev1", Namespace: "default"},
			InvolvedObject: corev1.ObjectReference{
				Kind: "Pod", Name: "test-pod", Namespace: "default",
			},
			Type: "Warning", Reason: "FailedScheduling", Message: "0/1 nodes available",
			LastTimestamp: metav1.Time{Time: now},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "ev2", Namespace: "default"},
			InvolvedObject: corev1.ObjectReference{
				Kind: "Pod", Name: "test-pod", Namespace: "default",
			},
			Type: "Normal", Reason: "Scheduled", Message: "Successfully assigned",
			LastTimestamp: metav1.Time{Time: now.Add(time.Minute)},
		},
	}

	client := fake.NewSimpleClientset(events[0], events[1])
	k8sClient := kubernetes.NewFromClientset(client)
	ec := &eventCollector{}

	graph := models.NewResourceGraph(models.ResourceRef{Kind: "Pod", Name: "test-pod", Namespace: "default"})
	graph.AddResource(models.CollectedResource{
		Ref: models.ResourceRef{Kind: "Pod", Name: "test-pod", Namespace: "default"},
	})

	timeline, err := ec.collectForGraph(context.Background(), k8sClient, "default", graph)
	if err != nil {
		t.Fatalf("collectForGraph() error: %v", err)
	}
	if len(timeline) == 0 {
		t.Fatal("expected events")
	}
}

func BenchmarkEventCollector(b *testing.B) {
	events := make([]*corev1.Event, 0, 50)
	now := time.Now()
	for i := 0; i < 50; i++ {
		events = append(events, &corev1.Event{
			ObjectMeta: metav1.ObjectMeta{Name: "ev", Namespace: "default"},
			InvolvedObject: corev1.ObjectReference{
				Kind: "Pod", Name: "test-pod", Namespace: "default",
			},
			LastTimestamp: metav1.Time{Time: now.Add(time.Duration(i) * time.Second)},
		})
	}

	objs := make([]interface{}, len(events))
	for i, e := range events {
		objs[i] = e
	}
	client := fake.NewSimpleClientset(objs...)
	k8sClient := kubernetes.NewFromClientset(client)
	ec := &eventCollector{}
	graph := models.NewResourceGraph(models.ResourceRef{Kind: "Pod", Name: "test-pod", Namespace: "default"})
	graph.AddResource(models.CollectedResource{
		Ref: models.ResourceRef{Kind: "Pod", Name: "test-pod", Namespace: "default"},
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := ec.collectForGraph(context.Background(), k8sClient, "default", graph)
		if err != nil {
			b.Fatalf("collectForGraph() error: %v", err)
		}
	}
}
