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
	client       *kubernetes.Client
	maxResources int
}

// NewOrchestrator creates an Orchestrator with the given Kubernetes client.
func NewOrchestrator(client *kubernetes.Client) *Orchestrator {
	return &Orchestrator{client: client, maxResources: 1000}
}

// SetMaxResources sets the collection budget. Values <= 0 use the default.
func (o *Orchestrator) SetMaxResources(limit int) {
	if limit <= 0 {
		limit = 1000
	}
	o.maxResources = limit
}

// Collect walks the resource graph starting from the given root kind and name.
func (o *Orchestrator) Collect(ctx context.Context, kind, name, namespace string) (*models.ResourceGraph, error) {
	normalizedKind := utils.NormalizeKind(kind)
	root, err := normalizeRootRef(normalizedKind, name, namespace)
	if err != nil {
		return nil, err
	}

	state := newCollectStateWithLimit(root, o.maxResources)

	switch normalizedKind {
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
	case "statefulset":
		if err := collectStatefulSet(ctx, o.client, root, state); err != nil {
			return nil, err
		}
	case "daemonset":
		if err := collectDaemonSet(ctx, o.client, root, state); err != nil {
			return nil, err
		}
	case "job":
		if err := collectJob(ctx, o.client, root, state); err != nil {
			return nil, err
		}
	case "cronjob":
		if err := collectCronJob(ctx, o.client, root, state); err != nil {
			return nil, err
		}
	default:
		return nil, ktraceerrors.UnsupportedKind(kind)
	}

	collectConfigurationReferences(ctx, o.client, state)

	ns := namespace
	if normalizedKind == "namespace" {
		ns = name
	}
	if ns != "" {
		if err := fetchNamespace(ctx, o.client, ns, state); err != nil {
			state.warn(fmt.Sprintf("collect Namespace metadata: %v", err))
		}
	}

	if err := fetchEventsForInvolvedObjects(ctx, o.client, ns, state); err != nil {
		state.warn(fmt.Sprintf("collect Events: %v", err))
	}

	return state.graph, nil
}
