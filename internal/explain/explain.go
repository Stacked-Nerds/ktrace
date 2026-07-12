package explain

import (
	"math"
	"sort"
	"strings"

	"github.com/Stacked-Nerds/ktrace/pkg/models"
)

// RootCause selects the most likely root cause from findings.
func RootCause(findings []models.Finding) *models.Finding {
	return Diagnose(models.ResourceRef{}, findings, nil, nil).RootCause
}

// Diagnose ranks findings using severity, causal specificity, graph position,
// and event ordering. It also separates causes from downstream symptoms.
func Diagnose(
	root models.ResourceRef,
	findings []models.Finding,
	edges []models.Edge,
	timeline []models.TimelineEntry,
) *models.Diagnosis {
	diagnosis := &models.Diagnosis{}
	actionable := make([]models.Finding, 0, len(findings))
	for _, finding := range findings {
		if isInformational(finding) {
			diagnosis.Context = append(diagnosis.Context, finding)
			continue
		}
		actionable = append(actionable, finding)
	}
	if len(actionable) == 0 {
		return diagnosis
	}

	ranked := make([]rankedFinding, 0, len(actionable))
	for i := range actionable {
		ranked = append(ranked, rankedFinding{
			finding: actionable[i],
			score:   findingScore(actionable[i], edges, timeline),
		})
	}
	sort.SliceStable(ranked, func(i, j int) bool {
		if ranked[i].score != ranked[j].score {
			return ranked[i].score > ranked[j].score
		}
		if severityRank(ranked[i].finding.Severity) != severityRank(ranked[j].finding.Severity) {
			return severityRank(ranked[i].finding.Severity) < severityRank(ranked[j].finding.Severity)
		}
		if ranked[i].finding.Source.String() != ranked[j].finding.Source.String() {
			return ranked[i].finding.Source.String() < ranked[j].finding.Source.String()
		}
		return ranked[i].finding.Condition < ranked[j].finding.Condition
	})

	rootCause := ranked[0].finding
	diagnosis.RootCause = &rootCause
	diagnosis.Confidence = confidence(ranked)
	diagnosis.EvidenceChain = evidenceChain(root, rootCause, edges)

	for i := 1; i < len(ranked); i++ {
		if isSymptom(ranked[i].finding.Condition) {
			diagnosis.Symptoms = append(diagnosis.Symptoms, ranked[i].finding)
			continue
		}
		diagnosis.ContributingCauses = append(diagnosis.ContributingCauses, ranked[i].finding)
	}
	return diagnosis
}

// Status derives overall health from findings.
func Status(findings []models.Finding) models.HealthStatus {
	actionable := make([]models.Finding, 0, len(findings))
	for _, finding := range findings {
		if !isInformational(finding) {
			actionable = append(actionable, finding)
		}
	}
	if len(actionable) == 0 {
		return models.StatusHealthy
	}
	rc := RootCause(actionable)
	switch rc.Severity {
	case models.SeverityCritical, models.SeverityHigh:
		return models.StatusFailed
	case models.SeverityMedium:
		return models.StatusDegraded
	default:
		return models.StatusDegraded
	}
}

