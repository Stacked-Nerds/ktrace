package collector

import (
	"context"
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/Stacked-Nerds/ktrace/internal/kubernetes"
	ktraceerrors "github.com/Stacked-Nerds/ktrace/pkg/errors"
	"github.com/Stacked-Nerds/ktrace/pkg/models"
)

// toCollectedResource converts a Kubernetes object to a CollectedResource.
func toCollectedResource(kind string, obj runtime.Object, meta metav1.ObjectMeta) (models.CollectedResource, error) {
	raw, err := json.Marshal(obj)
	if err != nil {
		return models.CollectedResource{}, fmt.Errorf("marshal %s/%s: %w", kind, meta.GetName(), err)
	}

	owners := make([]models.OwnerRef, 0, len(meta.GetOwnerReferences()))
	for _, o := range meta.GetOwnerReferences() {
		owners = append(owners, models.OwnerRef{
			Kind: o.Kind,
			Name: o.Name,
			UID:  string(o.UID),
		})
	}

	return models.CollectedResource{
		Ref: models.ResourceRef{
			Kind:      kind,
			Name:      meta.GetName(),
			Namespace: meta.GetNamespace(),
			UID:       string(meta.GetUID()),
		},
		Raw: raw,
		Metadata: models.ResourceMeta{
			UID:               string(meta.GetUID()),
			ResourceVersion:   meta.GetResourceVersion(),
			CreationTimestamp: meta.GetCreationTimestamp().Time,
			Labels:            meta.GetLabels(),
			OwnerReferences:   owners,
		},
	}, nil
}

// fetchNamespace collects the namespace resource.
func fetchNamespace(ctx context.Context, client *kubernetes.Client, namespace string, state *collectState) error {
	if namespace == "" {
		return nil
	}

	ns, err := client.Clientset.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("get namespace %q: %w", namespace, err)
	}

	cr, err := toCollectedResource("Namespace", ns, ns.ObjectMeta)
	if err != nil {
		return err
	}
	state.add(cr)
	return nil
}

// fetchEventsForInvolvedObjects collects events for all resources in the graph.
func fetchEventsForInvolvedObjects(ctx context.Context, client *kubernetes.Client, namespace string, state *collectState) error {
	ec := &eventCollector{}
	events, err := ec.collectForGraph(ctx, client, namespace, state.graph)
	if err != nil {
		return err
	}
	state.graph.Events = events
	state.graph.SortEvents()
	return nil
}

// extractPVCNamesFromPod returns PVC claim names referenced by a pod spec.
func extractPVCNamesFromPod(pod *corev1.Pod) []string {
	names := make([]string, 0)
	for _, vol := range pod.Spec.Volumes {
		if vol.PersistentVolumeClaim != nil {
			names = append(names, vol.PersistentVolumeClaim.ClaimName)
		}
	}
	return names
}

// wrapNotFound converts API not-found errors to ktrace errors.
func wrapNotFound(kind, name, namespace string, err error) error {
	if err == nil {
		return nil
	}
	if apierrors.IsNotFound(err) {
		return ktraceerrors.NotFound(kind, name, namespace)
	}
	return err
}

func isNotFound(err error) bool {
	return apierrors.IsNotFound(err)
}
