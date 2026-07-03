package collector

import (
	"fmt"
	"testing"

	corev1 "k8s.io/api/core/v1"

	"github.com/Stacked-Nerds/ktrace/pkg/models"
)

func TestEventNamespacesIncludesDefaultForClusterScoped(t *testing.T) {
	graph := models.NewResourceGraph(models.ResourceRef{Kind: "Deployment", Name: "app", Namespace: "production"})
	graph.AddResource(models.CollectedResource{Ref: models.ResourceRef{Kind: "Node", Name: "node-1"}})

	namespaces := eventNamespaces("production", graph)
	if len(namespaces) != 2 {
		t.Fatalf("expected 2 namespaces, got %d: %v", len(namespaces), namespaces)
	}
	if namespaces[0] != "production" || namespaces[1] != "default" {
		t.Fatalf("unexpected namespaces: %v", namespaces)
	}
}

func TestEventNamespacesSkipsDefaultWhenAlreadyWorkloads(t *testing.T) {
	graph := models.NewResourceGraph(models.ResourceRef{Kind: "Pod", Name: "app", Namespace: "default"})
	graph.AddResource(models.CollectedResource{Ref: models.ResourceRef{Kind: "Node", Name: "node-1"}})

	namespaces := eventNamespaces("default", graph)
	if len(namespaces) != 1 {
		t.Fatalf("expected 1 namespace, got %v", namespaces)
	}
}

func TestResourceMatchesInvolvedObjectNamespace(t *testing.T) {
	ref := models.ResourceRef{Kind: "Pod", Name: "nginx", Namespace: "production"}
	obj := corev1.ObjectReference{Kind: "Pod", Name: "nginx", Namespace: "default"}
	if resourceMatchesInvolvedObject(ref, obj) {
		t.Fatal("expected different namespaces not to match")
	}

	obj.Namespace = "production"
	if !resourceMatchesInvolvedObject(ref, obj) {
		t.Fatal("expected same namespace to match")
	}
}

func TestResourceMatchesInvolvedObjectEmptyEventNamespace(t *testing.T) {
	ref := models.ResourceRef{Kind: "Pod", Name: "nginx", Namespace: "default"}
	obj := corev1.ObjectReference{Kind: "Pod", Name: "nginx"}
	if !resourceMatchesInvolvedObject(ref, obj) {
		t.Fatal("empty involvedObject namespace should match default namespace resource")
	}
}

func TestResourceMatchesInvolvedObjectClusterScoped(t *testing.T) {
	ref := models.ResourceRef{Kind: "Node", Name: "node-1"}
	obj := corev1.ObjectReference{Kind: "Node", Name: "node-1", Namespace: "default"}
	if !resourceMatchesInvolvedObject(ref, obj) {
		t.Fatal("cluster-scoped event with default namespace should match")
	}

	obj.Namespace = ""
	if !resourceMatchesInvolvedObject(ref, obj) {
		t.Fatal("cluster-scoped event with empty namespace should match")
	}
}

func TestBuildFieldSelectorIncludesKindAndNamespace(t *testing.T) {
	got := buildFieldSelector("Pod", "nginx", "production")
	want := "involvedObject.kind=Pod,involvedObject.name=nginx,involvedObject.namespace=production"
	if got != want {
		t.Fatalf("buildFieldSelector() = %q, want %q", got, want)
	}
}

func TestIsOptionalNamespaceEventError(t *testing.T) {
	if isOptionalNamespaceEventError("production", "production", fmt.Errorf("forbidden")) {
		t.Fatal("workload namespace errors should not be optional")
	}
}
