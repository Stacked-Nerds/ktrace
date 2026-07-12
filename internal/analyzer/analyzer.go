package analyzer

import (
	"encoding/json"
	"sort"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	"github.com/Stacked-Nerds/ktrace/pkg/models"
)

// Rule detects failure conditions in a resource graph.
type Rule func(graph *models.ResourceGraph) []models.Finding

// Analyze runs all registered rules against the graph.
func Analyze(graph *models.ResourceGraph) []models.Finding {
	findings := make([]models.Finding, 0)
	for _, rule := range rules {
		findings = append(findings, rule(graph)...)
	}
	findings = dedupeFindings(findings)
	sortFindings(findings)
	return findings
}

var rules = []Rule{
	analyzePodContainers,
	analyzeScheduling,
	analyzePVC,
	analyzeMountFailures,
	analyzeNodes,
	analyzeDeploymentConditions,
	analyzeReferences,
	analyzeRollouts,
	analyzeWorkloadConditions,
}

func decodePod(raw []byte) (*corev1.Pod, error) {
	var pod corev1.Pod
	if err := json.Unmarshal(raw, &pod); err != nil {
		return nil, err
	}
	return &pod, nil
}

func decodePVC(raw []byte) (*corev1.PersistentVolumeClaim, error) {
	var pvc corev1.PersistentVolumeClaim
	if err := json.Unmarshal(raw, &pvc); err != nil {
		return nil, err
	}
	return &pvc, nil
}

func decodeNode(raw []byte) (*corev1.Node, error) {
	var node corev1.Node
	if err := json.Unmarshal(raw, &node); err != nil {
		return nil, err
	}
	return &node, nil
}

func decodeDeployment(raw []byte) (*appsv1.Deployment, error) {
	var d appsv1.Deployment
	if err := json.Unmarshal(raw, &d); err != nil {
		return nil, err
	}
	return &d, nil
}

func sortFindings(findings []models.Finding) {
	order := map[models.Severity]int{
		models.SeverityCritical: 0,
		models.SeverityHigh:     1,
		models.SeverityMedium:   2,
		models.SeverityLow:      3,
	}
	sort.SliceStable(findings, func(i, j int) bool {
		left, right := findings[i], findings[j]
		if order[left.Severity] != order[right.Severity] {
			return order[left.Severity] < order[right.Severity]
		}
		leftKey := left.Source.Kind + "|" + left.Source.Namespace + "|" + left.Source.Name + "|" +
			left.Condition + "|" + left.Container + "|" + left.FieldPath + "|" + left.Summary
		rightKey := right.Source.Kind + "|" + right.Source.Namespace + "|" + right.Source.Name + "|" +
			right.Condition + "|" + right.Container + "|" + right.FieldPath + "|" + right.Summary
		if leftKey == rightKey {
			return left.Explanation < right.Explanation
		}
		return leftKey < rightKey
	})
}
