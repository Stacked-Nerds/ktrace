package explain

import (
	"testing"

	"github.com/Stacked-Nerds/ktrace/pkg/models"
)

func TestRootCausePicksHighestSeverity(t *testing.T) {
	findings := []models.Finding{
		{Severity: models.SeverityMedium, Summary: "medium"},
		{Severity: models.SeverityCritical, Summary: "critical"},
	}
	rc := RootCause(findings)
	if rc.Summary != "critical" {
		t.Fatalf("got %q", rc.Summary)
	}
}

func TestStatusFailed(t *testing.T) {
	if Status([]models.Finding{{Severity: models.SeverityHigh}}) != models.StatusFailed {
		t.Fatal("expected failed status")
	}
	if Status(nil) != models.StatusHealthy {
		t.Fatal("expected healthy status")
	}
	if Status([]models.Finding{{
		Severity: models.SeverityLow, Condition: "CronJobSuspended",
	}}) != models.StatusHealthy {
		t.Fatal("informational context must not degrade health")
	}
}

func TestDiagnosePrefersSpecificUpstreamCause(t *testing.T) {
	deployment := models.ResourceRef{Kind: "Deployment", Name: "api", Namespace: "prod"}
	pod := models.ResourceRef{Kind: "Pod", Name: "api-123", Namespace: "prod"}
	secret := models.ResourceRef{Kind: "Secret", Name: "database", Namespace: "prod"}
	findings := []models.Finding{
		{
			Severity:  models.SeverityHigh,
			Condition: "CrashLoopBackOff",
			Summary:   "container is crash looping",
			Source:    pod,
		},
		{
			Severity:  models.SeverityHigh,
			Condition: "MissingSecret",
			Summary:   "Secret database does not exist",
			Source:    secret,
		},
	}
	edges := []models.Edge{
		{From: deployment, To: pod, Relation: "owns"},
		{From: pod, To: secret, Relation: "references-secret"},
	}

	got := Diagnose(deployment, findings, edges, nil)
	if got.RootCause == nil || got.RootCause.Condition != "MissingSecret" {
		t.Fatalf("root cause = %#v, want MissingSecret", got.RootCause)
	}
	if len(got.Symptoms) != 1 || got.Symptoms[0].Condition != "CrashLoopBackOff" {
		t.Fatalf("symptoms = %#v", got.Symptoms)
	}
	if len(got.EvidenceChain) < 3 {
		t.Fatalf("evidence chain = %#v", got.EvidenceChain)
	}
	if got.Confidence <= 0 || got.Confidence > 1 {
		t.Fatalf("confidence = %f", got.Confidence)
	}
}

func TestDiagnoseIsDeterministicForEqualScores(t *testing.T) {
	findings := []models.Finding{
		{Severity: models.SeverityHigh, Condition: "Unknown", Source: models.ResourceRef{Kind: "Pod", Name: "b"}},
		{Severity: models.SeverityHigh, Condition: "Unknown", Source: models.ResourceRef{Kind: "Pod", Name: "a"}},
	}

	got := Diagnose(models.ResourceRef{}, findings, nil, nil)
	if got.RootCause == nil || got.RootCause.Source.Name != "a" {
		t.Fatalf("root cause = %#v, want Pod/a", got.RootCause)
	}
}
