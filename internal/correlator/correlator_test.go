package correlator

import (
	"testing"

	"github.com/Stacked-Nerds/ktrace/pkg/models"
)

func TestCorrelateIncludesMissingConfigurationReference(t *testing.T) {
	pod := models.ResourceRef{Kind: "Pod", Name: "api", Namespace: "prod", UID: "pod-1"}
	secret := models.ResourceRef{Kind: "Secret", Name: "database", Namespace: "prod"}
	graph := models.NewResourceGraph(pod)
	graph.References = []models.ResourceReference{{
		From:      pod,
		To:        secret,
		FieldPath: "spec.containers[0].env[0]",
		Observed:  true,
		Exists:    false,
	}}

	edges := Correlate(graph)
	if len(edges) != 1 {
		t.Fatalf("edges = %#v", edges)
	}
	if edges[0].Relation != "references-secret" {
		t.Fatalf("relation = %q", edges[0].Relation)
	}
}
