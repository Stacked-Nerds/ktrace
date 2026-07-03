package collector

import (
	"context"
	"fmt"

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
	return collectReplicaSetsForOwner(ctx, client, ref.Namespace, deployUID, state)
}

func collectReplicaSetsForOwner(ctx context.Context, client *kubernetes.Client, namespace, ownerUID string, state *collectState) error {
	list, err := client.Clientset.AppsV1().ReplicaSets(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("list replicasets: %w", err)
	}

	rsUIDs := make([]string, 0)
	for i := range list.Items {
		rs := &list.Items[i]
		if !utils.HasOwner(rs.OwnerReferences, ownerUID) {
			continue
		}
		cr, err := toCollectedResource("ReplicaSet", rs, rs.ObjectMeta)
		if err != nil {
			return err
		}
		state.add(cr)
		rsUIDs = append(rsUIDs, string(rs.UID))
	}

	for _, rsUID := range rsUIDs {
		if err := collectPodsForOwner(ctx, client, namespace, rsUID, state); err != nil {
			return err
		}
	}
	return nil
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

	return collectPodsForOwner(ctx, client, ref.Namespace, string(rs.UID), state)
}

func collectFromReplicaSetRoot(ctx context.Context, client *kubernetes.Client, ref models.ResourceRef, state *collectState) error {
	kind := utils.NormalizeKind(ref.Kind)
	if kind != "replicaset" {
		return ktraceerrors.UnsupportedKind(ref.Kind)
	}
	return collectReplicaSet(ctx, client, ref, state)
}
