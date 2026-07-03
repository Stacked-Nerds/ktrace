package collector

import (
	"context"
	"fmt"

	"github.com/Stacked-Nerds/ktrace/internal/kubernetes"
	ktraceerrors "github.com/Stacked-Nerds/ktrace/pkg/errors"
	"github.com/Stacked-Nerds/ktrace/pkg/models"
	"github.com/Stacked-Nerds/ktrace/pkg/utils"
)

// Orchestrator collects related Kubernetes resources starting from a root reference.
type Orchestrator struct {
	client *kubernetes.Client
}

// NewOrchestrator creates an Orchestrator with the given Kubernetes client.
func NewOrchestrator(client *kubernetes.Client) *Orchestrator {
	return &Orchestrator{client: client}
}

// Collect walks the resource graph starting from the given root kind and name.
func (o *Orchestrator) Collect(ctx context.Context, kind, name, namespace string) (*models.ResourceGraph, error) {
	root, err := normalizeRootRef(utils.NormalizeKind(kind), name, namespace)
	if err != nil {
		return nil, err
	}

	state := newCollectState(root)

	switch utils.NormalizeKind(kind) {
	case "deployment":
		if err := collectDeployment(ctx, o.client, root, state); err != nil {
			return nil, err
		}
	case "replicaset":
		if err := collectFromReplicaSetRoot(ctx, o.client, root, state); err != nil {
			return nil, err
		}
	case "pod":
		if err := collectFromPodRoot(ctx, o.client, root, state); err != nil {
			return nil, err
		}
	case "namespace":
		if err := collectFromNamespaceRoot(ctx, o.client, root, state); err != nil {
			return nil, err
		}
	default:
		return nil, ktraceerrors.UnsupportedKind(kind)
	}

	ns := namespace
	if utils.NormalizeKind(kind) == "namespace" {
		ns = name
	}
	if ns != "" {
		if err := fetchNamespace(ctx, o.client, ns, state); err != nil {
			return nil, fmt.Errorf("collect namespace: %w", err)
		}
	}

	if err := fetchEventsForInvolvedObjects(ctx, o.client, ns, state); err != nil {
		return nil, fmt.Errorf("collect events: %w", err)
	}

	return state.graph, nil
}
