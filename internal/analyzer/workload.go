package analyzer

import (
	"encoding/json"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"

	"github.com/Stacked-Nerds/ktrace/pkg/models"
)

func analyzeWorkloadConditions(graph *models.ResourceGraph) []models.Finding {
	findings := make([]models.Finding, 0)
	findings = append(findings, analyzeStatefulSets(graph)...)
	findings = append(findings, analyzeDaemonSets(graph)...)
	findings = append(findings, analyzeJobs(graph)...)
	findings = append(findings, analyzeCronJobs(graph)...)
	return findings
}

func analyzeStatefulSets(graph *models.ResourceGraph) []models.Finding {
	findings := make([]models.Finding, 0)
	for _, cr := range graph.Resources["StatefulSet"] {
		var workload appsv1.StatefulSet
		if json.Unmarshal(cr.Raw, &workload) != nil {
			continue
		}
		ref := models.ResourceRef{Kind: "StatefulSet", Name: workload.Name, Namespace: workload.Namespace}
		if workload.Generation > 0 && workload.Status.ObservedGeneration < workload.Generation {
			findings = append(findings, generationLagFinding(ref, workload.Generation, workload.Status.ObservedGeneration))
		}
		for _, condition := range workload.Status.Conditions {
			if condition.Status != corev1.ConditionTrue {
				continue
			}
			findings = append(findings, workloadConditionFinding(
				ref, string(condition.Type), condition.Reason, condition.Message, models.SeverityHigh,
			))
		}
		if workload.Status.ObservedGeneration >= workload.Generation &&
			workload.Status.Replicas > 0 && workload.Status.ReadyReplicas < workload.Status.Replicas {
			findings = append(findings, models.Finding{
				Severity:    models.SeverityMedium,
				Condition:   "StatefulSetNotReady",
				Summary:     fmt.Sprintf("StatefulSet %q has unready replicas", workload.Name),
				Explanation: fmt.Sprintf("%d of %d replicas are ready", workload.Status.ReadyReplicas, workload.Status.Replicas),
				Source:      ref,
				FieldPath:   "status.readyReplicas",
				Category:    "Workload",
				Recommendations: []string{
					fmt.Sprintf("kubectl describe statefulset %s -n %s", workload.Name, workload.Namespace),
					fmt.Sprintf("kubectl get pods -n %s", workload.Namespace),
				},
			})
		}
	}
	return findings
}

func analyzeDaemonSets(graph *models.ResourceGraph) []models.Finding {
	findings := make([]models.Finding, 0)
	for _, cr := range graph.Resources["DaemonSet"] {
		var workload appsv1.DaemonSet
		if json.Unmarshal(cr.Raw, &workload) != nil {
			continue
		}
		ref := models.ResourceRef{Kind: "DaemonSet", Name: workload.Name, Namespace: workload.Namespace}
		if workload.Generation > 0 && workload.Status.ObservedGeneration < workload.Generation {
			findings = append(findings, generationLagFinding(ref, workload.Generation, workload.Status.ObservedGeneration))
		}
		for _, condition := range workload.Status.Conditions {
			if condition.Status != corev1.ConditionTrue {
				continue
			}
			findings = append(findings, workloadConditionFinding(
				ref, string(condition.Type), condition.Reason, condition.Message, models.SeverityHigh,
			))
		}
		if workload.Status.NumberMisscheduled > 0 {
			findings = append(findings, models.Finding{
				Severity:    models.SeverityHigh,
				Condition:   "DaemonSetMisscheduled",
				Summary:     fmt.Sprintf("DaemonSet %q has misscheduled pods", workload.Name),
				Explanation: fmt.Sprintf("%d pod(s) are running where they should not", workload.Status.NumberMisscheduled),
				Source:      ref,
				FieldPath:   "status.numberMisscheduled",
				Category:    "Workload",
				Recommendations: []string{
					fmt.Sprintf("kubectl describe daemonset %s -n %s", workload.Name, workload.Namespace),
					"Review node selectors, affinity, taints, and tolerations",
				},
			})
		}
		if workload.Status.DesiredNumberScheduled > workload.Status.NumberReady {
			findings = append(findings, models.Finding{
				Severity:    models.SeverityMedium,
				Condition:   "DaemonSetNotReady",
				Summary:     fmt.Sprintf("DaemonSet %q has unavailable pods", workload.Name),
				Explanation: fmt.Sprintf("%d of %d desired pods are ready", workload.Status.NumberReady, workload.Status.DesiredNumberScheduled),
				Source:      ref,
				FieldPath:   "status.numberReady",
				Category:    "Workload",
				Recommendations: []string{
					fmt.Sprintf("kubectl describe daemonset %s -n %s", workload.Name, workload.Namespace),
					fmt.Sprintf("kubectl get pods -n %s -o wide", workload.Namespace),
				},
			})
		}
	}
	return findings
}

