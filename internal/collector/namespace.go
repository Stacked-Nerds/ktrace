package collector

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/Stacked-Nerds/ktrace/internal/kubernetes"
	ktraceerrors "github.com/Stacked-Nerds/ktrace/pkg/errors"
	"github.com/Stacked-Nerds/ktrace/pkg/models"
)

type namespaceCollector struct{}

func (c *namespaceCollector) Kind() string { return "Namespace" }

func (c *namespaceCollector) Collect(ctx context.Context, client *kubernetes.Client, ref models.ResourceRef) ([]models.CollectedResource, error) {
	ns, err := client.Clientset.CoreV1().Namespaces().Get(ctx, ref.Name, metav1.GetOptions{})
	if err != nil {
		return nil, wrapNotFound("Namespace", ref.Name, ref.Namespace, err)
	}
	cr, err := toCollectedResource("Namespace", ns, ns.ObjectMeta)
	if err != nil {
		return nil, err
	}
	return []models.CollectedResource{cr}, nil
}

const namespaceResourceLimit = 100

func collectFromNamespaceRoot(ctx context.Context, client *kubernetes.Client, ref models.ResourceRef, state *collectState) error {
	nc := &namespaceCollector{}
	resources, err := nc.Collect(ctx, client, ref)
	if err != nil {
		return err
	}
	for _, r := range resources {
		state.add(r)
	}

	namespace := ref.Name
	if err := collectDeploymentsInNamespace(ctx, client, namespace, state, namespaceResourceLimit); err != nil {
		state.warn(err.Error())
	}
	if err := collectPodsInNamespace(ctx, client, namespace, state, namespaceResourceLimit); err != nil {
		state.warn(err.Error())
	}

	nodeNames := podNodeNames(state.resources("Pod"))
	if err := collectNodes(ctx, client, nodeNames, state); err != nil {
		state.warn(fmt.Sprintf("collect namespace Nodes: %v", err))
	}

	pvcNames := pvcNamesFromPods(state.resources("Pod"))
	if err := collectPVCs(ctx, client, namespace, pvcNames, state); err != nil {
		state.warn(fmt.Sprintf("collect namespace PVCs: %v", err))
	}
	return nil
}

func collectDeploymentsInNamespace(ctx context.Context, client *kubernetes.Client, namespace string, state *collectState, limit int) error {
	list, err := client.Clientset.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{Limit: int64(limit)})
	if err != nil {
		return fmt.Errorf("list deployments in namespace: %w", err)
	}
	for i := range list.Items {
		d := &list.Items[i]
		cr, err := toCollectedResource("Deployment", d, d.ObjectMeta)
		if err != nil {
			return err
		}
		state.add(cr)
	}
	if list.Continue != "" {
		state.warn(fmt.Sprintf("namespace Deployment list truncated at %d resources", limit))
	}
	return nil
}

func normalizeRootRef(kind, name, namespace string) (models.ResourceRef, error) {
	k := kind
	switch kind {
	case "deployment":
		k = "Deployment"
	case "replicaset":
		k = "ReplicaSet"
	case "pod":
		k = "Pod"
	case "namespace":
		k = "Namespace"
	case "statefulset":
		k = "StatefulSet"
	case "daemonset":
		k = "DaemonSet"
	case "job":
		k = "Job"
	case "cronjob":
		k = "CronJob"
	default:
		return models.ResourceRef{}, ktraceerrors.UnsupportedKind(kind)
	}

	ref := models.ResourceRef{
		Kind: k,
		Name: name,
	}
	if k != "Namespace" {
		ref.Namespace = namespace
	}
	return ref, nil
}
