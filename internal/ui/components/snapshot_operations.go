package components

import (
	"fmt"

	"github.com/devnullvoid/pvetui/pkg/api"
)

// SnapshotOperations handles snapshot CRUD operations.
type SnapshotOperations struct {
	app *App
	vm  *api.VM
}

// NewSnapshotOperations creates a new snapshot operations handler.
func NewSnapshotOperations(app *App, vm *api.VM) *SnapshotOperations {
	return &SnapshotOperations{
		app: app,
		vm:  vm,
	}
}

// CreateSnapshot creates a new snapshot with the given options.
func (so *SnapshotOperations) CreateSnapshot(name string, description string, vmState bool) error {
	if name == "" {
		return fmt.Errorf("snapshot name is required")
	}

	options := &api.SnapshotOptions{
		Description: description,
		VMState:     vmState,
	}

	return so.app.client.CreateSnapshot(so.vm, name, options)
}

// DeleteSnapshot deletes the specified snapshot.
func (so *SnapshotOperations) DeleteSnapshot(snapshotName string) error {
	// Prevent deleting "current" (NOW) as it's not a real snapshot
	if snapshotName == CurrentSnapshotName {
		return fmt.Errorf("cannot delete current state (NOW)")
	}

	return so.app.client.DeleteSnapshot(so.vm, snapshotName)
}

// RollbackToSnapshot rolls back to the specified snapshot.
func (so *SnapshotOperations) RollbackToSnapshot(snapshotName string) error {
	// Prevent rolling back to "current" (NOW) as it's not a real snapshot
	if snapshotName == CurrentSnapshotName {
		return fmt.Errorf("cannot rollback to current state (NOW)")
	}

	return so.app.client.RollbackToSnapshot(so.vm, snapshotName)
}

// GetSnapshots retrieves all snapshots for the VM.
func (so *SnapshotOperations) GetSnapshots() ([]api.Snapshot, error) {
	return so.app.client.GetSnapshots(so.vm)
}