func analyzeJobs(graph *models.ResourceGraph) []models.Finding {
	findings := make([]models.Finding, 0)
	for _, cr := range graph.Resources["Job"] {
		var workload batchv1.Job
		if json.Unmarshal(cr.Raw, &workload) != nil {
			continue
		}
		ref := models.ResourceRef{Kind: "Job", Name: workload.Name, Namespace: workload.Namespace}
		for _, condition := range workload.Status.Conditions {
			if condition.Status != corev1.ConditionTrue {
				continue
			}
			switch condition.Type {
			case batchv1.JobFailed, batchv1.JobFailureTarget:
				conditionName := "JobFailed"
				if condition.Type == batchv1.JobFailureTarget {
					conditionName = "JobFailureTarget"
				}
				findings = append(findings, workloadConditionFinding(
					ref, conditionName, condition.Reason, condition.Message, models.SeverityHigh,
				))
			}
		}
	}
	return findings
}

func analyzeCronJobs(graph *models.ResourceGraph) []models.Finding {
	findings := make([]models.Finding, 0)
	for _, cr := range graph.Resources["CronJob"] {
		var workload batchv1.CronJob
		if json.Unmarshal(cr.Raw, &workload) != nil {
			continue
		}
		if workload.Spec.Suspend != nil && *workload.Spec.Suspend {
			findings = append(findings, models.Finding{
				Severity:    models.SeverityLow,
				Condition:   "CronJobSuspended",
				Summary:     fmt.Sprintf("CronJob %q is suspended", workload.Name),
				Explanation: "The CronJob controller will not start scheduled jobs while spec.suspend is true",
				Source:      models.ResourceRef{Kind: "CronJob", Name: workload.Name, Namespace: workload.Namespace},
				FieldPath:   "spec.suspend",
				Category:    "Workload",
				Recommendations: []string{
					fmt.Sprintf("Confirm whether CronJob/%s should remain suspended", workload.Name),
				},
			})
		}
	}
	for _, event := range graph.Events {
		if event.Source.Kind != "CronJob" {
			continue
		}
		switch event.Reason {
		case "FailedCreate", "FailedNeedsStart", "InvalidSchedule", "MissSchedule":
			findings = append(findings, models.Finding{
				Severity:    models.SeverityHigh,
				Condition:   "CronJob" + event.Reason,
				Summary:     fmt.Sprintf("CronJob %q reported %s", event.Source.Name, event.Reason),
				Explanation: event.Message,
				Source:      event.Source,
				FieldPath:   "events",
				Category:    "Workload",
				Evidence:    []models.Evidence{{Type: "Event", Message: event.Message, Source: event.Source, Timestamp: event.Timestamp}},
				Recommendations: []string{
					fmt.Sprintf("kubectl describe cronjob %s -n %s", event.Source.Name, event.Source.Namespace),
					fmt.Sprintf("kubectl get jobs -n %s", event.Source.Namespace),
				},
			})
		}
	}
	return findings
}

func workloadConditionFinding(ref models.ResourceRef, condition, reason, message string, severity models.Severity) models.Finding {
	summary := fmt.Sprintf("%s reports %s", ref.String(), condition)
	if reason != "" {
		summary += ": " + reason
	}
	return models.Finding{
		Severity:    severity,
		Condition:   condition,
		Summary:     summary,
		Explanation: message,
		Source:      ref,
		FieldPath:   "status.conditions[" + condition + "]",
		Category:    "Workload",
		Recommendations: []string{
			fmt.Sprintf("Inspect %s status, events, and owned pods", ref.String()),
		},
	}
}
