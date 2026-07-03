// Package models defines shared domain types for ktrace.
package models

import (
	"encoding/json"
	"time"
)

// ResourceRef identifies a Kubernetes resource.
type ResourceRef struct {
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Namespace string `json:"namespace,omitempty"`
	UID       string `json:"uid,omitempty"`
}

// String returns a human-readable resource identifier.
func (r ResourceRef) String() string {
	if r.Namespace != "" {
		return r.Kind + "/" + r.Namespace + "/" + r.Name
	}
	return r.Kind + "/" + r.Name
}

// CollectedResource holds a snapshot of a collected Kubernetes object.
type CollectedResource struct {
	Ref      ResourceRef     `json:"ref"`
	Raw      json.RawMessage `json:"raw,omitempty"`
	Metadata ResourceMeta    `json:"metadata"`
}

// ResourceMeta holds essential metadata extracted from a resource.
type ResourceMeta struct {
	UID               string            `json:"uid,omitempty"`
	ResourceVersion   string            `json:"resourceVersion,omitempty"`
	CreationTimestamp time.Time         `json:"creationTimestamp,omitempty"`
	Labels            map[string]string `json:"labels,omitempty"`
	OwnerReferences   []OwnerRef        `json:"ownerReferences,omitempty"`
}

// OwnerRef is a simplified owner reference.
type OwnerRef struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
	UID  string `json:"uid"`
}

// TimelineEvent represents a chronological event from the cluster.
type TimelineEvent struct {
	Timestamp time.Time   `json:"timestamp"`
	Source    ResourceRef `json:"source"`
	Type      string      `json:"type"`
	Reason    string      `json:"reason"`
	Message   string      `json:"message"`
	Count     int32       `json:"count,omitempty"`
}

// ResourceGraph is the result of collecting related Kubernetes resources.
type ResourceGraph struct {
	Root      ResourceRef                  `json:"root"`
	Resources map[string][]CollectedResource `json:"resources"`
	Events    []TimelineEvent              `json:"events"`
}

// NewResourceGraph creates an empty graph for the given root reference.
func NewResourceGraph(root ResourceRef) *ResourceGraph {
	return &ResourceGraph{
		Root:      root,
		Resources: make(map[string][]CollectedResource),
		Events:    []TimelineEvent{},
	}
}

// AddResource appends a collected resource to the graph.
func (g *ResourceGraph) AddResource(r CollectedResource) {
	kind := r.Ref.Kind
	g.Resources[kind] = append(g.Resources[kind], r)
}

// Count returns the number of resources of the given kind.
func (g *ResourceGraph) Count(kind string) int {
	return len(g.Resources[kind])
}

// SortEvents sorts timeline events by timestamp ascending.
func (g *ResourceGraph) SortEvents() {
	if len(g.Events) < 2 {
		return
	}
	// insertion sort is fine for typical event counts
	for i := 1; i < len(g.Events); i++ {
		j := i
		for j > 0 && g.Events[j].Timestamp.Before(g.Events[j-1].Timestamp) {
			g.Events[j], g.Events[j-1] = g.Events[j-1], g.Events[j]
			j--
		}
	}
}
