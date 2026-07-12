package analyzer

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"

	"github.com/Stacked-Nerds/ktrace/pkg/models"
)

func analyzeScheduling(graph *models.ResourceGraph) []models.Finding {
	findings := make([]models.Finding, 0)

	for _, cr := range graph.Resources["Pod"] {
		pod, err := decodePod(cr.Raw)
		if err != nil {
			continue
		}
		if pod.Spec.NodeName != "" {
			continue
		}
		for _, cond := range pod.Status.Conditions {
			if cond.Type == corev1.PodScheduled && cond.Status == corev1.ConditionFalse && cond.Reason == "Unschedulable" {
				findings = append(findings, models.Finding{
					Severity:    models.SeverityHigh,
					Condition:   "FailedScheduling",
					Summary:     fmt.Sprintf("Pod %q cannot be scheduled", pod.Name),
					Explanation: cond.Message,
					Source: models.ResourceRef{
						Kind: "Pod", Name: pod.Name, Namespace: pod.Namespace,
					},
					FieldPath: "status.conditions[PodScheduled]",
					Category:  "Scheduling",
					Recommendations: []string{
						fmt.Sprintf("kubectl describe pod %s -n %s", pod.Name, pod.Namespace),
						"kubectl get nodes",
						"kubectl describe nodes",
					},
				})
			}
		}
	}

	for _, ev := range graph.Events {
		if ev.Reason != "FailedScheduling" {
			continue
		}
		findings = append(findings, models.Finding{
			Severity:    models.SeverityHigh,
			Condition:   "FailedScheduling",
			Summary:     fmt.Sprintf("%s/%s scheduling failed", ev.Source.Kind, ev.Source.Name),
			Explanation: ev.Message,
			Source:      ev.Source,
			FieldPath:   "status.conditions[PodScheduled]",
			Category:    "Scheduling",
			Evidence: []models.Evidence{{
				Type: "Event", Message: ev.Message, Source: ev.Source, Timestamp: ev.Timestamp,
			}},
			Recommendations: []string{
				fmt.Sprintf("kubectl describe %s %s -n %s", ev.Source.Kind, ev.Source.Name, ev.Source.Namespace),
				"kubectl get nodes",
			},
		})
	}

	return dedupeFindings(findings)
}
