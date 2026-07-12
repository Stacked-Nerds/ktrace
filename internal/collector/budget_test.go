package collector

import (
	"testing"

	"github.com/Stacked-Nerds/ktrace/pkg/models"
)

func TestCollectionBudgetMarksGraphPartial(t *testing.T) {
	state := newCollectStateWithLimit(models.ResourceRef{Kind: "Namespace", Name: "prod"}, 1)
	state.add(models.CollectedResource{
		Ref:      models.ResourceRef{Kind: "Pod", Name: "one", Namespace: "prod"},
		Metadata: models.ResourceMeta{UID: "one"},
	})
	state.add(models.CollectedResource{
		Ref:      models.ResourceRef{Kind: "Pod", Name: "two", Namespace: "prod"},
		Metadata: models.ResourceMeta{UID: "two"},
	})

	if !state.graph.Partial {
		t.Fatal("expected partial graph")
	}
	if state.graph.Count("Pod") != 1 {
		t.Fatalf("pod count = %d, want 1", state.graph.Count("Pod"))
	}
	if len(state.graph.Warnings) != 1 {
		t.Fatalf("warnings = %#v", state.graph.Warnings)
	}
}
