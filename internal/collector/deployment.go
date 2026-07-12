package collector

import (
	"context"
	"fmt"
	"sort"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/Stacked-Nerds/ktrace/internal/kubernetes"
	ktraceerrors "github.com/Stacked-Nerds/ktrace/pkg/errors"
	"github.com/Stacked-Nerds/ktrace/pkg/models"
	"github.com/Stacked-Nerds/ktrace/pkg/utils"
)

type deploymentCollector struct{}

func (c *deploymentCollector) Kind() string { return "Deployment" }

func (c *deploymentCollector) Collect(ctx context.Context, client *kubernetes.Client, ref models.ResourceRef) ([]models.CollectedResource, error) {
	deploy, err := client.Clientset.AppsV1().Deployments(ref.Namespace).Get(ctx, ref.Name, metav1.GetOptions{})
	if err != nil {
		return nil, wrapNotFound("Deployment", ref.Name, ref.Namespace, err)
	}

	cr, err := toCollectedResource("Deployment", deploy, deploy.ObjectMeta)
	if err != nil {
		return nil, err
	}
	return []models.CollectedResource{cr}, nil
}

func collectDeployment(ctx context.Context, client *kubernetes.Client, ref models.ResourceRef, state *collectState) error {
	dc := &deploymentCollector{}
	resources, err := dc.Collect(ctx, client, ref)
	if err != nil {
		return err
	}
	for _, r := range resources {
		state.add(r)
	}

	deployUID := resources[0].Metadata.UID
	if err := collectReplicaSetsForOwner(ctx, client, ref.Namespace, deployUID, state); err != nil {
		state.warn(fmt.Sprintf("collect Deployment children: %v", err))
	}
	return nil
}

func collectReplicaSetsForOwner(ctx context.Context, client *kubernetes.Client, namespace, ownerUID string, state *collectState) error {
	list, err := client.Clientset.AppsV1().ReplicaSets(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("list replicasets: %w", err)
	}

	owned := make([]int, 0)
	for i := range list.Items {
		if utils.HasOwner(list.Items[i].OwnerReferences, ownerUID) {
			owned = append(owned, i)
		}
	}
	sort.Slice(owned, func(i, j int) bool {
		left := list.Items[owned[i]].CreationTimestamp.Time
		right := list.Items[owned[j]].CreationTimestamp.Time
		return left.After(right)
	})
	const replicaSetHistoryLimit = 10
	if len(owned) > replicaSetHistoryLimit {
		owned = owned[:replicaSetHistoryLimit]
		state.notice("Deployment revision history limited to 10 most recent ReplicaSets")
	}

	type replicaSetTarget struct {
		uid      string
		selector string
	}
	targets := make([]replicaSetTarget, 0)
	for _, index := range owned {
		rs := &list.Items[index]
		cr, err := toCollectedResource("ReplicaSet", rs, rs.ObjectMeta)
		if err != nil {
			return err
		}
		state.add(cr)
		targets = append(targets, replicaSetTarget{
			uid:      string(rs.UID),
			selector: formatLabelSelector(rs.Spec.Selector),
		})
	}

	for _, target := range targets {
		if err := collectPodsForOwnerWithSelectorMode(
			ctx, client, namespace, target.uid, target.selector, state, false,
		); err != nil {
			return err
		}
	}
	if err := collectRelatedFromPods(ctx, client, namespace, state); err != nil {
		return err
	}
	return collectServicesForPods(ctx, client, namespace, collectedPodLabels(state), state)
}

func collectReplicaSet(ctx context.Context, client *kubernetes.Client, ref models.ResourceRef, state *collectState) error {
	rs, err := client.Clientset.AppsV1().ReplicaSets(ref.Namespace).Get(ctx, ref.Name, metav1.GetOptions{})
	if err != nil {
		return wrapNotFound("ReplicaSet", ref.Name, ref.Namespace, err)
	}

	cr, err := toCollectedResource("ReplicaSet", rs, rs.ObjectMeta)
	if err != nil {
		return err
	}
	state.add(cr)

	if err := collectPodsForOwnerWithSelector(
		ctx,
		client,
		ref.Namespace,
		string(rs.UID),
		formatLabelSelector(rs.Spec.Selector),
		state,
	); err != nil {
		state.warn(fmt.Sprintf("collect ReplicaSet pods: %v", err))
	}
	return nil
}

func collectFromReplicaSetRoot(ctx context.Context, client *kubernetes.Client, ref models.ResourceRef, state *collectState) error {
	kind := utils.NormalizeKind(ref.Kind)
	if kind != "replicaset" {
		return ktraceerrors.UnsupportedKind(ref.Kind)
	}
	return collectReplicaSet(ctx, client, ref, state)
}
