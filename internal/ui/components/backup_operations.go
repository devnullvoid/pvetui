package components

import (
	"fmt"

	"github.com/devnullvoid/pvetui/pkg/api"
)

// BackupOperations handles backup CRUD operations.
type BackupOperations struct {
	app *App
	vm  *api.VM
}

// NewBackupOperations creates a new backup operations handler.
func NewBackupOperations(app *App, vm *api.VM) *BackupOperations {
	return &BackupOperations{
		app: app,
		vm:  vm,
	}
}

// GetBackups retrieves all backups for the VM.
func (bo *BackupOperations) GetBackups() ([]api.Backup, error) {
	client, err := bo.app.getClientForVM(bo.vm)
	if err != nil {
		return nil, err
	}
	return client.GetBackups(bo.vm)
}

// CreateBackup creates a new backup with the given options.
func (bo *BackupOperations) CreateBackup(options api.BackupOptions) (string, error) {
	if options.Storage == "" {
		return "", fmt.Errorf("storage is required")
	}

	client, err := bo.app.getClientForVM(bo.vm)
	if err != nil {
		return "", err
	}
	return client.CreateBackup(bo.vm, options)
}

// DeleteBackup deletes the specified backup.
func (bo *BackupOperations) DeleteBackup(volID string) error {
	client, err := bo.app.getClientForVM(bo.vm)
	if err != nil {
		return err
	}
	return client.DeleteBackup(bo.vm, volID)
}

// RestoreBackup restores the specified backup.
func (bo *BackupOperations) RestoreBackup(volID string) (string, error) {
	client, err := bo.app.getClientForVM(bo.vm)
	if err != nil {
		return "", err
	}
	return client.RestoreBackup(bo.vm, volID)
}
