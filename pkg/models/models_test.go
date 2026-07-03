package models

import (
	"testing"
	"time"
)

func TestResourceGraphAddAndCount(t *testing.T) {
	g := NewResourceGraph(ResourceRef{Kind: "Deployment", Name: "frontend", Namespace: "default"})
	g.AddResource(CollectedResource{
		Ref: ResourceRef{Kind: "Pod", Name: "frontend-abc"},
	})
	g.AddResource(CollectedResource{
		Ref: ResourceRef{Kind: "Pod", Name: "frontend-def"},
	})

	if g.Count("Pod") != 2 {
		t.Fatalf("expected 2 pods, got %d", g.Count("Pod"))
	}
}

func TestSortEvents(t *testing.T) {
	now := time.Now()
	g := NewResourceGraph(ResourceRef{Kind: "Pod", Name: "test"})
	g.Events = []TimelineEvent{
		{Timestamp: now.Add(2 * time.Minute), Reason: "second"},
		{Timestamp: now, Reason: "first"},
		{Timestamp: now.Add(time.Minute), Reason: "middle"},
	}
	g.SortEvents()

	if g.Events[0].Reason != "first" || g.Events[2].Reason != "second" {
		t.Fatalf("events not sorted: %+v", g.Events)
	}
}

func TestResourceRefString(t *testing.T) {
	ref := ResourceRef{Kind: "Deployment", Name: "frontend", Namespace: "production"}
	want := "Deployment/production/frontend"
	if ref.String() != want {
		t.Errorf("String() = %q, want %q", ref.String(), want)
	}
}
