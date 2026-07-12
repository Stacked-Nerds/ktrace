package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/Stacked-Nerds/ktrace/internal/redact"
	"github.com/Stacked-Nerds/ktrace/pkg/models"
)

const summarySchemaVersion = "ktrace.dev/v1alpha1"

type summaryJSON struct {
	SchemaVersion  string                     `json:"schemaVersion"`
	Root           models.ResourceRef         `json:"root"`
	Status         models.HealthStatus        `json:"status"`
	Partial        bool                       `json:"partial,omitempty"`
	Warnings       []string                   `json:"warnings,omitempty"`
	Diagnosis      *models.Diagnosis          `json:"diagnosis,omitempty"`
	Findings       []models.Finding           `json:"findings"`
	Timeline       []models.TimelineEntry     `json:"timeline"`
	Logs           []models.ContainerLog      `json:"logs,omitempty"`
	ResourceCounts map[string]int             `json:"resourceCounts,omitempty"`
	CollectedAt    time.Time                  `json:"collectedAt"`
}

func writeJSON(w io.Writer, result *models.TraceResult, raw ...bool) error {
	if result == nil {
		return fmt.Errorf("nil trace result")
	}
	includeRaw := len(raw) > 0 && raw[0]
	if includeRaw {
		return encodeRedactedJSON(w, sanitizedResult(result))
	}
	return encodeRedactedJSON(w, toSummaryJSON(result))
}

func toSummaryJSON(result *models.TraceResult) summaryJSON {
	summary := summaryJSON{
		SchemaVersion: summarySchemaVersion,
		Root:          result.Root,
		Status:        result.Status,
		Partial:       result.Partial,
		Warnings:      append([]string(nil), result.Warnings...),
		Diagnosis:     result.Diagnosis,
		Findings:      result.Findings,
		Timeline:      result.Timeline,
		CollectedAt:   result.CollectedAt,
	}
	if result.Graph != nil {
		summary.Logs = result.Graph.Logs
		summary.ResourceCounts = make(map[string]int, len(result.Graph.Resources))
		for kind, resources := range result.Graph.Resources {
			summary.ResourceCounts[kind] = len(resources)
		}
	}
	return summary
}

func sanitizedResult(result *models.TraceResult) *models.TraceResult {
	if result == nil {
		return nil
	}
	data, err := json.Marshal(result)
	if err != nil {
		return result
	}
	var clone models.TraceResult
	if err := json.Unmarshal(data, &clone); err != nil {
		return result
	}
	if clone.Graph != nil {
		for kind, resources := range clone.Graph.Resources {
			for i := range resources {
				resources[i].Raw = redact.JSON(resources[i].Raw)
			}
			clone.Graph.Resources[kind] = resources
		}
	}
	return &clone
}

func encodeRedactedJSON(w io.Writer, value interface{}) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	data = redact.JSON(data)
	var formatted bytes.Buffer
	if err := json.Indent(&formatted, data, "", "  "); err != nil {
		return err
	}
	formatted.WriteByte('\n')
	_, err = w.Write(formatted.Bytes())
	return err
}
