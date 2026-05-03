package components

import (
	"testing"

	"github.com/devnullvoid/pvetui/pkg/api"
	"github.com/stretchr/testify/require"
)

func TestFilterTemplates(t *testing.T) {
	t.Parallel()

	templates := []api.ApplianceTemplate{
		{Package: "debian-12-standard", Filename: "debian-12-standard_12.12-1_amd64.tar.zst", Section: "system", Description: "Debian 12 Bookworm"},
		{Package: "ubuntu-24.04-standard", Filename: "ubuntu-24.04-standard_24.04-2_amd64.tar.zst", Section: "system", Description: "Ubuntu 24.04 LTS"},
		{Package: "alpine-3.22-default", Filename: "alpine-3.22-default_20250617_amd64.tar.xz", Section: "system", Description: "Alpine 3.22"},
		{Package: "wordpress", Filename: "wordpress_6.7.0-1_amd64.tar.gz", Section: "turnkeylinux", Description: "Wordpress"},
		{Package: "mailserver", Filename: "mailserver_1.0_amd64.tar.gz", Section: "mail", Description: "Mail Server"},
	}

	tests := []struct {
		name     string
		section  string
		wantLen  int
		wantPkgs []string
	}{
		{
			name:    "all templates",
			section: "All",
			wantLen: 5,
		},
		{
			name:    "empty section treated as all",
			section: "",
			wantLen: 5,
		},
		{
			name:    "system section sorted alphabetically by filename",
			section: "system",
			wantLen: 3,
			wantPkgs: []string{
				"alpine-3.22-default_20250617_amd64.tar.xz",
				"debian-12-standard_12.12-1_amd64.tar.zst",
				"ubuntu-24.04-standard_24.04-2_amd64.tar.zst",
			},
		},
		{
			name:     "turnkeylinux section",
			section:  "turnkeylinux",
			wantLen:  1,
			wantPkgs: []string{"wordpress_6.7.0-1_amd64.tar.gz"},
		},
		{
			name:     "mail section",
			section:  "mail",
			wantLen:  1,
			wantPkgs: []string{"mailserver_1.0_amd64.tar.gz"},
		},
		{
			name:    "case insensitive match",
			section: "SYSTEM",
			wantLen: 3,
		},
		{
			name:    "unknown section returns empty",
			section: "unknown",
			wantLen: 0,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := filterTemplates(templates, tt.section)
			require.Len(t, got, tt.wantLen)
			if tt.wantPkgs != nil {
				gotFilenames := make([]string, len(got))
				for i, tmpl := range got {
					gotFilenames[i] = tmpl.Filename
				}
				require.Equal(t, tt.wantPkgs, gotFilenames)
			}
		})
	}
}

func TestTemplateDropdownLabel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		tmpl     api.ApplianceTemplate
		expected string
	}{
		{
			name:     "always uses filename regardless of description",
			tmpl:     api.ApplianceTemplate{Package: "debian-12-standard", Filename: "debian-12-standard_12.12-1_amd64.tar.zst", Description: "Debian 12 Bookworm"},
			expected: "debian-12-standard_12.12-1_amd64.tar.zst",
		},
		{
			name:     "without description",
			tmpl:     api.ApplianceTemplate{Package: "alpine-3.22-default", Filename: "alpine-3.22-default_20250617_amd64.tar.xz"},
			expected: "alpine-3.22-default_20250617_amd64.tar.xz",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.expected, templateDropdownLabel(tt.tmpl))
		})
	}
}

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
