package components

import (
	"testing"

	"github.com/devnullvoid/pvetui/pkg/api"
	"github.com/stretchr/testify/require"
)

func TestInferBackupRestoreDefaults(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		volID    string
		wantType string
		wantVMID int
	}{
		{
			name:     "qemu backup",
			volID:    "local:backup/vzdump-qemu-123-2026_03_10-12_00_00.vma.zst",
			wantType: api.VMTypeQemu,
			wantVMID: 123,
		},
		{
			name:     "lxc backup",
			volID:    "local:backup/vzdump-lxc-456-2026_03_10-12_00_00.tar.zst",
			wantType: api.VMTypeLXC,
			wantVMID: 456,
		},
		{
			name:     "unknown backup format",
			volID:    "local:backup/custom-backup.tar.zst",
			wantType: "",
			wantVMID: 0,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			gotType, gotVMID := inferBackupRestoreDefaults(tt.volID)
			require.Equal(t, tt.wantType, gotType)
			require.Equal(t, tt.wantVMID, gotVMID)
		})
	}
}

func TestStorageTaskTargetID(t *testing.T) {
	t.Parallel()

	item := api.StorageContentItem{
		VolID: "local:iso/debian-12.5.iso",
	}

	first := storageTaskTargetID(item)
	second := storageTaskTargetID(item)

	require.Positive(t, first)
	require.Equal(t, first, second)
}
