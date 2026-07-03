package analyzer

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"

	"github.com/Stacked-Nerds/ktrace/pkg/models"
)

func analyzePodContainers(graph *models.ResourceGraph) []models.Finding {
	findings := make([]models.Finding, 0)

	for _, cr := range graph.Resources["Pod"] {
		pod, err := decodePod(cr.Raw)
		if err != nil {
			continue
		}

		for _, cs := range pod.Status.ContainerStatuses {
			state := cs.State
			ref := models.ResourceRef{
				Kind:      "Pod",
				Name:      pod.Name,
				Namespace: pod.Namespace,
				UID:       string(pod.UID),
			}

			if w := state.Waiting; w != nil {
				switch w.Reason {
				case "ImagePullBackOff", "ErrImagePull":
					findings = append(findings, models.Finding{
						Severity:    models.SeverityHigh,
						Condition:   w.Reason,
						Summary:     fmt.Sprintf("Container %q cannot pull image", cs.Name),
						Explanation: w.Message,
						Source:      ref,
						Recommendations: []string{
							fmt.Sprintf("kubectl describe pod %s -n %s", pod.Name, pod.Namespace),
							fmt.Sprintf("kubectl get events -n %s --field-selector involvedObject.name=%s", pod.Namespace, pod.Name),
							"Verify image name, tag, registry credentials, and imagePullSecrets",
						},
					})
				case "CrashLoopBackOff":
					findings = append(findings, models.Finding{
						Severity:    models.SeverityHigh,
						Condition:   w.Reason,
						Summary:     fmt.Sprintf("Container %q is crash looping", cs.Name),
						Explanation: w.Message,
						Source:      ref,
						Recommendations: []string{
							fmt.Sprintf("kubectl logs %s -n %s -c %s --previous", pod.Name, pod.Namespace, cs.Name),
							fmt.Sprintf("kubectl describe pod %s -n %s", pod.Name, pod.Namespace),
						},
					})
				case "CreateContainerConfigError":
					findings = append(findings, models.Finding{
						Severity:    models.SeverityHigh,
						Condition:   w.Reason,
						Summary:     fmt.Sprintf("Container %q config error (missing Secret/ConfigMap?)", cs.Name),
						Explanation: w.Message,
						Source:      ref,
						Recommendations: []string{
							fmt.Sprintf("kubectl describe pod %s -n %s", pod.Name, pod.Namespace),
							"Verify referenced Secrets and ConfigMaps exist in the namespace",
						},
					})
				}
			}

			if oom := oomTerminatedState(cs); oom != nil {
				findings = append(findings, models.Finding{
					Severity:    models.SeverityCritical,
					Condition:   "OOMKilled",
					Summary:     fmt.Sprintf("Container %q was OOM killed", cs.Name),
					Explanation: fmt.Sprintf("Exit code %d: container exceeded memory limits", oom.ExitCode),
					Source:      ref,
					Recommendations: []string{
						fmt.Sprintf("kubectl describe pod %s -n %s", pod.Name, pod.Namespace),
						"Increase memory limits or fix memory leak in the application",
					},
				})
			}
		}

		for _, cond := range pod.Status.Conditions {
			if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionFalse {
				if strings.Contains(cond.Reason, "ContainersNotReady") || cond.Reason == "Unhealthy" {
					findings = append(findings, models.Finding{
						Severity:    models.SeverityMedium,
						Condition:   cond.Reason,
						Summary:     fmt.Sprintf("Pod %q is not ready", pod.Name),
						Explanation: cond.Message,
						Source: models.ResourceRef{
							Kind: "Pod", Name: pod.Name, Namespace: pod.Namespace,
						},
						Recommendations: []string{
							fmt.Sprintf("kubectl describe pod %s -n %s", pod.Name, pod.Namespace),
						},
					})
				}
			}
		}
	}

	return findings
}

func oomTerminatedState(cs corev1.ContainerStatus) *corev1.ContainerStateTerminated {
	if cs.State.Terminated != nil && cs.State.Terminated.Reason == "OOMKilled" {
		return cs.State.Terminated
	}
	if cs.LastTerminationState.Terminated != nil && cs.LastTerminationState.Terminated.Reason == "OOMKilled" {
		return cs.LastTerminationState.Terminated
	}
	return nil
}
