package models

import "time"

// HealthStatus represents the overall health of the traced resource.
type HealthStatus string

const (
	StatusHealthy  HealthStatus = "Healthy"
	StatusDegraded HealthStatus = "Degraded"
	StatusFailed   HealthStatus = "Failed"
	StatusUnknown  HealthStatus = "Unknown"
)

// Severity ranks finding importance.
type Severity string

const (
	SeverityCritical Severity = "Critical"
	SeverityHigh     Severity = "High"
	SeverityMedium   Severity = "Medium"
	SeverityLow      Severity = "Low"
)

// Edge links two resources in the collection graph.
type Edge struct {
	From     ResourceRef `json:"from"`
	To       ResourceRef `json:"to"`
	Relation string      `json:"relation"`
}

// TimelineEntry is a human-readable timeline step.
type TimelineEntry struct {
	Timestamp time.Time   `json:"timestamp"`
	Title     string      `json:"title"`
	Detail    string      `json:"detail,omitempty"`
	Source    ResourceRef `json:"source,omitempty"`
	Severity  Severity    `json:"severity,omitempty"`
}

// Finding is a detected failure condition with explanation and fixes.
type Finding struct {
	Severity        Severity    `json:"severity"`
	Condition       string      `json:"condition"`
	Summary         string      `json:"summary"`
	Explanation     string      `json:"explanation"`
	Source          ResourceRef `json:"source"`
	Recommendations []string    `json:"recommendations"`
	Category        string      `json:"category,omitempty"`
	Container       string      `json:"container,omitempty"`
	FieldPath       string      `json:"fieldPath,omitempty"`
	Evidence        []Evidence  `json:"evidence,omitempty"`
}

// Evidence is one concrete observation supporting a finding.
type Evidence struct {
	Type      string      `json:"type"`
	Message   string      `json:"message"`
	Source    ResourceRef `json:"source,omitempty"`
	Timestamp time.Time   `json:"timestamp,omitempty"`
}

// EvidenceStep describes one link from the traced root to a diagnosis.
type EvidenceStep struct {
	Source    ResourceRef `json:"source"`
	Relation  string      `json:"relation,omitempty"`
	Condition string      `json:"condition,omitempty"`
	Summary   string      `json:"summary,omitempty"`
}

// Diagnosis separates likely causes from their downstream symptoms.
type Diagnosis struct {
	RootCause          *Finding       `json:"rootCause,omitempty"`
	ContributingCauses []Finding      `json:"contributingCauses,omitempty"`
	Symptoms           []Finding      `json:"symptoms,omitempty"`
	Context            []Finding      `json:"context,omitempty"`
	Confidence         float64        `json:"confidence"`
	EvidenceChain      []EvidenceStep `json:"evidenceChain,omitempty"`
}

// TraceResult is the full output of a ktrace analysis.
type TraceResult struct {
	Root        ResourceRef     `json:"root"`
	Status      HealthStatus    `json:"status"`
	Graph       *ResourceGraph  `json:"graph"`
	Edges       []Edge          `json:"edges,omitempty"`
	Timeline    []TimelineEntry `json:"timeline"`
	Findings    []Finding       `json:"findings"`
	RootCause   *Finding        `json:"rootCause,omitempty"`
	Diagnosis   *Diagnosis      `json:"diagnosis,omitempty"`
	Partial     bool            `json:"partial,omitempty"`
	Warnings    []string        `json:"warnings,omitempty"`
	CollectedAt time.Time       `json:"collectedAt"`
}
