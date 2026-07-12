package correlator

import (
	"encoding/json"
	"sort"

	corev1 "k8s.io/api/core/v1"

	"github.com/Stacked-Nerds/ktrace/pkg/models"
)

// Correlate builds explicit edges between collected resources.
func Correlate(graph *models.ResourceGraph) []models.Edge {
	if graph == nil {
		return nil
	}

	edges := make([]models.Edge, 0)
	seen := make(map[string]bool)

	add := func(from, to models.ResourceRef, relation string) {
		key := from.String() + "->" + to.String() + ":" + relation
		if seen[key] {
			return
		}
		seen[key] = true
		edges = append(edges, models.Edge{From: from, To: to, Relation: relation})
	}

	byUID := indexByUID(graph)
	pvcByName := indexByKindName(graph, "PersistentVolumeClaim")
	pvByName := indexByKindName(graph, "PersistentVolume")
	nodeByName := indexByKindName(graph, "Node")
	configMapByName := indexByKindName(graph, "ConfigMap")
	secretByName := indexByKindName(graph, "Secret")
	serviceAccountByName := indexByKindName(graph, "ServiceAccount")
	storageClassByName := indexByKindName(graph, "StorageClass")

	for _, resources := range graph.Resources {
		for _, r := range resources {
			for _, owner := range r.Metadata.OwnerReferences {
				parent, ok := byUID[owner.UID]
				if !ok {
					continue
				}
				add(parent, r.Ref, "owns")
			}
		}
	}
	for _, reference := range graph.References {
		relation := "references"
		switch reference.To.Kind {
		case "Secret":
			relation = "references-secret"
		case "ConfigMap":
			relation = "references-configmap"
		case "ServiceAccount":
			relation = "uses-service-account"
		case "StorageClass":
			relation = "provisioned-by"
		}
		add(reference.From, reference.To, relation)
	}

	for _, cr := range graph.Resources["Pod"] {
		pod, err := decodePod(cr.Raw)
		if err != nil {
			continue
		}
		for _, vol := range pod.Spec.Volumes {
			if vol.PersistentVolumeClaim == nil {
				continue
			}
			claim := vol.PersistentVolumeClaim.ClaimName
			if pvc, ok := pvcByName[pod.Namespace+"/"+claim]; ok {
				add(cr.Ref, pvc, "mounts")
			}
		}
		if pod.Spec.NodeName != "" {
			if node, ok := nodeByName[pod.Spec.NodeName]; ok {
				add(cr.Ref, node, "scheduled")
			}
		}
		linkPodReferences(
			cr.Ref,
			pod,
			configMapByName,
			secretByName,
			serviceAccountByName,
			add,
		)
	}

	for _, cr := range graph.Resources["PersistentVolumeClaim"] {
		pvc, err := decodePVC(cr.Raw)
		if err != nil {
			continue
		}
		if pvc.Spec.VolumeName != "" {
			if pv, ok := pvByName[pvc.Spec.VolumeName]; ok {
				add(cr.Ref, pv, "bound")
			}
		}
		if pvc.Spec.StorageClassName != nil {
			if storageClass, ok := storageClassByName[*pvc.Spec.StorageClassName]; ok {
				add(cr.Ref, storageClass, "provisioned-by")
			}
		}
	}

	linkServices(graph, add)
	sort.Slice(edges, func(i, j int) bool {
		left := edges[i].From.String() + "|" + edges[i].Relation + "|" + edges[i].To.String()
		right := edges[j].From.String() + "|" + edges[j].Relation + "|" + edges[j].To.String()
		return left < right
	})
	return edges
}