func isInformational(finding models.Finding) bool {
	return strings.HasPrefix(finding.Condition, "RecentChange") ||
		finding.Condition == "CronJobSuspended"
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

type rankedFinding struct {
	finding models.Finding
	score   float64
}

func findingScore(
	finding models.Finding,
	edges []models.Edge,
	timeline []models.TimelineEntry,
) float64 {
	score := map[models.Severity]float64{
		models.SeverityCritical: 40,
		models.SeverityHigh:     30,
		models.SeverityMedium:   20,
		models.SeverityLow:      10,
	}[finding.Severity]
	score += causalPriority(finding.Condition)

	for _, edge := range edges {
		if sameResource(edge.To, finding.Source) {
			score += 8
		}
		if sameResource(edge.From, finding.Source) {
			score += 2
		}
	}

	for i, entry := range timeline {
		if !sameResource(entry.Source, finding.Source) {
			continue
		}
		if strings.EqualFold(entry.Title, finding.Condition) ||
			strings.Contains(strings.ToLower(entry.Detail), strings.ToLower(finding.Condition)) {
			// Earlier evidence is slightly more likely to be causal.
			score += 10 - 5*float64(i)/float64(max(1, len(timeline)))
			break
		}
	}

	if finding.Explanation != "" {
		score += 2
	}
	score += math.Min(float64(len(finding.Evidence))*2, 6)
	return score
}

func causalPriority(condition string) float64 {
	switch strings.ToLower(condition) {
	case "missingsecret", "missingsecretkey", "missingconfigmap", "missingconfigmapkey",
		"missingserviceaccount", "missingstorageclass", "storageclassnotfound",
		"invalidconfigreference":
		return 55
	case "pvcpending", "provisioningfailed", "nodenotready":
		return 45
	case "failedscheduling", "oomkilled", "jobfailed", "jobfailuretarget",
		"initcontainerfailed", "deadlineexceeded", "backofflimitexceeded":
		return 38
	case "imagepullbackoff", "errimagepull", "invalidimagename",
		"createcontainerconfigerror", "createcontainererror":
		return 32
	case "failedmount", "failedattachvolume", "volumefailedattach":
		return 20
	case "crashloopbackoff", "probefailed", "unhealthy":
		return 12
	case "containersnotready", "deploymentunavailable", "progressdeadlineexceeded":
		return 0
	default:
		return 16
	}
}

func isSymptom(condition string) bool {
	switch strings.ToLower(condition) {
	case "crashloopbackoff", "containersnotready", "deploymentunavailable",
		"progressdeadlineexceeded", "failedmount", "failedattachvolume":
		return true
	default:
		return false
	}
}

func confidence(ranked []rankedFinding) float64 {
	if len(ranked) == 0 {
		return 0
	}
	base := 0.55
	if len(ranked) == 1 {
		base = 0.82
	} else {
		gap := ranked[0].score - ranked[1].score
		base += math.Min(math.Max(gap, 0)/100, 0.25)
	}
	if len(ranked[0].finding.Evidence) > 0 {
		base += 0.05
	}
	return math.Round(math.Min(base, 0.98)*100) / 100
}

func evidenceChain(
	root models.ResourceRef,
	finding models.Finding,
	edges []models.Edge,
) []models.EvidenceStep {
	path := resourcePath(root, finding.Source, edges)
	steps := make([]models.EvidenceStep, 0, len(path)+1)
	for i, ref := range path {
		step := models.EvidenceStep{Source: ref}
		if i > 0 {
			step.Relation = relationBetween(path[i-1], ref, edges)
		}
		steps = append(steps, step)
	}
	if len(steps) == 0 {
		steps = append(steps, models.EvidenceStep{Source: finding.Source})
	}
	steps = append(steps, models.EvidenceStep{
		Source:    finding.Source,
		Relation:  "exhibits",
		Condition: finding.Condition,
		Summary:   finding.Summary,
	})
	return steps
}

func resourcePath(root, target models.ResourceRef, edges []models.Edge) []models.ResourceRef {
	if root.Name == "" {
		return []models.ResourceRef{target}
	}
	if sameResource(root, target) {
		return []models.ResourceRef{root}
	}

	type neighbor struct {
		ref models.ResourceRef
	}
	adj := make(map[string][]neighbor)
	refs := make(map[string]models.ResourceRef)
	for _, edge := range edges {
		fromKey, toKey := resourceKey(edge.From), resourceKey(edge.To)
		refs[fromKey], refs[toKey] = edge.From, edge.To
		adj[fromKey] = append(adj[fromKey], neighbor{ref: edge.To})
		adj[toKey] = append(adj[toKey], neighbor{ref: edge.From})
	}

	start, goal := resourceKey(root), resourceKey(target)
	queue := []string{start}
	previous := map[string]string{start: ""}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		if current == goal {
			break
		}
		for _, next := range adj[current] {
			key := resourceKey(next.ref)
			if _, seen := previous[key]; seen {
				continue
			}
			previous[key] = current
			refs[key] = next.ref
			queue = append(queue, key)
		}
	}
	if _, ok := previous[goal]; !ok {
		return []models.ResourceRef{root, target}
	}

	keys := []string{}
	for key := goal; key != ""; key = previous[key] {
		keys = append(keys, key)
	}
	path := make([]models.ResourceRef, 0, len(keys))
	for i := len(keys) - 1; i >= 0; i-- {
		if keys[i] == start {
			path = append(path, root)
			continue
		}
		path = append(path, refs[keys[i]])
	}
	return path
}

func relationBetween(from, to models.ResourceRef, edges []models.Edge) string {
	for _, edge := range edges {
		if sameResource(edge.From, from) && sameResource(edge.To, to) {
			return edge.Relation
		}
		if sameResource(edge.To, from) && sameResource(edge.From, to) {
			return "reverse:" + edge.Relation
		}
	}
	return "related"
}

func sameResource(a, b models.ResourceRef) bool {
	if a.UID != "" && b.UID != "" {
		return a.UID == b.UID
	}
	return a.Kind == b.Kind && a.Namespace == b.Namespace && a.Name == b.Name
}

func resourceKey(ref models.ResourceRef) string {
	if ref.UID != "" {
		return "uid:" + ref.UID
	}
	return ref.String()
}
