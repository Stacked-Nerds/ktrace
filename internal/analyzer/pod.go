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

		statusGroups := []struct {
			role     string
			statuses []corev1.ContainerStatus
		}{
			{role: "container", statuses: pod.Status.ContainerStatuses},
			{role: "init container", statuses: pod.Status.InitContainerStatuses},
			{role: "ephemeral container", statuses: pod.Status.EphemeralContainerStatuses},
		}
		for _, group := range statusGroups {
			for _, cs := range group.statuses {
				findings = append(findings, analyzeContainerStatus(pod, cs, group.role)...)
			}
		}

		if pod.Status.Reason == "Evicted" {
			findings = append(findings, models.Finding{
				Severity:      models.SeverityCritical,
				Condition:     "PodEvicted",
				Summary:       fmt.Sprintf("Pod %q was evicted", pod.Name),
				Explanation:   pod.Status.Message,
				Source:        podRef(pod),
				FieldPath:     "status.reason",
				Category:      "Pod",
				Recommendations: []string{
					fmt.Sprintf("kubectl describe pod %s -n %s", pod.Name, pod.Namespace),
					"Check node pressure, taints, ephemeral storage, and eviction events",
				},
			})
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
						FieldPath: "status.conditions[Ready]",
						Category:  "Pod",
						Recommendations: []string{
							fmt.Sprintf("kubectl describe pod %s -n %s", pod.Name, pod.Namespace),
						},
					})
				}
			}
		}
	}

	findings = append(findings, analyzeProbeEvents(graph)...)
	return dedupeFindings(findings)
}

func analyzeContainerStatus(pod *corev1.Pod, cs corev1.ContainerStatus, role string) []models.Finding {
	findings := make([]models.Finding, 0)
	ref := podRef(pod)
	fieldPrefix := "status.containerStatuses"
	switch role {
	case "init container":
		fieldPrefix = "status.initContainerStatuses"
	case "ephemeral container":
		fieldPrefix = "status.ephemeralContainerStatuses"
	}

	if waiting := cs.State.Waiting; waiting != nil && waiting.Reason != "" &&
		waiting.Reason != "ContainerCreating" && waiting.Reason != "PodInitializing" {
		severity := models.SeverityMedium
		summary := fmt.Sprintf("%s %q is waiting: %s", containerRoleTitle(role), cs.Name, waiting.Reason)
		recommendations := []string{
			fmt.Sprintf("kubectl describe pod %s -n %s", pod.Name, pod.Namespace),
		}

		switch waiting.Reason {
		case "ImagePullBackOff", "ErrImagePull", "InvalidImageName":
			severity = models.SeverityHigh
			summary = fmt.Sprintf("%s %q cannot pull image", containerRoleTitle(role), cs.Name)
			recommendations = append(recommendations,
				fmt.Sprintf("kubectl get events -n %s --field-selector involvedObject.name=%s", pod.Namespace, pod.Name),
				imagePullHint(pod, waiting.Message),
			)
		case "CrashLoopBackOff":
			severity = models.SeverityHigh
			summary = fmt.Sprintf("%s %q is crash looping", containerRoleTitle(role), cs.Name)
			recommendations = append(recommendations,
				fmt.Sprintf("kubectl logs %s -n %s -c %s --previous", pod.Name, pod.Namespace, cs.Name),
			)
		case "CreateContainerConfigError", "CreateContainerError", "RunContainerError":
			severity = models.SeverityHigh
			summary = fmt.Sprintf("%s %q cannot be created", containerRoleTitle(role), cs.Name)
			recommendations = append(recommendations,
				"Verify referenced Secrets, ConfigMaps, volumes, security settings, and container command",
			)
		default:
			if strings.Contains(waiting.Reason, "CreateContainer") {
				severity = models.SeverityHigh
				summary = fmt.Sprintf("%s %q cannot be created", containerRoleTitle(role), cs.Name)
				recommendations = append(recommendations,
					"Verify referenced Secrets, ConfigMaps, volumes, security settings, and container command",
				)
			}
		}

		findings = append(findings, models.Finding{
			Severity:        severity,
			Condition:       waiting.Reason,
			Summary:         summary,
			Explanation:     waiting.Message,
			Source:          ref,
			Container:       cs.Name,
			FieldPath:       fieldPrefix + "[" + cs.Name + "].state.waiting",
			Category:        "Pod",
			Recommendations: recommendations,
		})
	}

	terminations := []struct {
		state *corev1.ContainerStateTerminated
		field string
		last  bool
	}{
		{state: cs.State.Terminated, field: fieldPrefix + "[" + cs.Name + "].state.terminated"},
		{state: cs.LastTerminationState.Terminated, field: fieldPrefix + "[" + cs.Name + "].lastState.terminated", last: true},
	}
	for _, termination := range terminations {
		t := termination.state
		if t == nil || (t.ExitCode == 0 && t.Signal == 0 && (t.Reason == "" || t.Reason == "Completed")) {
			continue
		}
		condition := t.Reason
		if condition == "" {
			if t.Signal != 0 {
				condition = "ContainerSignaled"
			} else {
				condition = "NonZeroExit"
			}
		}
		if role == "init container" && condition != "OOMKilled" {
			condition = "InitContainerFailed"
		}
		severity := models.SeverityHigh
		recommendations := []string{
			fmt.Sprintf("kubectl logs %s -n %s -c %s --previous", pod.Name, pod.Namespace, cs.Name),
			fmt.Sprintf("kubectl describe pod %s -n %s", pod.Name, pod.Namespace),
		}
		if condition == "OOMKilled" {
			severity = models.SeverityCritical
			recommendations = append(recommendations, "Increase memory limits or investigate application memory use")
		}
		when := "terminated"
		if termination.last {
			when = "previously terminated"
		}
		findings = append(findings, models.Finding{
			Severity:        severity,
			Condition:       condition,
			Summary:         fmt.Sprintf("%s %q %s", containerRoleTitle(role), cs.Name, when),
			Explanation:     terminationExplanation(t),
			Source:          ref,
			Container:       cs.Name,
			FieldPath:       termination.field,
			Category:        "Pod",
			Recommendations: recommendations,
		})
	}

	if cs.RestartCount > 0 {
		explanation := fmt.Sprintf("%s %q has restarted %d time(s)", containerRoleTitle(role), cs.Name, cs.RestartCount)
		if last := cs.LastTerminationState.Terminated; last != nil {
			explanation += "; last termination: " + terminationExplanation(last)
		}
		findings = append(findings, models.Finding{
			Severity:      models.SeverityMedium,
			Condition:     "ContainerRestartHistory",
			Summary:       fmt.Sprintf("%s %q has restart history", containerRoleTitle(role), cs.Name),
			Explanation:   explanation,
			Source:        ref,
			Container:     cs.Name,
			FieldPath:     fieldPrefix + "[" + cs.Name + "].restartCount",
			Category:      "Pod",
			Recommendations: []string{
				fmt.Sprintf("kubectl logs %s -n %s -c %s --previous", pod.Name, pod.Namespace, cs.Name),
				fmt.Sprintf("kubectl describe pod %s -n %s", pod.Name, pod.Namespace),
			},
		})
	}
	return findings
}

