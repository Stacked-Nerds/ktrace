package analyzer

import (
	"fmt"
	"sort"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	"github.com/Stacked-Nerds/ktrace/pkg/models"
)

func analyzeNodes(graph *models.ResourceGraph) []models.Finding {
	findings := make([]models.Finding, 0)

	for _, cr := range graph.Resources["Node"] {
		node, err := decodeNode(cr.Raw)
		if err != nil {
			continue
		}
		for _, cond := range node.Status.Conditions {
			if cond.Type == corev1.NodeReady && cond.Status != corev1.ConditionTrue {
				findings = append(findings, models.Finding{
					Severity:    models.SeverityCritical,
					Condition:   "NodeNotReady",
					Summary:     fmt.Sprintf("Node %q is not ready", node.Name),
					Explanation: cond.Message,
					Source: models.ResourceRef{
						Kind: "Node", Name: node.Name,
					},
					Recommendations: []string{
						fmt.Sprintf("kubectl describe node %s", node.Name),
						"kubectl get pods -A --field-selector spec.nodeName=" + node.Name,
					},
				})
			}
		}
	}

	return findings
}

func analyzeDeploymentConditions(graph *models.ResourceGraph) []models.Finding {
	findings := make([]models.Finding, 0)

	for _, cr := range graph.Resources["Deployment"] {
		d, err := decodeDeployment(cr.Raw)
		if err != nil {
			continue
		}
		for _, cond := range d.Status.Conditions {
			if cond.Status == corev1.ConditionFalse && (cond.Type == appsv1.DeploymentAvailable || cond.Type == appsv1.DeploymentProgressing) {
				findings = append(findings, models.Finding{
					Severity:    models.SeverityMedium,
					Condition:   string(cond.Type),
					Summary:     fmt.Sprintf("Deployment %q: %s", d.Name, cond.Reason),
					Explanation: cond.Message,
					Source: models.ResourceRef{
						Kind: "Deployment", Name: d.Name, Namespace: d.Namespace,
					},
					Recommendations: []string{
						fmt.Sprintf("kubectl describe deployment %s -n %s", d.Name, d.Namespace),
						fmt.Sprintf("kubectl get rs,pods -n %s", d.Namespace),
					},
				})
			}
		}
	}

	return findings
}

func dedupeFindings(findings []models.Finding) []models.Finding {
	sort.SliceStable(findings, func(i, j int) bool {
		left := findings[i]
		right := findings[j]
		leftKey := left.Condition + "|" + left.Source.String() + "|" + left.Container + "|" +
			left.FieldPath + "|" + left.Summary + "|" + left.Explanation
		rightKey := right.Condition + "|" + right.Source.String() + "|" + right.Container + "|" +
			right.FieldPath + "|" + right.Summary + "|" + right.Explanation
		return leftKey < rightKey
	})
	seen := make(map[string]bool)
	out := make([]models.Finding, 0, len(findings))
	for _, f := range findings {
		key := f.Condition + "|" + f.Source.String() + "|" + f.Container + "|" + f.FieldPath
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, f)
	}
	return out
}
