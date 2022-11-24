package controllers

import (
	"sigs.k8s.io/controller-runtime/pkg/event"
)

type EventFilter struct {
}

// Create returns true if the Create event should be processed
func (f EventFilter) Create(e event.CreateEvent) bool {
	return true
}

// Delete returns true if the Delete event should be processed
func (f EventFilter) Delete(e event.DeleteEvent) bool {
	return true
}

// Update returns true if the Update event should be processed
func (f EventFilter) Update(e event.UpdateEvent) bool {
	return true
}

// Generic returns true if the Generic event should be processed
func (f EventFilter) Generic(e event.GenericEvent) bool {
	return true
}