func podRef(pod *corev1.Pod) models.ResourceRef {
	return models.ResourceRef{
		Kind: "Pod", Name: pod.Name, Namespace: pod.Namespace, UID: string(pod.UID),
	}
}

func containerRoleTitle(role string) string {
	if role == "" {
		return "Container"
	}
	return strings.ToUpper(role[:1]) + role[1:]
}

func terminationExplanation(terminated *corev1.ContainerStateTerminated) string {
	parts := make([]string, 0, 4)
	if terminated.Reason != "" {
		parts = append(parts, "reason "+terminated.Reason)
	}
	parts = append(parts, fmt.Sprintf("exit code %d", terminated.ExitCode))
	if terminated.Signal != 0 {
		parts = append(parts, fmt.Sprintf("signal %d", terminated.Signal))
	}
	if terminated.Message != "" {
		parts = append(parts, terminated.Message)
	}
	return strings.Join(parts, "; ")
}

func imagePullHint(pod *corev1.Pod, message string) string {
	lower := strings.ToLower(message)
	if strings.Contains(lower, "unauthorized") || strings.Contains(lower, "authentication required") ||
		strings.Contains(lower, "pull access denied") || strings.Contains(lower, "no basic auth credentials") ||
		strings.Contains(lower, "denied") {
		if len(pod.Spec.ImagePullSecrets) == 0 {
			return "Registry authentication appears to have failed and the pod declares no imagePullSecrets"
		}
		return "Registry authentication appears to have failed; verify the referenced imagePullSecrets and service account"
	}
	return "Verify the image name, tag, registry reachability, and pull credentials"
}

func analyzeProbeEvents(graph *models.ResourceGraph) []models.Finding {
	findings := make([]models.Finding, 0)
	for _, event := range graph.Events {
		if event.Source.Kind != "Pod" {
			continue
		}
		lower := strings.ToLower(event.Message)
		probe := ""
		probeName := ""
		switch {
		case event.Reason == "StartupProbeFailed" || strings.Contains(lower, "startup probe failed"):
			probe = "StartupProbeFailed"
			probeName = "Startup probe"
		case event.Reason == "ReadinessProbeFailed" || strings.Contains(lower, "readiness probe failed"):
			probe = "ReadinessProbeFailed"
			probeName = "Readiness probe"
		case event.Reason == "LivenessProbeFailed" || strings.Contains(lower, "liveness probe failed"):
			probe = "LivenessProbeFailed"
			probeName = "Liveness probe"
		}
		if probe == "" {
			continue
		}
		findings = append(findings, models.Finding{
			Severity:      models.SeverityHigh,
			Condition:     probe,
			Summary:       fmt.Sprintf("%s failed on pod %q", probeName, event.Source.Name),
			Explanation:   event.Message,
			Source:        event.Source,
			FieldPath:     "events",
			Category:      "Pod",
			Evidence:      []models.Evidence{{Type: "Event", Message: event.Message, Source: event.Source, Timestamp: event.Timestamp}},
			Recommendations: []string{
				fmt.Sprintf("kubectl describe pod %s -n %s", event.Source.Name, event.Source.Namespace),
				"Verify probe path, port, command, timing thresholds, and application startup behavior",
			},
		})
	}
	return findings
}
