package collector

import (
	"testing"

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
