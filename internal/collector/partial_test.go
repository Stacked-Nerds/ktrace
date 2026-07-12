package collector

import (
	"context"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ktesting "k8s.io/client-go/testing"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/Stacked-Nerds/ktrace/internal/kubernetes"
)

func TestOrchestratorReturnsPartialGraphWhenChildrenAreForbidden(t *testing.T) {
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "prod", UID: "deploy-1"},
	}
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: "prod", UID: "namespace-1"},
	}
	clientset := fake.NewSimpleClientset(deployment, namespace)
	clientset.PrependReactor("list", "replicasets", func(ktesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewForbidden(
			schema.GroupResource{Group: "apps", Resource: "replicasets"},
			"",
			nil,
		)
	})

	orchestrator := NewOrchestrator(kubernetes.NewFromClientset(clientset))
	graph, err := orchestrator.Collect(context.Background(), "deployment", "api", "prod")
	if err != nil {
		t.Fatalf("Collect() error = %v", err)
	}
	if !graph.Partial {
		t.Fatal("expected partial graph")
	}
	if graph.Count("Deployment") != 1 {
		t.Fatalf("deployment count = %d", graph.Count("Deployment"))
	}
	if len(graph.Warnings) == 0 {
		t.Fatal("expected collection warning")
	}
}
