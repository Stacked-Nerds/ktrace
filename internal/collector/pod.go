package collector

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/Stacked-Nerds/ktrace/internal/kubernetes"
	"github.com/Stacked-Nerds/ktrace/pkg/models"
)

type podCollector struct{}

func (c *podCollector) Kind() string { return "Pod" }

func (c *podCollector) Collect(ctx context.Context, client *kubernetes.Client, ref models.ResourceRef) ([]models.CollectedResource, error) {
	pod, err := client.Clientset.CoreV1().Pods(ref.Namespace).Get(ctx, ref.Name, metav1.GetOptions{})
	if err != nil {
		return nil, wrapNotFound("Pod", ref.Name, ref.Namespace, err)
	}
	cr, err := toCollectedResource("Pod", pod, pod.ObjectMeta)
	if err != nil {
		return nil, err
	}
	return []models.CollectedResource{cr}, nil
}

func collectFromPodRoot(ctx context.Context, client *kubernetes.Client, ref models.ResourceRef, state *collectState) error {
	if err := collectPod(ctx, client, ref, state); err != nil {
		return err
	}
	for _, pod := range state.resources("Pod") {
		if pod.Ref.Name == ref.Name && pod.Ref.Namespace == ref.Namespace {
			if err := collectOwnersForPod(ctx, client, pod, state); err != nil {
				state.warn(fmt.Sprintf("collect Pod owner chain: %v", err))
			}
			return nil
		}
	}
	return nil
}

func collectPodsInNamespace(ctx context.Context, client *kubernetes.Client, namespace string, state *collectState, limit int) error {
	list, err := client.Clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{Limit: int64(limit)})
	if err != nil {
		return fmt.Errorf("list pods in namespace: %w", err)
	}
	for i := range list.Items {
		pod := &list.Items[i]
		cr, err := toCollectedResource("Pod", pod, pod.ObjectMeta)
		if err != nil {
			return err
		}
		state.add(cr)
	}
	if list.Continue != "" {
		state.warn(fmt.Sprintf("namespace Pod list truncated at %d resources", limit))
	}
	return nil
}

func podNodeNames(pods []models.CollectedResource) map[string]struct{} {
	names := make(map[string]struct{})
	for _, p := range pods {
		var pod corev1.Pod
		if err := decodeRaw(p.Raw, &pod); err != nil {
			continue
		}
		if pod.Spec.NodeName != "" {
			names[pod.Spec.NodeName] = struct{}{}
		}
	}
	return names
}
