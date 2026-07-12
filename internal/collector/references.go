package collector

import (
	"context"
	"encoding/json"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/Stacked-Nerds/ktrace/internal/kubernetes"
	"github.com/Stacked-Nerds/ktrace/pkg/models"
)

type referenceLookup struct {
	observed bool
	exists   bool
	keys     map[string]bool
	resource *models.CollectedResource
	err      error
}

// collectConfigurationReferences records and resolves pod configuration
// dependencies. Secret values and ConfigMap values are never retained.
func collectConfigurationReferences(
	ctx context.Context,
	client *kubernetes.Client,
	state *collectState,
) {
	references := extractConfigurationReferences(state.resources("Pod"))
	references = append(references, extractWorkloadConfigurationReferences(state)...)
	references = append(references, extractStorageReferences(state.resources("PersistentVolumeClaim"))...)
	const referenceBudget = 2000
	if len(references) > referenceBudget {
		references = references[:referenceBudget]
		state.warn("configuration reference budget reached; results may be incomplete")
	}
	cache := make(map[string]referenceLookup)

	resolve := func(ref *models.ResourceReference) {
		key := ref.To.String()
		lookup, ok := cache[key]
		if !ok {
			lookup = lookupReference(ctx, client, ref.To)
			cache[key] = lookup
		}
		if lookup.err != nil {
			state.warn(fmt.Sprintf("inspect %s: %v", ref.To.String(), lookup.err))
			return
		}
		ref.Observed = lookup.observed
		ref.TargetExists = lookup.exists
		ref.Exists = lookup.exists
		if ref.Key != "" && lookup.exists {
			ref.Exists = lookup.keys[ref.Key]
		}
		if lookup.resource != nil {
			state.add(*lookup.resource)
		}
	}
	for i := range references {
		resolve(&references[i])
	}
	serviceAccountRefs := extractServiceAccountReferences(state.resources("ServiceAccount"))
	for i := range serviceAccountRefs {
		resolve(&serviceAccountRefs[i])
	}
	references = append(references, serviceAccountRefs...)

	state.mu.Lock()
	state.graph.References = append(state.graph.References, references...)
	state.mu.Unlock()
}

func extractServiceAccountReferences(resources []models.CollectedResource) []models.ResourceReference {
	references := make([]models.ResourceReference, 0)
	for _, resource := range resources {
		var account corev1.ServiceAccount
		if decodeRaw(resource.Raw, &account) != nil {
			continue
		}
		for i, secret := range account.ImagePullSecrets {
			references = append(references, models.ResourceReference{
				From: resource.Ref,
				To: models.ResourceRef{
					Kind: "Secret", Name: secret.Name, Namespace: account.Namespace,
				},
				FieldPath: fmt.Sprintf("imagePullSecrets[%d]", i),
			})
		}
	}
	return references
}