func linkPodReferences(
	podRef models.ResourceRef,
	pod *corev1.Pod,
	configMaps map[string]models.ResourceRef,
	secrets map[string]models.ResourceRef,
	serviceAccounts map[string]models.ResourceRef,
	add func(from, to models.ResourceRef, relation string),
) {
	key := func(name string) string { return pod.Namespace + "/" + name }

	if pod.Spec.ServiceAccountName != "" {
		if ref, ok := serviceAccounts[key(pod.Spec.ServiceAccountName)]; ok {
			add(podRef, ref, "uses-service-account")
		}
	}
	for _, pullSecret := range pod.Spec.ImagePullSecrets {
		if ref, ok := secrets[key(pullSecret.Name)]; ok {
			add(podRef, ref, "uses-image-pull-secret")
		}
	}
	for _, volume := range pod.Spec.Volumes {
		if volume.ConfigMap != nil {
			if ref, ok := configMaps[key(volume.ConfigMap.Name)]; ok {
				add(podRef, ref, "references-configmap")
			}
		}
		if volume.Secret != nil {
			if ref, ok := secrets[key(volume.Secret.SecretName)]; ok {
				add(podRef, ref, "references-secret")
			}
		}
	}

	containers := append([]corev1.Container{}, pod.Spec.InitContainers...)
	containers = append(containers, pod.Spec.Containers...)
	for _, container := range containers {
		for _, source := range container.EnvFrom {
			if source.ConfigMapRef != nil {
				if ref, ok := configMaps[key(source.ConfigMapRef.Name)]; ok {
					add(podRef, ref, "references-configmap")
				}
			}
			if source.SecretRef != nil {
				if ref, ok := secrets[key(source.SecretRef.Name)]; ok {
					add(podRef, ref, "references-secret")
				}
			}
		}
		for _, env := range container.Env {
			if env.ValueFrom == nil {
				continue
			}
			if env.ValueFrom.ConfigMapKeyRef != nil {
				if ref, ok := configMaps[key(env.ValueFrom.ConfigMapKeyRef.Name)]; ok {
					add(podRef, ref, "references-configmap")
				}
			}
			if env.ValueFrom.SecretKeyRef != nil {
				if ref, ok := secrets[key(env.ValueFrom.SecretKeyRef.Name)]; ok {
					add(podRef, ref, "references-secret")
				}
			}
		}
	}
}

func indexByUID(graph *models.ResourceGraph) map[string]models.ResourceRef {
	idx := make(map[string]models.ResourceRef)
	for _, resources := range graph.Resources {
		for _, r := range resources {
			if r.Metadata.UID != "" {
				idx[r.Metadata.UID] = r.Ref
			}
		}
	}
	return idx
}

func indexByKindName(graph *models.ResourceGraph, kind string) map[string]models.ResourceRef {
	idx := make(map[string]models.ResourceRef)
	for _, r := range graph.Resources[kind] {
		key := r.Ref.Name
		if r.Ref.Namespace != "" {
			key = r.Ref.Namespace + "/" + r.Ref.Name
		}
		idx[key] = r.Ref
	}
	return idx
}

func decodePod(raw []byte) (*corev1.Pod, error) {
	var pod corev1.Pod
	return &pod, json.Unmarshal(raw, &pod)
}

func decodePVC(raw []byte) (*corev1.PersistentVolumeClaim, error) {
	var pvc corev1.PersistentVolumeClaim
	return &pvc, json.Unmarshal(raw, &pvc)
}

func linkServices(graph *models.ResourceGraph, add func(from, to models.ResourceRef, relation string)) {
	for _, svcCR := range graph.Resources["Service"] {
		svc, err := decodeService(svcCR.Raw)
		if err != nil || len(svc.Spec.Selector) == 0 {
			continue
		}
		for _, podCR := range graph.Resources["Pod"] {
			pod, err := decodePod(podCR.Raw)
			if err != nil || pod.Namespace != svc.Namespace {
				continue
			}
			if selectorMatches(svc.Spec.Selector, pod.Labels) {
				add(svcCR.Ref, podCR.Ref, "selects")
			}
		}
	}
}

func decodeService(raw []byte) (*corev1.Service, error) {
	var svc corev1.Service
	return &svc, json.Unmarshal(raw, &svc)
}

func selectorMatches(selector, labels map[string]string) bool {
	for k, v := range selector {
		if labels[k] != v {
			return false
		}
	}
	return len(selector) > 0
}
