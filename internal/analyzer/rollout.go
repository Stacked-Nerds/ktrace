package analyzer

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/Stacked-Nerds/ktrace/pkg/models"
)

const revisionAnnotation = "deployment.kubernetes.io/revision"

type replicaSetRevision struct {
	replicaSet appsv1.ReplicaSet
	revision   int64
	owner      string
}

type rolloutComparison struct {
	condition string
	summary   string
	fieldPath string
	changed   bool
}

func analyzeRollouts(graph *models.ResourceGraph) []models.Finding {
	findings := make([]models.Finding, 0)
	deployments := make(map[string]appsv1.Deployment)

	for _, cr := range graph.Resources["Deployment"] {
		var deployment appsv1.Deployment
		if json.Unmarshal(cr.Raw, &deployment) != nil {
			continue
		}
		key := namespacedName(deployment.Namespace, deployment.Name)
		deployments[key] = deployment
		ref := models.ResourceRef{Kind: "Deployment", Name: deployment.Name, Namespace: deployment.Namespace}
		if deployment.Generation > 0 && deployment.Status.ObservedGeneration < deployment.Generation {
			findings = append(findings, generationLagFinding(
				ref, deployment.Generation, deployment.Status.ObservedGeneration,
			))
		}
		for _, condition := range deployment.Status.Conditions {
			if condition.Type == appsv1.DeploymentProgressing &&
				condition.Status == corev1.ConditionFalse &&
				condition.Reason == "ProgressDeadlineExceeded" {
				findings = append(findings, models.Finding{
					Severity:    models.SeverityHigh,
					Condition:   "StalledRollout",
					Summary:     fmt.Sprintf("Deployment %q rollout is stalled", deployment.Name),
					Explanation: condition.Message,
					Source:      ref,
					FieldPath:   "status.conditions[Progressing]",
					Category:    "Rollout",
					Recommendations: []string{
						fmt.Sprintf("kubectl rollout status deployment/%s -n %s", deployment.Name, deployment.Namespace),
						fmt.Sprintf("kubectl describe deployment %s -n %s", deployment.Name, deployment.Namespace),
						fmt.Sprintf("kubectl get rs,pods -n %s", deployment.Namespace),
					},
				})
			}
		}
	}

	revisionsByDeployment := make(map[string][]replicaSetRevision)
	for _, cr := range graph.Resources["ReplicaSet"] {
		var replicaSet appsv1.ReplicaSet
		if json.Unmarshal(cr.Raw, &replicaSet) != nil {
			continue
		}
		ref := models.ResourceRef{Kind: "ReplicaSet", Name: replicaSet.Name, Namespace: replicaSet.Namespace}
		if replicaSet.Generation > 0 && replicaSet.Status.ObservedGeneration < replicaSet.Generation {
			findings = append(findings, generationLagFinding(
				ref, replicaSet.Generation, replicaSet.Status.ObservedGeneration,
			))
		}
		revision, err := strconv.ParseInt(replicaSet.Annotations[revisionAnnotation], 10, 64)
		owner := deploymentOwner(replicaSet.OwnerReferences)
		if err != nil || revision <= 0 || owner == "" {
			continue
		}
		key := namespacedName(replicaSet.Namespace, owner)
		revisionsByDeployment[key] = append(revisionsByDeployment[key], replicaSetRevision{
			replicaSet: replicaSet, revision: revision, owner: owner,
		})
	}

	keys := make([]string, 0, len(revisionsByDeployment))
	for key := range revisionsByDeployment {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		revisions := revisionsByDeployment[key]
		sort.Slice(revisions, func(i, j int) bool {
			if revisions[i].revision != revisions[j].revision {
				return revisions[i].revision > revisions[j].revision
			}
			return revisions[i].replicaSet.Name < revisions[j].replicaSet.Name
		})
		current, previous, ok := twoNewestDistinctRevisions(revisions)
		if !ok {
			continue
		}
		deployment, exists := deployments[key]
		source := models.ResourceRef{
			Kind: "Deployment", Name: current.owner, Namespace: current.replicaSet.Namespace,
		}
		if exists {
			source.UID = string(deployment.UID)
		}
		if !rolloutHasFailure(graph, current.replicaSet, deployment, exists) {
			continue
		}
		changes := compareReplicaSetRevisions(source, current, previous)
		failureEvidence := currentRevisionFailureEvidence(graph, current.replicaSet)
		for i := range changes {
			changes[i].Evidence = append(changes[i].Evidence, failureEvidence...)
			if len(failureEvidence) > 0 {
				changes[i].Explanation += "; this change is correlated with failures in the current revision, not proven as their cause"
			}
		}
		findings = append(findings, changes...)
	}
	return findings
}

