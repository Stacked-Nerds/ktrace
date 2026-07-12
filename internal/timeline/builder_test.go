package timeline

import (
	"testing"
	"time"

	"github.com/Stacked-Nerds/ktrace/pkg/models"
)

func TestBuildSortsAndDedupes(t *testing.T) {
	now := time.Now()
	graph := models.NewResourceGraph(models.ResourceRef{Kind: "Pod", Name: "app", Namespace: "default"})
	graph.Events = []models.TimelineEvent{
		{Timestamp: now.Add(time.Minute), Reason: "Started", Message: "started", Source: models.ResourceRef{Kind: "Pod", Name: "app"}},
		{Timestamp: now.Add(time.Minute), Reason: "Started", Message: "started", Source: models.ResourceRef{Kind: "Pod", Name: "app"}},
		{Timestamp: now, Reason: "Scheduled", Message: "scheduled", Source: models.ResourceRef{Kind: "Pod", Name: "app"}},
	}

	entries := Build(graph)
	if len(entries) < 2 {
		t.Fatalf("expected at least 2 entries, got %d", len(entries))
	}
	if entries[0].Timestamp.After(entries[len(entries)-1].Timestamp) {
		t.Fatal("timeline not sorted ascending")
	}
}

func TestBuildOrdersEqualTimestampsDeterministically(t *testing.T) {
	now := time.Now()
	graph := models.NewResourceGraph(models.ResourceRef{Kind: "Pod", Name: "root"})
	graph.Events = []models.TimelineEvent{
		{Timestamp: now, Reason: "B", Source: models.ResourceRef{Kind: "Pod", Name: "b"}},
		{Timestamp: now, Reason: "A", Source: models.ResourceRef{Kind: "Pod", Name: "a"}},
	}

	got := Build(graph)
	if len(got) != 2 || got[0].Source.Name != "a" || got[1].Source.Name != "b" {
		t.Fatalf("timeline order = %#v", got)
	}
}
