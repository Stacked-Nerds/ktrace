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
	CollectedAt time.Time       `json:"collectedAt"`
}