func currentRevisionFailureEvidence(
	graph *models.ResourceGraph,
	current appsv1.ReplicaSet,
) []models.Evidence {
	evidence := make([]models.Evidence, 0)
	for _, resource := range graph.Resources["Pod"] {
		var pod corev1.Pod
		if json.Unmarshal(resource.Raw, &pod) != nil {
			continue
		}
		owned := false
		for _, owner := range pod.OwnerReferences {
			if replicaSetOwnerMatches(owner, current) {
				owned = true
				break
			}
		}
		if !owned {
			continue
		}
		for _, status := range pod.Status.ContainerStatuses {
			if status.State.Waiting != nil {
				evidence = append(evidence, models.Evidence{
					Type:    "CurrentRevisionFailure",
					Message: status.Name + ": " + status.State.Waiting.Reason,
					Source:  resource.Ref,
				})
			}
			if terminated := status.LastTerminationState.Terminated; terminated != nil && terminated.ExitCode != 0 {
				evidence = append(evidence, models.Evidence{
					Type:    "CurrentRevisionFailure",
					Message: fmt.Sprintf("%s: %s (exit %d)", status.Name, terminated.Reason, terminated.ExitCode),
					Source:  resource.Ref,
				})
			}
		}
	}
	if len(evidence) > 5 {
		evidence = evidence[:5]
	}
	return evidence
}

func rolloutHasFailure(
	graph *models.ResourceGraph,
	current appsv1.ReplicaSet,
	deployment appsv1.Deployment,
	hasDeployment bool,
) bool {
	if hasDeployment {
		for _, condition := range deployment.Status.Conditions {
			if condition.Status == corev1.ConditionFalse &&
				(condition.Type == appsv1.DeploymentAvailable || condition.Type == appsv1.DeploymentProgressing) {
				return true
			}
		}
		if deployment.Status.UnavailableReplicas > 0 {
			return true
		}
	}
	for _, resource := range graph.Resources["Pod"] {
		var pod corev1.Pod
		if json.Unmarshal(resource.Raw, &pod) != nil {
			continue
		}
		owned := false
		for _, owner := range pod.OwnerReferences {
			if replicaSetOwnerMatches(owner, current) {
				owned = true
				break
			}
		}
		if !owned {
			continue
		}
		if pod.Status.Phase == corev1.PodFailed {
			return true
		}
		for _, status := range pod.Status.ContainerStatuses {
			if status.RestartCount > 0 || status.State.Waiting != nil ||
				(status.State.Terminated != nil && status.State.Terminated.ExitCode != 0) {
				return true
			}
		}
		for _, condition := range pod.Status.Conditions {
			if condition.Type == corev1.PodReady && condition.Status == corev1.ConditionFalse {
				return true
			}
		}
	}
	return false
}

func replicaSetOwnerMatches(owner metav1.OwnerReference, replicaSet appsv1.ReplicaSet) bool {
	if owner.Kind != "ReplicaSet" {
		return false
	}
	if owner.UID != "" && replicaSet.UID != "" {
		return owner.UID == replicaSet.UID
	}
	return owner.Name == replicaSet.Name
}

func generationLagFinding(ref models.ResourceRef, generation, observed int64) models.Finding {
	return models.Finding{
		Severity:    models.SeverityMedium,
		Condition:   "ObservedGenerationLag",
		Summary:     fmt.Sprintf("%s controller has not observed the latest generation", ref.String()),
		Explanation: fmt.Sprintf("metadata.generation is %d but status.observedGeneration is %d", generation, observed),
		Source:      ref,
		FieldPath:   "status.observedGeneration",
		Category:    "Rollout",
		Recommendations: []string{
			fmt.Sprintf("Inspect %s status and controller events", ref.String()),
		},
	}
}

