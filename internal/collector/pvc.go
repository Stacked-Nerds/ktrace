package collector

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/Stacked-Nerds/ktrace/internal/kubernetes"
	"github.com/Stacked-Nerds/ktrace/pkg/models"
)

type pvcCollector struct{}

func (c *pvcCollector) Kind() string { return "PersistentVolumeClaim" }

func (c *pvcCollector) Collect(ctx context.Context, client *kubernetes.Client, ref models.ResourceRef) ([]models.CollectedResource, error) {
	pvc, err := client.Clientset.CoreV1().PersistentVolumeClaims(ref.Namespace).Get(ctx, ref.Name, metav1.GetOptions{})
	if err != nil {
		return nil, wrapNotFound("PersistentVolumeClaim", ref.Name, ref.Namespace, err)
	}
	cr, err := toCollectedResource("PersistentVolumeClaim", pvc, pvc.ObjectMeta)
	if err != nil {
		return nil, err
	}
	return []models.CollectedResource{cr}, nil
}

func collectPVCs(ctx context.Context, client *kubernetes.Client, namespace string, names map[string]struct{}, state *collectState) error {
	pvNames := make(map[string]struct{})

	for name := range names {
		pvc, err := client.Clientset.CoreV1().PersistentVolumeClaims(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			if isNotFound(err) {
				continue
			}
			return fmt.Errorf("get pvc %q: %w", name, err)
		}
		cr, err := toCollectedResource("PersistentVolumeClaim", pvc, pvc.ObjectMeta)
		if err != nil {
			return err
		}
		state.add(cr)

		if pvc.Spec.VolumeName != "" {
			pvNames[pvc.Spec.VolumeName] = struct{}{}
		}
	}

	return collectPVs(ctx, client, pvNames, state)
}

func pvcNamesFromPods(pods []models.CollectedResource) map[string]struct{} {
	names := make(map[string]struct{})
	for _, p := range pods {
		var pod corev1.Pod
		if err := decodeRaw(p.Raw, &pod); err != nil {
			continue
		}
		for _, n := range extractPVCNamesFromPod(&pod) {
			names[n] = struct{}{}
		}
	}
	return names
}
