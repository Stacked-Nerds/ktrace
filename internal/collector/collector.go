package collector

import (
	"context"
	"sync"

	"github.com/Stacked-Nerds/ktrace/internal/kubernetes"
	"github.com/Stacked-Nerds/ktrace/pkg/models"
)

// KindCollector collects resources of a specific kind.
type KindCollector interface {
	Kind() string
	Collect(ctx context.Context, client *kubernetes.Client, ref models.ResourceRef) ([]models.CollectedResource, error)
}

// collectState tracks resources during graph collection to avoid duplicates.
type collectState struct {
	mu      sync.Mutex
	graph   *models.ResourceGraph
	seenUID map[string]bool
}

func newCollectState(root models.ResourceRef) *collectState {
	return &collectState{
		graph:   models.NewResourceGraph(root),
		seenUID: make(map[string]bool),
	}
}

func (s *collectState) add(r models.CollectedResource) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if r.Metadata.UID != "" {
		if s.seenUID[r.Metadata.UID] {
			return
		}
		s.seenUID[r.Metadata.UID] = true
	}
	s.graph.AddResource(r)
}

func (s *collectState) hasUID(uid string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.seenUID[uid]
}

func (s *collectState) resources(kind string) []models.CollectedResource {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.graph.Resources[kind]
}
