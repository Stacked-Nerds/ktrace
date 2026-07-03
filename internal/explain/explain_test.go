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
}
