package collector

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/Stacked-Nerds/ktrace/internal/kubernetes"
	"github.com/Stacked-Nerds/ktrace/pkg/models"
)

type nodeCollector struct{}

func (c *nodeCollector) Kind() string { return "Node" }

func (c *nodeCollector) Collect(ctx context.Context, client *kubernetes.Client, ref models.ResourceRef) ([]models.CollectedResource, error) {
	node, err := client.Clientset.CoreV1().Nodes().Get(ctx, ref.Name, metav1.GetOptions{})
	if err != nil {
		return nil, wrapNotFound("Node", ref.Name, ref.Namespace, err)
	}
	cr, err := toCollectedResource("Node", node, node.ObjectMeta)
	if err != nil {
		return nil, err
	}
	return []models.CollectedResource{cr}, nil
}

func collectNodes(ctx context.Context, client *kubernetes.Client, names map[string]struct{}, state *collectState) error {
	for name := range names {
		node, err := client.Clientset.CoreV1().Nodes().Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			if isNotFound(err) {
				continue
			}
			return fmt.Errorf("get node %q: %w", name, err)
		}
		cr, err := toCollectedResource("Node", node, node.ObjectMeta)
		if err != nil {
			return err
		}
		state.add(cr)
	}
	return nil
}
