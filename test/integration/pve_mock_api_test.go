package integration

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/devnullvoid/pvetui/internal/adapters"
	"github.com/devnullvoid/pvetui/pkg/api"
	"github.com/devnullvoid/pvetui/test/testutils"
	"github.com/stretchr/testify/require"
)

func TestPVEMockAPI(t *testing.T) {
	// Resolve repository root (absolute) so relative paths work in CI and locally
	cwd, _ := os.Getwd()
	repoRoot, err := filepath.Abs(filepath.Join(cwd, "../.."))
	require.NoError(t, err)

	buildPath := filepath.Join(repoRoot, "cmd/pve-mock-api")
	binPath := filepath.Join(repoRoot, "pve-mock-api-test-bin")
	specPath := filepath.Join(repoRoot, "docs/api/pve-openapi.yaml")

	// Ensure OpenAPI spec exists (CI doesn't track generated spec)
	if _, err := os.Stat(specPath); os.IsNotExist(err) {
		cmdGen := exec.Command("go", "run", "./cmd/pve-openapi-gen", "-out", specPath, "-version", "test")
		cmdGen.Dir = repoRoot
		output, genErr := cmdGen.CombinedOutput()
		require.NoError(t, genErr, "Failed to generate OpenAPI spec: %s", string(output))
	}

	// 1. Build pve-mock-api
	cmd := exec.Command("go", "build", "-o", binPath, buildPath)
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "Failed to build pve-mock-api: %s", string(output))
	defer os.Remove(binPath)

	// 2. Start mock api
	port := 8086
	serverCmd := exec.Command(binPath, "-spec", specPath, "-port", fmt.Sprintf("%d", port))
	// Redirect stdout/stderr for debug
	serverCmd.Stdout = os.Stdout
	serverCmd.Stderr = os.Stderr

	err = serverCmd.Start()
	require.NoError(t, err, "Failed to start pve-mock-api")
	defer func() {
		if serverCmd.Process != nil {
			serverCmd.Process.Kill()
		}
	}()

	// Wait for start
	deadline := time.Now().Add(5 * time.Second)
	started := false
	for time.Now().Before(deadline) {
		resp, err := http.Get(fmt.Sprintf("http://localhost:%d/access/ticket", port))
		if err == nil {
			resp.Body.Close()
			started = true
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	require.True(t, started, "Mock server did not start in time")

	// 3. Setup client
	mockURL := fmt.Sprintf("http://localhost:%d", port)
	itc := testutils.NewIntegrationTestConfig(t)
	itc.ProxmoxAddr = mockURL
	cfg := itc.CreateTestConfig()
	cfg.Addr = mockURL
	cfg.Insecure = true

	configAdapter := adapters.NewConfigAdapter(cfg)
	loggerAdapter := adapters.NewLoggerAdapter(cfg)
	_, _, testCache, _ := itc.SetupIntegrationTest(t)

	client, err := api.NewClient(configAdapter,
		api.WithLogger(loggerAdapter),
		api.WithCache(testCache))
	require.NoError(t, err)

	// 4. Run tests
	t.Run("list_vms", func(t *testing.T) {
		vms, err := client.GetVmList(context.Background())
		require.NoError(t, err)
		require.NotEmpty(t, vms)

		found := false
		for _, vm := range vms {
			if name, ok := vm["name"].(string); ok && name == "test-vm" {
				found = true
				break
			}
		}
		require.True(t, found, "Expected test-vm from stateful mock")
	})

	t.Run("node_status", func(t *testing.T) {
		node, err := client.GetNodeStatus("pve")
		require.NoError(t, err)
		require.Equal(t, "pve", node.Name)
		require.Equal(t, "8.1.3", node.Version)
		require.Equal(t, 2, node.CPUInfo.Sockets)
	})

	t.Run("vm_config_lifecycle", func(t *testing.T) {
		vm := &api.VM{
			Node: "pve",
			Type: "qemu",
			ID:   100,
		}

		config, err := client.GetVMConfig(vm)
		require.NoError(t, err)
		require.Equal(t, "test-vm", config.Name)

		// Update resources (4 cores, 8GB)
		err = client.UpdateVMResources(vm, 4, 8192*1024*1024)
		require.NoError(t, err)

		// Verify update
		configUpdated, err := client.GetVMConfig(vm)
		require.NoError(t, err)
		require.Equal(t, 4, configUpdated.Cores)
		require.Equal(t, int64(8192*1024*1024), configUpdated.Memory)
	})

	t.Run("vm_status_action", func(t *testing.T) {
		vm := &api.VM{
			Node:   "pve",
			Type:   "qemu",
			ID:     100,
			Name:   "test-vm",
			Status: "running", // Initial status
		}

		// Stop
		_, err := client.StopVM(vm)
		require.NoError(t, err)

		// Status transitions are async; wait for task completion to be reflected
		require.Eventually(t, func() bool {
			refreshedVM, refreshErr := client.RefreshVMData(vm, nil)
			if refreshErr != nil {
				return false
			}
			return refreshedVM.Status == "stopped"
		}, 8*time.Second, 200*time.Millisecond)

		// Start
		_, err = client.StartVM(vm)
		require.NoError(t, err)

		require.Eventually(t, func() bool {
			refreshedVM, refreshErr := client.RefreshVMData(vm, nil)
			if refreshErr != nil {
				return false
			}
			return refreshedVM.Status == "running"
		}, 8*time.Second, 200*time.Millisecond)
	})

	t.Run("cluster_nextid_and_guest_creation", func(t *testing.T) {
		var nextIDResp map[string]interface{}
		err := client.Get("/cluster/nextid", &nextIDResp)
		require.NoError(t, err)
		require.Equal(t, float64(102), nextIDResp["data"])

		resp, err := http.PostForm(
			fmt.Sprintf("%s/api2/json/nodes/pve/qemu", mockURL),
			url.Values{
				"vmid":    {"102"},
				"name":    {"created-vm"},
				"memory":  {"2048"},
				"cores":   {"2"},
				"sockets": {"1"},
				"scsi0":   {"local-zfs:20"},
				"cdrom":   {"local:iso/debian-12.5.iso"},
				"net0":    {"virtio=DE:AD:BE:EF:00:01,bridge=vmbr0"},
				"start":   {"1"},
			},
		)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)

		require.Eventually(t, func() bool {
			vm := &api.VM{Node: "pve", Type: "qemu", ID: 102}
			config, cfgErr := client.GetVMConfig(vm)
			if cfgErr != nil {
				return false
			}
			return config.Name == "created-vm"
		}, 3*time.Second, 100*time.Millisecond)

		createdVM, err := client.RefreshVMData(&api.VM{Node: "pve", Type: "qemu", ID: 102}, nil)
		require.NoError(t, err)
		require.Equal(t, "running", createdVM.Status)

		var storageContent map[string]interface{}
		err = client.Get("/nodes/pve/storage/local-zfs/content?content=images&vmid=102", &storageContent)
		require.NoError(t, err)

		items, ok := storageContent["data"].([]interface{})
		require.True(t, ok)
		require.Len(t, items, 1)
	})

	t.Run("lxc_creation_storage_content_and_resize", func(t *testing.T) {
		resp, err := http.PostForm(
			fmt.Sprintf("%s/api2/json/nodes/pve/lxc", mockURL),
			url.Values{
				"vmid":         {"103"},
				"hostname":     {"created-ct"},
				"memory":       {"1024"},
				"swap":         {"512"},
				"cores":        {"2"},
				"rootfs":       {"local-zfs:12"},
				"ostemplate":   {"local:vztmpl/debian-12-standard_12.7-1_amd64.tar.zst"},
				"net0":         {"name=eth0,bridge=vmbr0,ip=dhcp"},
				"unprivileged": {"1"},
			},
		)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)

		require.Eventually(t, func() bool {
			vm := &api.VM{Node: "pve", Type: "lxc", ID: 103}
			config, cfgErr := client.GetVMConfig(vm)
			if cfgErr != nil {
				return false
			}
			return config.Hostname == "created-ct"
		}, 3*time.Second, 100*time.Millisecond)

		ct := &api.VM{Node: "pve", Type: "lxc", ID: 103}
		err = client.ResizeVMStorage(ct, "rootfs", "+4G")
		require.NoError(t, err)

		require.Eventually(t, func() bool {
			config, cfgErr := client.GetVMConfig(ct)
			if cfgErr != nil {
				return false
			}
			return config.Disks["rootfs"] == int64(16*1024*1024*1024)
		}, 3*time.Second, 100*time.Millisecond)

		for _, contentType := range []string{"iso", "vztmpl", "snippets", "backup"} {
			items, listErr := client.GetStorageContent("pve", "local", contentType)
			err = listErr
			require.NoError(t, err)
			require.NotEmpty(t, items, "expected mock storage content for %s", contentType)
		}
	})

	t.Run("storage_content_delete_and_backup_restore", func(t *testing.T) {
		backups, err := client.GetStorageContent("pve", "local", "backup")
		require.NoError(t, err)
		require.NotEmpty(t, backups)

		backupVolID := backups[0].VolID

		upid, err := client.DeleteStorageContent("pve", "local", backupVolID)
		require.NoError(t, err)
		require.NotEmpty(t, upid)

		require.Eventually(t, func() bool {
			client.ClearAPICache()
			items, listErr := client.GetStorageContent("pve", "local", "backup")
			if listErr != nil {
				return false
			}
			for _, item := range items {
				if item.VolID == backupVolID {
					return false
				}
			}
			return true
		}, 3*time.Second, 100*time.Millisecond)

		restoreUPID, err := client.RestoreGuestFromBackup(
			"pve",
			api.VMTypeQemu,
			104,
			"local:backup/vzdump-qemu-100-2023_01_01-12_00_00.vma.zst",
			false,
		)
		require.NoError(t, err)
		require.NotEmpty(t, restoreUPID)

		require.Eventually(t, func() bool {
			client.ClearAPICache()
			vm := &api.VM{Node: "pve", Type: api.VMTypeQemu, ID: 104}
			_, refreshErr := client.RefreshVMData(vm, nil)
			return refreshErr == nil
		}, 3*time.Second, 100*time.Millisecond)
	})

	t.Run("storage_download_url_and_oci_pull", func(t *testing.T) {
		downloadUPID, err := client.DownloadStorageContentFromURL("pve", "local", api.StorageDownloadURLOptions{
			URL:                "https://example.invalid/ubuntu-24.04.iso",
			Content:            "iso",
			Filename:           "ubuntu-24.04.iso",
			VerifyCertificates: true,
		})
		require.NoError(t, err)
		require.NotEmpty(t, downloadUPID)

		require.Eventually(t, func() bool {
			client.ClearAPICache()
			items, listErr := client.GetStorageContent("pve", "local", "iso")
			if listErr != nil {
				return false
			}
			for _, item := range items {
				if item.VolID == "local:iso/ubuntu-24.04.iso" {
					return true
				}
			}
			return false
		}, 3*time.Second, 100*time.Millisecond)

		ociUPID, err := client.PullStorageOCIImage("pve", "local-zfs", api.StorageOCIPullOptions{
			Reference: "docker.io/library/alpine:latest",
			Filename:  "alpine-latest.oci",
		})
		require.Error(t, err)
		require.Empty(t, ociUPID)

		ociUPID, err = client.PullStorageOCIImage("pve", "local", api.StorageOCIPullOptions{
			Reference: "docker.io/library/alpine:latest",
			Filename:  "alpine-latest.oci",
		})
		require.NoError(t, err)
		require.NotEmpty(t, ociUPID)

		require.Eventually(t, func() bool {
			client.ClearAPICache()
			items, listErr := client.GetStorageContent("pve", "local", "import")
			if listErr != nil {
				return false
			}
			for _, item := range items {
				if item.VolID == "local:import/alpine-latest.oci" {
					return true
				}
			}
			return false
		}, 3*time.Second, 100*time.Millisecond)
	})
}
