package analyzer

import (
	"encoding/json"
	"fmt"
	"sort"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"

	"github.com/Stacked-Nerds/ktrace/pkg/models"
)

type podSpecSource struct {
	ref    models.ResourceRef
	spec   corev1.PodSpec
	prefix string
}

type objectReference struct {
	kind      string
	name      string
	namespace string
	fieldPath string
	optional  bool
	source    models.ResourceRef
	container string
}

func analyzeReferences(graph *models.ResourceGraph) []models.Finding {
	refs := append([]models.ResourceReference(nil), graph.References...)
	sort.Slice(refs, func(i, j int) bool {
		left := refs[i].To.String() + "|" + refs[i].From.String() + "|" +
			refs[i].FieldPath + "|" + refs[i].Key
		right := refs[j].To.String() + "|" + refs[j].From.String() + "|" +
			refs[j].FieldPath + "|" + refs[j].Key
		return left < right
	})

	findings := make([]models.Finding, 0)
	findingIndex := make(map[string]int)
	for _, ref := range refs {
		if !ref.Observed || ref.Optional || ref.Exists {
			continue
		}
		condition := "Missing" + ref.To.Kind
		missing := ref.To.String()
		explanation := fmt.Sprintf(
			"The non-optional reference at %s does not match an existing %s",
			ref.FieldPath,
			ref.To.Kind,
		)
		if ref.TargetExists && ref.Key != "" {
			condition += "Key"
			missing += " key " + ref.Key
			explanation = fmt.Sprintf(
				"%s exists, but key %q referenced at %s does not",
				ref.To.String(),
				ref.Key,
				ref.FieldPath,
			)
		}
		evidence := models.Evidence{
			Type:    "SpecReference",
			Message: fmt.Sprintf("%s references %s at %s", ref.From.String(), ref.To.String(), ref.FieldPath),
			Source:  ref.From,
		}
		key := condition + "|" + ref.To.String() + "|" + ref.Key
		if index, ok := findingIndex[key]; ok {
			findings[index].Evidence = append(findings[index].Evidence, evidence)
			continue
		}
		findingIndex[key] = len(findings)
		findings = append(findings, models.Finding{
			Severity:    models.SeverityHigh,
			Condition:   condition,
			Summary:     fmt.Sprintf("%s references missing %s", ref.From.String(), missing),
			Explanation: explanation,
			Source:      ref.To,
			FieldPath:   ref.FieldPath,
			Category:    "Reference",
			Evidence:    []models.Evidence{evidence},
			Recommendations: []string{
				fmt.Sprintf("Verify %s %q exists with the required key", ref.To.Kind, ref.To.Name),
				fmt.Sprintf("Inspect %s", ref.From.String()),
			},
		})
	}
	return findings
}

func collectPodSpecs(graph *models.ResourceGraph) []podSpecSource {
	sources := make([]podSpecSource, 0)
	for _, cr := range graph.Resources["Pod"] {
		var pod corev1.Pod
		if json.Unmarshal(cr.Raw, &pod) == nil {
			sources = append(sources, podSpecSource{
				ref: models.ResourceRef{Kind: "Pod", Name: pod.Name, Namespace: pod.Namespace},
				spec: pod.Spec, prefix: "spec",
			})
		}
	}
	for _, cr := range graph.Resources["Deployment"] {
		var workload appsv1.Deployment
		if json.Unmarshal(cr.Raw, &workload) == nil {
			sources = append(sources, podSpecSource{
				ref: models.ResourceRef{Kind: "Deployment", Name: workload.Name, Namespace: workload.Namespace},
				spec: workload.Spec.Template.Spec, prefix: "spec.template.spec",
			})
		}
	}
	for _, cr := range graph.Resources["ReplicaSet"] {
		var workload appsv1.ReplicaSet
		if json.Unmarshal(cr.Raw, &workload) == nil {
			sources = append(sources, podSpecSource{
				ref: models.ResourceRef{Kind: "ReplicaSet", Name: workload.Name, Namespace: workload.Namespace},
				spec: workload.Spec.Template.Spec, prefix: "spec.template.spec",
			})
		}
	}
	for _, cr := range graph.Resources["StatefulSet"] {
		var workload appsv1.StatefulSet
		if json.Unmarshal(cr.Raw, &workload) == nil {
			sources = append(sources, podSpecSource{
				ref: models.ResourceRef{Kind: "StatefulSet", Name: workload.Name, Namespace: workload.Namespace},
				spec: workload.Spec.Template.Spec, prefix: "spec.template.spec",
			})
		}
	}
	for _, cr := range graph.Resources["DaemonSet"] {
		var workload appsv1.DaemonSet
		if json.Unmarshal(cr.Raw, &workload) == nil {
			sources = append(sources, podSpecSource{
				ref: models.ResourceRef{Kind: "DaemonSet", Name: workload.Name, Namespace: workload.Namespace},
				spec: workload.Spec.Template.Spec, prefix: "spec.template.spec",
			})
		}
	}
	for _, cr := range graph.Resources["Job"] {
		var workload batchv1.Job
		if json.Unmarshal(cr.Raw, &workload) == nil {
			sources = append(sources, podSpecSource{
				ref: models.ResourceRef{Kind: "Job", Name: workload.Name, Namespace: workload.Namespace},
				spec: workload.Spec.Template.Spec, prefix: "spec.template.spec",
			})
		}
	}
	for _, cr := range graph.Resources["CronJob"] {
		var workload batchv1.CronJob
		if json.Unmarshal(cr.Raw, &workload) == nil {
			sources = append(sources, podSpecSource{
				ref: models.ResourceRef{Kind: "CronJob", Name: workload.Name, Namespace: workload.Namespace},
				spec: workload.Spec.JobTemplate.Spec.Template.Spec, prefix: "spec.jobTemplate.spec.template.spec",
			})
		}
	}
	return sources
}

