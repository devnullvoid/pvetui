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

func TestValidateDownloadURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		rawURL  string
		wantErr bool
	}{
		{name: "valid https", rawURL: "https://example.com/file.iso"},
		{name: "valid http", rawURL: "http://example.com/file.iso"},
		{name: "missing host", rawURL: "https:///file.iso", wantErr: true},
		{name: "unsupported scheme", rawURL: "ftp://example.com/file.iso", wantErr: true},
		{name: "not a url", rawURL: "nope", wantErr: true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validateDownloadURL(tt.rawURL)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestInferDownloadFilename(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		rawURL   string
		expected string
	}{
		{name: "simple path", rawURL: "https://example.com/debian.iso", expected: "debian.iso"},
		{name: "query string", rawURL: "https://example.com/images/alpine.iso?mirror=1", expected: "alpine.iso"},
		{name: "no path filename", rawURL: "https://example.com/", expected: ""},
		{name: "invalid url", rawURL: "://bad", expected: ""},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.expected, inferDownloadFilename(tt.rawURL))
		})
	}
}
