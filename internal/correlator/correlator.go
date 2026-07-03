package correlator

import (
	"encoding/json"

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
	}

	linkServices(graph, add)
	return edges
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
