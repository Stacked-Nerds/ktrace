package explain

import (
	"github.com/Stacked-Nerds/ktrace/pkg/models"
)

// RootCause selects the most likely root cause from findings.
func RootCause(findings []models.Finding) *models.Finding {
	if len(findings) == 0 {
		return nil
	}
	best := findings[0]
	for i := 1; i < len(findings); i++ {
		if severityRank(findings[i].Severity) < severityRank(best.Severity) {
			best = findings[i]
		}
	}
	return &best
}

// Status derives overall health from findings.
func Status(findings []models.Finding) models.HealthStatus {
	if len(findings) == 0 {
		return models.StatusHealthy
	}
	rc := RootCause(findings)
	switch rc.Severity {
	case models.SeverityCritical, models.SeverityHigh:
		return models.StatusFailed
	case models.SeverityMedium:
		return models.StatusDegraded
	default:
		return models.StatusDegraded
	}
}

// Recommendations returns deduplicated kubectl commands from all findings.
func Recommendations(findings []models.Finding) []string {
	seen := make(map[string]bool)
	out := make([]string, 0)
	for _, f := range findings {
		for _, rec := range f.Recommendations {
			if rec == "" || seen[rec] {
				continue
			}
			seen[rec] = true
			out = append(out, rec)
		}
	}
	return out
}

func severityRank(s models.Severity) int {
	switch s {
	case models.SeverityCritical:
		return 0
	case models.SeverityHigh:
		return 1
	case models.SeverityMedium:
		return 2
	default:
		return 3
	}
}