func deploymentOwner(owners []metav1.OwnerReference) string {
	for _, owner := range owners {
		if owner.Kind == "Deployment" {
			return owner.Name
		}
	}
	return ""
}

func twoNewestDistinctRevisions(revisions []replicaSetRevision) (replicaSetRevision, replicaSetRevision, bool) {
	if len(revisions) < 2 {
		return replicaSetRevision{}, replicaSetRevision{}, false
	}
	current := revisions[0]
	for _, candidate := range revisions[1:] {
		if candidate.revision != current.revision {
			return current, candidate, true
		}
	}
	return replicaSetRevision{}, replicaSetRevision{}, false
}

func compareReplicaSetRevisions(source models.ResourceRef, current, previous replicaSetRevision) []models.Finding {
	currentSpec := current.replicaSet.Spec.Template.Spec
	previousSpec := previous.replicaSet.Spec.Template.Spec
	comparisons := []rolloutComparison{
		{
			condition: "RecentChangeImages",
			summary:   "Container images changed in the latest revision",
			fieldPath: "spec.template.spec.containers[*].image",
			changed:   !reflect.DeepEqual(containerImages(currentSpec), containerImages(previousSpec)),
		},
		{
			condition: "RecentChangeResources",
			summary:   "Container resource requests or limits changed in the latest revision",
			fieldPath: "spec.template.spec.containers[*].resources",
			changed:   !reflect.DeepEqual(containerResources(currentSpec), containerResources(previousSpec)),
		},
		{
			condition: "RecentChangeMemoryLimitDecreased",
			summary:   "A container memory limit decreased in the failing revision",
			fieldPath: "spec.template.spec.containers[*].resources.limits.memory",
			changed:   memoryLimitDecreased(currentSpec, previousSpec),
		},
		{
			condition: "RecentChangeProbes",
			summary:   "Container probes changed in the latest revision",
			fieldPath: "spec.template.spec.containers[*].probes",
			changed:   !reflect.DeepEqual(containerProbes(currentSpec), containerProbes(previousSpec)),
		},
		{
			condition: "RecentChangeConfigReferences",
			summary:   "ConfigMap or Secret references changed in the latest revision",
			fieldPath: "spec.template.spec",
			changed:   !reflect.DeepEqual(configReferences(currentSpec), configReferences(previousSpec)),
		},
		{
			condition: "RecentChangeServiceAccount",
			summary:   "Service account changed in the latest revision",
			fieldPath: "spec.template.spec.serviceAccountName",
			changed:   currentSpec.ServiceAccountName != previousSpec.ServiceAccountName,
		},
		{
			condition: "RecentChangeSchedulingConstraints",
			summary:   "Scheduling constraints changed in the latest revision",
			fieldPath: "spec.template.spec",
			changed:   !reflect.DeepEqual(schedulingConstraints(currentSpec), schedulingConstraints(previousSpec)),
		},
	}

	findings := make([]models.Finding, 0)
	for _, comparison := range comparisons {
		if !comparison.changed {
			continue
		}
		explanation := fmt.Sprintf(
			"Deployment revision %d (%s) differs from revision %d (%s)",
			current.revision, current.replicaSet.Name, previous.revision, previous.replicaSet.Name,
		)
		findings = append(findings, models.Finding{
			Severity:    models.SeverityLow,
			Condition:   comparison.condition,
			Summary:     comparison.summary,
			Explanation: explanation,
			Source:      source,
			FieldPath:   comparison.fieldPath,
			Category:    "Correlation",
			Evidence: []models.Evidence{
				{Type: "CurrentRevision", Message: fmt.Sprintf("ReplicaSet/%s revision %d", current.replicaSet.Name, current.revision), Source: models.ResourceRef{Kind: "ReplicaSet", Name: current.replicaSet.Name, Namespace: current.replicaSet.Namespace}},
				{Type: "PreviousRevision", Message: fmt.Sprintf("ReplicaSet/%s revision %d", previous.replicaSet.Name, previous.revision), Source: models.ResourceRef{Kind: "ReplicaSet", Name: previous.replicaSet.Name, Namespace: previous.replicaSet.Namespace}},
			},
			Recommendations: []string{
				fmt.Sprintf("Review rollout history for deployment/%s", source.Name),
				"Correlate the changed field with the first failing pod or event",
			},
		})
	}
	return findings
}

