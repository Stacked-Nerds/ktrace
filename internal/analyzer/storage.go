package analyzer

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"

	"github.com/Stacked-Nerds/ktrace/pkg/models"
)

func analyzePVC(graph *models.ResourceGraph) []models.Finding {
	findings := make([]models.Finding, 0)

	for _, cr := range graph.Resources["PersistentVolumeClaim"] {
		pvc, err := decodePVC(cr.Raw)
		if err != nil {
			continue
		}
		ref := models.ResourceRef{
			Kind: "PersistentVolumeClaim", Name: pvc.Name, Namespace: pvc.Namespace,
		}

		if pvc.Status.Phase == corev1.ClaimPending {
			sc := pvc.Spec.StorageClassName
			scName := "<none>"
			if sc != nil && *sc != "" {
				scName = *sc
			}
			findings = append(findings, models.Finding{
				Severity:    models.SeverityHigh,
				Condition:   "PVCPending",
				Summary:     fmt.Sprintf("PVC %q is pending", pvc.Name),
				Explanation: fmt.Sprintf("StorageClass %q may be missing or unable to provision volume", scName),
				Source:      ref,
				Recommendations: []string{
					"kubectl get storageclass",
					fmt.Sprintf("kubectl describe pvc %s -n %s", pvc.Name, pvc.Namespace),
					fmt.Sprintf("kubectl get events -n %s --field-selector involvedObject.name=%s", pvc.Namespace, pvc.Name),
				},
			})
		}
	}

	return findings
}

func analyzeMountFailures(graph *models.ResourceGraph) []models.Finding {
	findings := make([]models.Finding, 0)

	for _, ev := range graph.Events {
		switch ev.Reason {
		case "FailedMount", "FailedAttachVolume", "VolumeFailedAttach", "ProvisioningFailed":
			findings = append(findings, models.Finding{
				Severity:    models.SeverityHigh,
				Condition:   ev.Reason,
				Summary:     fmt.Sprintf("Volume issue on %s/%s", ev.Source.Kind, ev.Source.Name),
				Explanation: ev.Message,
				Source:      ev.Source,
				Recommendations: []string{
					fmt.Sprintf("kubectl describe %s %s -n %s", ev.Source.Kind, ev.Source.Name, ev.Source.Namespace),
					"kubectl get storageclass",
					"kubectl get pv",
				},
			})
		}
	}

	return dedupeFindings(findings)
}