func extractWorkloadConfigurationReferences(state *collectState) []models.ResourceReference {
	references := make([]models.ResourceReference, 0)
	addTemplate := func(ref models.ResourceRef, namespace string, spec corev1.PodSpec, prefix string) {
		pod := corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Namespace: namespace},
			Spec:       spec,
		}
		raw, err := json.Marshal(&pod)
		if err != nil {
			return
		}
		extracted := extractConfigurationReferences([]models.CollectedResource{{Ref: ref, Raw: raw}})
		for i := range extracted {
			extracted[i].FieldPath = prefix + extracted[i].FieldPath[len("spec"):]
		}
		references = append(references, extracted...)
	}

	for _, resource := range state.resources("Deployment") {
		var object appsv1.Deployment
		if decodeRaw(resource.Raw, &object) == nil {
			addTemplate(resource.Ref, object.Namespace, object.Spec.Template.Spec, "spec.template.spec")
		}
	}
	for _, resource := range state.resources("ReplicaSet") {
		var object appsv1.ReplicaSet
		if decodeRaw(resource.Raw, &object) == nil {
			addTemplate(resource.Ref, object.Namespace, object.Spec.Template.Spec, "spec.template.spec")
		}
	}
	for _, resource := range state.resources("StatefulSet") {
		var object appsv1.StatefulSet
		if decodeRaw(resource.Raw, &object) == nil {
			addTemplate(resource.Ref, object.Namespace, object.Spec.Template.Spec, "spec.template.spec")
		}
	}
	for _, resource := range state.resources("DaemonSet") {
		var object appsv1.DaemonSet
		if decodeRaw(resource.Raw, &object) == nil {
			addTemplate(resource.Ref, object.Namespace, object.Spec.Template.Spec, "spec.template.spec")
		}
	}
	for _, resource := range state.resources("Job") {
		var object batchv1.Job
		if decodeRaw(resource.Raw, &object) == nil {
			addTemplate(resource.Ref, object.Namespace, object.Spec.Template.Spec, "spec.template.spec")
		}
	}
	for _, resource := range state.resources("CronJob") {
		var object batchv1.CronJob
		if decodeRaw(resource.Raw, &object) == nil {
			addTemplate(
				resource.Ref,
				object.Namespace,
				object.Spec.JobTemplate.Spec.Template.Spec,
				"spec.jobTemplate.spec.template.spec",
			)
		}
	}
	return references
}

func extractStorageReferences(resources []models.CollectedResource) []models.ResourceReference {
	references := make([]models.ResourceReference, 0)
	for _, resource := range resources {
		var pvc corev1.PersistentVolumeClaim
		if err := decodeRaw(resource.Raw, &pvc); err != nil || pvc.Spec.StorageClassName == nil {
			continue
		}
		references = append(references, models.ResourceReference{
			From: resource.Ref,
			To: models.ResourceRef{
				Kind: "StorageClass",
				Name: *pvc.Spec.StorageClassName,
			},
			FieldPath: "spec.storageClassName",
		})
	}
	return references
}

