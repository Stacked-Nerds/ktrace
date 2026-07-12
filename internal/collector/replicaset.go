package collector

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/Stacked-Nerds/ktrace/internal/kubernetes"
	"github.com/Stacked-Nerds/ktrace/pkg/models"
	"github.com/Stacked-Nerds/ktrace/pkg/utils"
)

type replicaSetCollector struct{}

func (c *replicaSetCollector) Kind() string { return "ReplicaSet" }

func (c *replicaSetCollector) Collect(ctx context.Context, client *kubernetes.Client, ref models.ResourceRef) ([]models.CollectedResource, error) {
	rs, err := client.Clientset.AppsV1().ReplicaSets(ref.Namespace).Get(ctx, ref.Name, metav1.GetOptions{})
	if err != nil {
		return nil, wrapNotFound("ReplicaSet", ref.Name, ref.Namespace, err)
	}
	cr, err := toCollectedResource("ReplicaSet", rs, rs.ObjectMeta)
	if err != nil {
		return nil, err
	}
	return []models.CollectedResource{cr}, nil
}

func collectPodsForOwner(ctx context.Context, client *kubernetes.Client, namespace, ownerUID string, state *collectState) error {
	return collectPodsForOwnerWithSelector(ctx, client, namespace, ownerUID, "", state)
}

func collectPodsForOwnerWithSelector(
	ctx context.Context,
	client *kubernetes.Client,
	namespace, ownerUID, labelSelector string,
	state *collectState,
) error {
	return collectPodsForOwnerWithSelectorMode(
		ctx, client, namespace, ownerUID, labelSelector, state, true,
	)
}

func collectPodsForOwnerWithSelectorMode(
	ctx context.Context,
	client *kubernetes.Client,
	namespace, ownerUID, labelSelector string,
	state *collectState,
	collectRelated bool,
) error {
	list, err := client.Clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return fmt.Errorf("list pods: %w", err)
	}

	podLabels := make([]map[string]string, 0)
	for i := range list.Items {
		pod := &list.Items[i]
		if !utils.HasOwner(pod.OwnerReferences, ownerUID) {
			continue
		}
		cr, err := toCollectedResource("Pod", pod, pod.ObjectMeta)
		if err != nil {
			return err
		}
		state.add(cr)
		podLabels = append(podLabels, pod.Labels)
	}

	if !collectRelated {
		return nil
	}
	if err := collectRelatedFromPods(ctx, client, namespace, state); err != nil {
		return err
	}

	return collectServicesForPods(ctx, client, namespace, podLabels, state)
}

func collectPod(ctx context.Context, client *kubernetes.Client, ref models.ResourceRef, state *collectState) error {
	pod, err := client.Clientset.CoreV1().Pods(ref.Namespace).Get(ctx, ref.Name, metav1.GetOptions{})
	if err != nil {
		return wrapNotFound("Pod", ref.Name, ref.Namespace, err)
	}

	cr, err := toCollectedResource("Pod", pod, pod.ObjectMeta)
	if err != nil {
		return err
	}
	state.add(cr)

	if err := collectRelatedFromPods(ctx, client, ref.Namespace, state); err != nil {
		return err
	}

	return collectServicesForPods(ctx, client, ref.Namespace, []map[string]string{pod.Labels}, state)
}

func collectRelatedFromPods(ctx context.Context, client *kubernetes.Client, namespace string, state *collectState) error {
	pods := state.resources("Pod")
	pvcNames := make(map[string]struct{})
	nodeNames := make(map[string]struct{})

	for _, p := range pods {
		var pod corev1.Pod
		if err := decodeRaw(p.Raw, &pod); err != nil {
			return err
		}
		for _, name := range extractPVCNamesFromPod(&pod) {
			pvcNames[name] = struct{}{}
		}
		if pod.Spec.NodeName != "" {
			nodeNames[pod.Spec.NodeName] = struct{}{}
		}
	}

	var wg sync.WaitGroup
	var pvcErr, nodeErr error
	wg.Add(2)
	go func() {
		defer wg.Done()
		pvcErr = collectPVCs(ctx, client, namespace, pvcNames, state)
	}()
	go func() {
		defer wg.Done()
		nodeErr = collectNodes(ctx, client, nodeNames, state)
	}()
	wg.Wait()

	if pvcErr != nil {
		state.warn(fmt.Sprintf("collect PVC dependencies: %v", pvcErr))
	}
	if nodeErr != nil {
		state.warn(fmt.Sprintf("collect Node dependencies: %v", nodeErr))
	}
	return nil
}

func collectedPodLabels(state *collectState) []map[string]string {
	pods := state.resources("Pod")
	labels := make([]map[string]string, 0, len(pods))
	for _, resource := range pods {
		var pod corev1.Pod
		if decodeRaw(resource.Raw, &pod) == nil {
			labels = append(labels, pod.Labels)
		}
	}
	return labels
}

func decodeRaw(raw []byte, obj interface{}) error {
	if len(raw) == 0 {
		return fmt.Errorf("empty raw object")
	}
	return json.Unmarshal(raw, obj)
}
