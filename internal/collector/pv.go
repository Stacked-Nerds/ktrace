package collector

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/Stacked-Nerds/ktrace/internal/kubernetes"
	"github.com/Stacked-Nerds/ktrace/pkg/models"
)

type pvCollector struct{}

func (c *pvCollector) Kind() string { return "PersistentVolume" }

func (c *pvCollector) Collect(ctx context.Context, client *kubernetes.Client, ref models.ResourceRef) ([]models.CollectedResource, error) {
	pv, err := client.Clientset.CoreV1().PersistentVolumes().Get(ctx, ref.Name, metav1.GetOptions{})
	if err != nil {
		return nil, wrapNotFound("PersistentVolume", ref.Name, ref.Namespace, err)
	}
	cr, err := toCollectedResource("PersistentVolume", pv, pv.ObjectMeta)
	if err != nil {
		return nil, err
	}
	return []models.CollectedResource{cr}, nil
}

func collectPVs(ctx context.Context, client *kubernetes.Client, names map[string]struct{}, state *collectState) error {
	for name := range names {
		pv, err := client.Clientset.CoreV1().PersistentVolumes().Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			if metav1.IsNotFound(err) {
				continue
			}
			return fmt.Errorf("get pv %q: %w", name, err)
		}
		cr, err := toCollectedResource("PersistentVolume", pv, pv.ObjectMeta)
		if err != nil {
			return err
		}
		state.add(cr)
	}
	return nil
}