func memoryLimitDecreased(current, previous corev1.PodSpec) bool {
	previousLimits := make(map[string]resource.Quantity)
	for _, container := range previous.Containers {
		if value, ok := container.Resources.Limits[corev1.ResourceMemory]; ok {
			previousLimits[container.Name] = value
		}
	}
	for _, container := range current.Containers {
		currentLimit, currentSet := container.Resources.Limits[corev1.ResourceMemory]
		previousLimit, previousSet := previousLimits[container.Name]
		if currentSet && previousSet && currentLimit.Cmp(previousLimit) < 0 {
			return true
		}
	}
	return false
}

func containerImages(spec corev1.PodSpec) []string {
	values := make([]string, 0, len(spec.InitContainers)+len(spec.Containers))
	for _, container := range spec.InitContainers {
		values = append(values, "init/"+container.Name+"="+container.Image)
	}
	for _, container := range spec.Containers {
		values = append(values, "container/"+container.Name+"="+container.Image)
	}
	sort.Strings(values)
	return values
}

func containerResources(spec corev1.PodSpec) map[string]corev1.ResourceRequirements {
	values := make(map[string]corev1.ResourceRequirements)
	for _, container := range spec.InitContainers {
		values["init/"+container.Name] = container.Resources
	}
	for _, container := range spec.Containers {
		values["container/"+container.Name] = container.Resources
	}
	return values
}

type probeSet struct {
	Startup   *corev1.Probe
	Readiness *corev1.Probe
	Liveness  *corev1.Probe
}

func containerProbes(spec corev1.PodSpec) map[string]probeSet {
	values := make(map[string]probeSet)
	for _, container := range spec.InitContainers {
		values["init/"+container.Name] = probeSet{
			Startup: container.StartupProbe, Readiness: container.ReadinessProbe, Liveness: container.LivenessProbe,
		}
	}
	for _, container := range spec.Containers {
		values["container/"+container.Name] = probeSet{
			Startup: container.StartupProbe, Readiness: container.ReadinessProbe, Liveness: container.LivenessProbe,
		}
	}
	return values
}

func configReferences(spec corev1.PodSpec) []string {
	source := podSpecSource{spec: spec}
	refs := podSpecReferences(source)
	values := make([]string, 0, len(refs))
	for _, ref := range refs {
		if ref.kind != "ConfigMap" && ref.kind != "Secret" {
			continue
		}
		values = append(values, strings.Join([]string{
			ref.kind, ref.name, ref.fieldPath, strconv.FormatBool(ref.optional),
		}, "|"))
	}
	sort.Strings(values)
	return values
}

type podSchedulingConstraints struct {
	NodeSelector              map[string]string
	Affinity                  *corev1.Affinity
	Tolerations               []corev1.Toleration
	TopologySpreadConstraints []corev1.TopologySpreadConstraint
	SchedulerName             string
	PriorityClassName         string
	RuntimeClassName          *string
	HostNetwork               bool
}

func schedulingConstraints(spec corev1.PodSpec) podSchedulingConstraints {
	return podSchedulingConstraints{
		NodeSelector:              spec.NodeSelector,
		Affinity:                  spec.Affinity,
		Tolerations:               spec.Tolerations,
		TopologySpreadConstraints: spec.TopologySpreadConstraints,
		SchedulerName:             spec.SchedulerName,
		PriorityClassName:         spec.PriorityClassName,
		RuntimeClassName:          spec.RuntimeClassName,
		HostNetwork:               spec.HostNetwork,
	}
}

func namespacedName(namespace, name string) string {
	return namespace + "/" + name
}
