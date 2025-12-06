package integration

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/devnullvoid/pvetui/internal/adapters"
	"github.com/devnullvoid/pvetui/pkg/api"
	"github.com/devnullvoid/pvetui/test/testutils"
	"github.com/stretchr/testify/require"
)

func TestPVEMockAPI(t *testing.T) {
	rootDir := "../../"

	// Check if we are in root (if running via some other way)
	if _, err := os.Stat("cmd/pve-mock-api"); err == nil {
		rootDir = "."
	}

	buildPath := fmt.Sprintf("%s/cmd/pve-mock-api", rootDir)
	binPath := fmt.Sprintf("%s/pve-mock-api-test-bin", rootDir)
    specPath := fmt.Sprintf("%s/docs/api/pve-openapi.yaml", rootDir)

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
            Node: "pve",
            Type: "qemu",
            ID:   100,
            Name: "test-vm",
            Status: "running", // Initial status
        }

        // Stop
        err := client.StopVM(vm)
        require.NoError(t, err)

        // Refresh to see update (GetVmStatus is cached)
        refreshedVM, err := client.RefreshVMData(vm, nil)
        require.NoError(t, err)
        require.Equal(t, "stopped", refreshedVM.Status)

        // Start
        err = client.StartVM(vm)
        require.NoError(t, err)

        refreshedVM, err = client.RefreshVMData(vm, nil)
        require.NoError(t, err)
        require.Equal(t, "running", refreshedVM.Status)
    })
}