func extractConfigurationReferences(resources []models.CollectedResource) []models.ResourceReference {
	references := make([]models.ResourceReference, 0)
	seen := make(map[string]bool)
	add := func(ref models.ResourceReference) {
		key := ref.From.String() + "|" + ref.To.String() + "|" + ref.FieldPath + "|" + ref.Key
		if seen[key] {
			return
		}
		seen[key] = true
		references = append(references, ref)
	}

	for _, resource := range resources {
		var pod corev1.Pod
		if err := decodeRaw(resource.Raw, &pod); err != nil {
			continue
		}
		from := resource.Ref
		if from.UID == "" {
			from.UID = string(pod.UID)
		}
		podRef := func(kind, name, field, key string, optional bool) models.ResourceReference {
			return models.ResourceReference{
				From: from,
				To: models.ResourceRef{
					Kind: kind, Name: name, Namespace: pod.Namespace,
				},
				FieldPath: field,
				Key:       key,
				Optional:  optional,
			}
		}

		if pod.Spec.ServiceAccountName != "" {
			add(podRef("ServiceAccount", pod.Spec.ServiceAccountName, "spec.serviceAccountName", "", false))
		}
		for i, pullSecret := range pod.Spec.ImagePullSecrets {
			add(podRef("Secret", pullSecret.Name, fmt.Sprintf("spec.imagePullSecrets[%d]", i), "", false))
		}
		for i, volume := range pod.Spec.Volumes {
			if volume.ConfigMap != nil {
				optional := volume.ConfigMap.Optional != nil && *volume.ConfigMap.Optional
				if len(volume.ConfigMap.Items) == 0 {
					add(podRef("ConfigMap", volume.ConfigMap.Name, fmt.Sprintf("spec.volumes[%d].configMap", i), "", optional))
				}
				for _, item := range volume.ConfigMap.Items {
					add(podRef("ConfigMap", volume.ConfigMap.Name, fmt.Sprintf("spec.volumes[%d].configMap.items", i), item.Key, optional))
				}
			}
			if volume.Secret != nil {
				optional := volume.Secret.Optional != nil && *volume.Secret.Optional
				if len(volume.Secret.Items) == 0 {
					add(podRef("Secret", volume.Secret.SecretName, fmt.Sprintf("spec.volumes[%d].secret", i), "", optional))
				}
				for _, item := range volume.Secret.Items {
					add(podRef("Secret", volume.Secret.SecretName, fmt.Sprintf("spec.volumes[%d].secret.items", i), item.Key, optional))
				}
			}
			if volume.Projected != nil {
				for j, source := range volume.Projected.Sources {
					if source.ConfigMap != nil {
						optional := source.ConfigMap.Optional != nil && *source.ConfigMap.Optional
						add(podRef("ConfigMap", source.ConfigMap.Name, fmt.Sprintf("spec.volumes[%d].projected.sources[%d]", i, j), "", optional))
					}
					if source.Secret != nil {
						optional := source.Secret.Optional != nil && *source.Secret.Optional
						add(podRef("Secret", source.Secret.Name, fmt.Sprintf("spec.volumes[%d].projected.sources[%d]", i, j), "", optional))
					}
				}
			}
		}

		containers := append([]corev1.Container{}, pod.Spec.InitContainers...)
		containers = append(containers, pod.Spec.Containers...)
		for i, container := range containers {
			base := fmt.Sprintf("spec.containers[%d]", i)
			for j, source := range container.EnvFrom {
				if source.ConfigMapRef != nil {
					optional := source.ConfigMapRef.Optional != nil && *source.ConfigMapRef.Optional
					add(podRef("ConfigMap", source.ConfigMapRef.Name, fmt.Sprintf("%s.envFrom[%d]", base, j), "", optional))
				}
				if source.SecretRef != nil {
					optional := source.SecretRef.Optional != nil && *source.SecretRef.Optional
					add(podRef("Secret", source.SecretRef.Name, fmt.Sprintf("%s.envFrom[%d]", base, j), "", optional))
				}
			}
			for j, env := range container.Env {
				if env.ValueFrom == nil {
					continue
				}
				if env.ValueFrom.ConfigMapKeyRef != nil {
					source := env.ValueFrom.ConfigMapKeyRef
					optional := source.Optional != nil && *source.Optional
					add(podRef("ConfigMap", source.Name, fmt.Sprintf("%s.env[%d]", base, j), source.Key, optional))
				}
				if env.ValueFrom.SecretKeyRef != nil {
					source := env.ValueFrom.SecretKeyRef
					optional := source.Optional != nil && *source.Optional
					add(podRef("Secret", source.Name, fmt.Sprintf("%s.env[%d]", base, j), source.Key, optional))
				}
			}
		}
		for i, container := range pod.Spec.EphemeralContainers {
			base := fmt.Sprintf("spec.ephemeralContainers[%d]", i)
			for j, source := range container.EnvFrom {
				if source.ConfigMapRef != nil {
					optional := source.ConfigMapRef.Optional != nil && *source.ConfigMapRef.Optional
					add(podRef("ConfigMap", source.ConfigMapRef.Name, fmt.Sprintf("%s.envFrom[%d]", base, j), "", optional))
				}
				if source.SecretRef != nil {
					optional := source.SecretRef.Optional != nil && *source.SecretRef.Optional
					add(podRef("Secret", source.SecretRef.Name, fmt.Sprintf("%s.envFrom[%d]", base, j), "", optional))
				}
			}
			for j, env := range container.Env {
				if env.ValueFrom == nil {
					continue
				}
				if env.ValueFrom.ConfigMapKeyRef != nil {
					source := env.ValueFrom.ConfigMapKeyRef
					optional := source.Optional != nil && *source.Optional
					add(podRef("ConfigMap", source.Name, fmt.Sprintf("%s.env[%d]", base, j), source.Key, optional))
				}
				if env.ValueFrom.SecretKeyRef != nil {
					source := env.ValueFrom.SecretKeyRef
					optional := source.Optional != nil && *source.Optional
					add(podRef("Secret", source.Name, fmt.Sprintf("%s.env[%d]", base, j), source.Key, optional))
				}
			}
		}
	}
	return references
}

