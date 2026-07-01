package worker

import (
	"fmt"
	"strings"
)

// EventType represents the type of a CRD resource event.
type EventType string

const (
	EventTypeAdd    EventType = "Add"
	EventTypeUpdate EventType = "Update"
	EventTypeDelete EventType = "Delete"
)

// CRDWorkerEvent carries the information needed to reconcile a single CRD event.
type CRDWorkerEvent struct {
	Type          EventType
	ContainerName string
	Namespace     string
}

// key encodes the event as a workqueue key string.
// Format: <EventType>/<Namespace>/<ContainerName>
// Both Namespace and ContainerName follow Kubernetes naming rules and cannot contain '/'.
func (e *CRDWorkerEvent) key() string {
	return fmt.Sprintf("%s/%s/%s", e.Type, e.Namespace, e.ContainerName)
}

func parseEventKey(key string) (*CRDWorkerEvent, error) {
	parts := strings.SplitN(key, "/", 3)
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid event key %q: expected <EventType>/<Namespace>/<ContainerName>", key)
	}
	return &CRDWorkerEvent{
		Type:          EventType(parts[0]),
		Namespace:     parts[1],
		ContainerName: parts[2],
	}, nil
}