func podSpecReferences(source podSpecSource) []objectReference {
	refs := make([]objectReference, 0)
	namespace := source.ref.Namespace
	add := func(kind, name, path, container string, optional bool) {
		refs = append(refs, objectReference{
			kind: kind, name: name, namespace: namespace, fieldPath: source.prefix + "." + path,
			optional: optional, source: source.ref, container: container,
		})
	}

	if source.spec.ServiceAccountName != "" {
		add("ServiceAccount", source.spec.ServiceAccountName, "serviceAccountName", "", false)
	}
	for i, pullSecret := range source.spec.ImagePullSecrets {
		add("Secret", pullSecret.Name, fmt.Sprintf("imagePullSecrets[%d].name", i), "", false)
	}
	for i, volume := range source.spec.Volumes {
		base := fmt.Sprintf("volumes[%d]", i)
		if volume.ConfigMap != nil {
			add("ConfigMap", volume.ConfigMap.Name, base+".configMap.name", "", boolValue(volume.ConfigMap.Optional))
		}
		if volume.Secret != nil {
			add("Secret", volume.Secret.SecretName, base+".secret.secretName", "", boolValue(volume.Secret.Optional))
		}
		if volume.Projected != nil {
			for j, projection := range volume.Projected.Sources {
				projected := fmt.Sprintf("%s.projected.sources[%d]", base, j)
				if projection.ConfigMap != nil {
					add("ConfigMap", projection.ConfigMap.Name, projected+".configMap.name", "", boolValue(projection.ConfigMap.Optional))
				}
				if projection.Secret != nil {
					add("Secret", projection.Secret.Name, projected+".secret.name", "", boolValue(projection.Secret.Optional))
				}
			}
		}
	}

	containerGroups := []struct {
		name       string
		containers []corev1.Container
	}{
		{name: "containers", containers: source.spec.Containers},
		{name: "initContainers", containers: source.spec.InitContainers},
	}
	for _, group := range containerGroups {
		for i, container := range group.containers {
			base := fmt.Sprintf("%s[%d]", group.name, i)
			for j, envFrom := range container.EnvFrom {
				if envFrom.ConfigMapRef != nil {
					add("ConfigMap", envFrom.ConfigMapRef.Name, fmt.Sprintf("%s.envFrom[%d].configMapRef.name", base, j), container.Name, boolValue(envFrom.ConfigMapRef.Optional))
				}
				if envFrom.SecretRef != nil {
					add("Secret", envFrom.SecretRef.Name, fmt.Sprintf("%s.envFrom[%d].secretRef.name", base, j), container.Name, boolValue(envFrom.SecretRef.Optional))
				}
			}
			for j, env := range container.Env {
				if env.ValueFrom == nil {
					continue
				}
				if env.ValueFrom.ConfigMapKeyRef != nil {
					add("ConfigMap", env.ValueFrom.ConfigMapKeyRef.Name, fmt.Sprintf("%s.env[%d].valueFrom.configMapKeyRef.name", base, j), container.Name, boolValue(env.ValueFrom.ConfigMapKeyRef.Optional))
				}
				if env.ValueFrom.SecretKeyRef != nil {
					add("Secret", env.ValueFrom.SecretKeyRef.Name, fmt.Sprintf("%s.env[%d].valueFrom.secretKeyRef.name", base, j), container.Name, boolValue(env.ValueFrom.SecretKeyRef.Optional))
				}
			}
		}
	}
	for i, container := range source.spec.EphemeralContainers {
		base := fmt.Sprintf("ephemeralContainers[%d]", i)
		for j, envFrom := range container.EnvFrom {
			if envFrom.ConfigMapRef != nil {
				add("ConfigMap", envFrom.ConfigMapRef.Name, fmt.Sprintf("%s.envFrom[%d].configMapRef.name", base, j), container.Name, boolValue(envFrom.ConfigMapRef.Optional))
			}
			if envFrom.SecretRef != nil {
				add("Secret", envFrom.SecretRef.Name, fmt.Sprintf("%s.envFrom[%d].secretRef.name", base, j), container.Name, boolValue(envFrom.SecretRef.Optional))
			}
		}
		for j, env := range container.Env {
			if env.ValueFrom == nil {
				continue
			}
			if env.ValueFrom.ConfigMapKeyRef != nil {
				add("ConfigMap", env.ValueFrom.ConfigMapKeyRef.Name, fmt.Sprintf("%s.env[%d].valueFrom.configMapKeyRef.name", base, j), container.Name, boolValue(env.ValueFrom.ConfigMapKeyRef.Optional))
			}
			if env.ValueFrom.SecretKeyRef != nil {
				add("Secret", env.ValueFrom.SecretKeyRef.Name, fmt.Sprintf("%s.env[%d].valueFrom.secretKeyRef.name", base, j), container.Name, boolValue(env.ValueFrom.SecretKeyRef.Optional))
			}
		}
	}
	return refs
}

func boolValue(value *bool) bool {
	return value != nil && *value
}
