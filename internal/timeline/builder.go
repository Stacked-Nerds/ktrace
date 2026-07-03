package timeline

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Stacked-Nerds/ktrace/pkg/models"
)

// Build constructs a deduplicated chronological timeline from the resource graph.
func Build(graph *models.ResourceGraph) []models.TimelineEntry {
	if graph == nil {
		return nil
	}

	entries := make([]models.TimelineEntry, 0, len(graph.Events)+16)
	entries = append(entries, resourceLifecycleEntries(graph)...)
	entries = append(entries, eventEntries(graph.Events)...)

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Timestamp.Before(entries[j].Timestamp)
	})

	return dedupe(entries)
}

func resourceLifecycleEntries(graph *models.ResourceGraph) []models.TimelineEntry {
	entries := make([]models.TimelineEntry, 0)
	kindOrder := []string{"Namespace", "Deployment", "ReplicaSet", "PersistentVolumeClaim", "PersistentVolume", "Pod", "Service", "Node"}

	for _, kind := range kindOrder {
		for _, r := range graph.Resources[kind] {
			if r.Metadata.CreationTimestamp.IsZero() {
				continue
			}
			entries = append(entries, models.TimelineEntry{
				Timestamp: r.Metadata.CreationTimestamp,
				Title:     fmt.Sprintf("%s created", kind),
				Detail:    r.Ref.Name,
				Source:    r.Ref,
			})
		}
	}
	return entries
}

func eventEntries(events []models.TimelineEvent) []models.TimelineEntry {
	entries := make([]models.TimelineEntry, 0, len(events))
	for _, ev := range events {
		if ev.Timestamp.IsZero() {
			continue
		}
		title := ev.Reason
		if title == "" {
			title = ev.Type
		}
		sev := models.SeverityLow
		if ev.Type == "Warning" {
			sev = models.SeverityMedium
		}
		entries = append(entries, models.TimelineEntry{
			Timestamp: ev.Timestamp,
			Title:     title,
			Detail:    strings.TrimSpace(ev.Message),
			Source:    ev.Source,
			Severity:  sev,
		})
	}
	return entries
}

func dedupe(entries []models.TimelineEntry) []models.TimelineEntry {
	if len(entries) == 0 {
		return entries
	}
	out := make([]models.TimelineEntry, 0, len(entries))
	var prev *models.TimelineEntry

	for i := range entries {
		e := entries[i]
		if prev != nil && isDuplicate(*prev, e) {
			continue
		}
		out = append(out, e)
		prev = &out[len(out)-1]
	}
	return out
}

func isDuplicate(a, b models.TimelineEntry) bool {
	if a.Title != b.Title || a.Source.Name != b.Source.Name || a.Source.Kind != b.Source.Kind {
		return false
	}
	return a.Detail == b.Detail && b.Timestamp.Sub(a.Timestamp) < time.Minute
}