func lookupReference(
	ctx context.Context,
	client *kubernetes.Client,
	ref models.ResourceRef,
) referenceLookup {
	switch ref.Kind {
	case "ConfigMap":
		obj, err := client.Clientset.CoreV1().ConfigMaps(ref.Namespace).Get(ctx, ref.Name, metav1.GetOptions{})
		if err != nil {
			return lookupError(err)
		}
		keys := make(map[string]bool, len(obj.Data)+len(obj.BinaryData))
		safe := &corev1.ConfigMap{ObjectMeta: obj.ObjectMeta}
		safe.Data = make(map[string]string, len(obj.Data))
		for key := range obj.Data {
			keys[key], safe.Data[key] = true, "[REDACTED]"
		}
		safe.BinaryData = make(map[string][]byte, len(obj.BinaryData))
		for key := range obj.BinaryData {
			keys[key], safe.BinaryData[key] = true, nil
		}
		resource, err := toCollectedResource("ConfigMap", safe, safe.ObjectMeta)
		return lookupSuccess(resource, keys, err)
	case "Secret":
		obj, err := client.Clientset.CoreV1().Secrets(ref.Namespace).Get(ctx, ref.Name, metav1.GetOptions{})
		if err != nil {
			return lookupError(err)
		}
		keys := make(map[string]bool, len(obj.Data))
		safe := &corev1.Secret{ObjectMeta: obj.ObjectMeta, Type: obj.Type, Data: map[string][]byte{}}
		for key := range obj.Data {
			keys[key], safe.Data[key] = true, nil
		}
		resource, err := toCollectedResource("Secret", safe, safe.ObjectMeta)
		return lookupSuccess(resource, keys, err)
	case "ServiceAccount":
		obj, err := client.Clientset.CoreV1().ServiceAccounts(ref.Namespace).Get(ctx, ref.Name, metav1.GetOptions{})
		if err != nil {
			return lookupError(err)
		}
		safe := &corev1.ServiceAccount{
			ObjectMeta:       obj.ObjectMeta,
			ImagePullSecrets: append([]corev1.LocalObjectReference(nil), obj.ImagePullSecrets...),
		}
		resource, err := toCollectedResource("ServiceAccount", safe, safe.ObjectMeta)
		return lookupSuccess(resource, nil, err)
	case "StorageClass":
		obj, err := client.Clientset.StorageV1().StorageClasses().Get(ctx, ref.Name, metav1.GetOptions{})
		if err != nil {
			return lookupError(err)
		}
		safe := &storagev1.StorageClass{
			ObjectMeta:           obj.ObjectMeta,
			Provisioner:          obj.Provisioner,
			Parameters:           map[string]string{},
			ReclaimPolicy:        obj.ReclaimPolicy,
			MountOptions:         append([]string(nil), obj.MountOptions...),
			AllowVolumeExpansion: obj.AllowVolumeExpansion,
			VolumeBindingMode:    obj.VolumeBindingMode,
			AllowedTopologies:    obj.AllowedTopologies,
		}
		resource, err := toCollectedResource("StorageClass", safe, safe.ObjectMeta)
		return lookupSuccess(resource, nil, err)
	default:
		return referenceLookup{err: fmt.Errorf("unsupported reference kind %q", ref.Kind)}
	}
}

func lookupError(err error) referenceLookup {
	if apierrors.IsNotFound(err) {
		return referenceLookup{observed: true, exists: false}
	}
	return referenceLookup{err: err}
}

func lookupSuccess(
	resource models.CollectedResource,
	keys map[string]bool,
	err error,
) referenceLookup {
	if err != nil {
		return referenceLookup{err: err}
	}
	return referenceLookup{
		observed: true,
		exists:   true,
		keys:     keys,
		resource: &resource,
	}
}
