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
	mu            sync.Mutex
	graph         *models.ResourceGraph
	seenUID       map[string]bool
	seenRef       map[string]bool
	resourceCount int
	maxResources  int
}

func newCollectState(root models.ResourceRef) *collectState {
	return newCollectStateWithLimit(root, 1000)
}

func newCollectStateWithLimit(root models.ResourceRef, maxResources int) *collectState {
	if maxResources <= 0 {
		maxResources = 1000
	}
	return &collectState{
		graph:        models.NewResourceGraph(root),
		seenUID:      make(map[string]bool),
		seenRef:      make(map[string]bool),
		maxResources: maxResources,
	}
}

func (s *collectState) add(r models.CollectedResource) {
	s.mu.Lock()
	defer s.mu.Unlock()
	refKey := r.Ref.String()
	if s.seenRef[refKey] {
		return
	}
	if s.resourceCount >= s.maxResources {
		s.graph.Partial = true
		s.addWarningLocked("resource collection budget reached; results may be incomplete")
		return
	}
	if r.Metadata.UID != "" {
		if s.seenUID[r.Metadata.UID] {
			return
		}
		s.seenUID[r.Metadata.UID] = true
	}
	s.graph.AddResource(r)
	s.seenRef[refKey] = true
	s.resourceCount++
}

func (s *collectState) hasUID(uid string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.seenUID[uid]
}

func (s *collectState) resources(kind string) []models.CollectedResource {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]models.CollectedResource, len(s.graph.Resources[kind]))
	copy(out, s.graph.Resources[kind])
	return out
}

func (s *collectState) warn(message string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.graph.Partial = true
	s.addWarningLocked(message)
}

func (s *collectState) notice(message string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.addWarningLocked(message)
}

func (s *collectState) addWarningLocked(message string) {
	if message == "" {
		return
	}
	for _, warning := range s.graph.Warnings {
		if warning == message {
			return
		}
	}
	s.graph.Warnings = append(s.graph.Warnings, message)
}
