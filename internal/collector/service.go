package collector

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/Stacked-Nerds/ktrace/internal/kubernetes"
	"github.com/Stacked-Nerds/ktrace/pkg/models"
	"github.com/Stacked-Nerds/ktrace/pkg/utils"
)

type serviceCollector struct{}

func (c *serviceCollector) Kind() string { return "Service" }

func (c *serviceCollector) Collect(ctx context.Context, client *kubernetes.Client, ref models.ResourceRef) ([]models.CollectedResource, error) {
	svc, err := client.Clientset.CoreV1().Services(ref.Namespace).Get(ctx, ref.Name, metav1.GetOptions{})
	if err != nil {
		return nil, wrapNotFound("Service", ref.Name, ref.Namespace, err)
	}
	cr, err := toCollectedResource("Service", svc, svc.ObjectMeta)
	if err != nil {
		return nil, err
	}
	return []models.CollectedResource{cr}, nil
}

func collectServicesForPods(ctx context.Context, client *kubernetes.Client, namespace string, podLabels []map[string]string, state *collectState) error {
	if len(podLabels) == 0 {
		return nil
	}

	list, err := client.Clientset.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		state.warn(fmt.Sprintf("collect related Services: %v", err))
		return nil
	}

	for i := range list.Items {
		svc := &list.Items[i]
		if !serviceMatchesAnyPod(svc, podLabels) {
			continue
		}
		cr, err := toCollectedResource("Service", svc, svc.ObjectMeta)
		if err != nil {
			return err
		}
		state.add(cr)
	}
	return nil
}

func serviceMatchesAnyPod(svc *corev1.Service, podLabels []map[string]string) bool {
	if len(svc.Spec.Selector) == 0 {
		return false
	}
	for _, labels := range podLabels {
		if utils.SelectorMatches(svc.Spec.Selector, labels) {
			return true
		}
	}
	return false
}
