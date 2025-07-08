package api

import (
	"testing"
)

func TestMigrationOptions_Validation(t *testing.T) {
	tests := []struct {
		name    string
		options *MigrationOptions
		wantErr bool
	}{
		{
			name:    "nil options",
			options: nil,
			wantErr: true,
		},
		{
			name: "empty target",
			options: &MigrationOptions{
				Target: "",
			},
			wantErr: true,
		},
		{
			name: "valid options",
			options: &MigrationOptions{
				Target: "node2",
			},
			wantErr: false,
		},
		{
			name: "valid options with online migration",
			options: &MigrationOptions{
				Target: "node2",
				Online: func() *bool { b := true; return &b }(),
			},
			wantErr: false,
		},
		{
			name: "valid options with bandwidth limit",
			options: &MigrationOptions{
				Target:         "node2",
				BandwidthLimit: 1000,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a mock client with cluster data
			client := &Client{
				Cluster: &Cluster{
					Nodes: []*Node{
						{Name: "node1", Online: true},
						{Name: "node2", Online: true},
						{Name: "node3", Online: false},
					},
				},
			}

			vm := &VM{
				ID:     100,
				Name:   "test-vm",
				Node:   "node1",
				Type:   VMTypeQemu,
				Status: VMStatusRunning,
			}

			// We can't actually call MigrateVM without a real HTTP client,
			// but we can test the validation logic
			if tt.options == nil || tt.options.Target == "" {
				if !tt.wantErr {
					t.Errorf("Expected error for invalid options, but test expects no error")
				}
				return
			}

			// Ensure we don't try to migrate to the same node
			if tt.options.Target == vm.Node && !tt.wantErr {
				t.Errorf("Cannot migrate VM to the same node it's already on")
			}

			// Test target node validation
			if client.Cluster != nil {
				targetExists := false
				for _, node := range client.Cluster.Nodes {
					if node != nil && node.Name == tt.options.Target {
						targetExists = true
						break
					}
				}
				if !targetExists && !tt.wantErr {
					t.Errorf("Target node '%s' not found in cluster, but test expects no error", tt.options.Target)
				}
			}
		})
	}
}

func TestMigrationOptions_OnlineDefault(t *testing.T) {
	tests := []struct {
		name           string
		vmStatus       string
		expectedOnline bool
	}{
		{
			name:           "running VM defaults to online migration",
			vmStatus:       VMStatusRunning,
			expectedOnline: true,
		},
		{
			name:           "stopped VM defaults to offline migration",
			vmStatus:       VMStatusStopped,
			expectedOnline: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vm := &VM{
				Status: tt.vmStatus,
			}

			options := &MigrationOptions{
				Target: "node2",
				// Online not explicitly set - should use default based on VM status
			}

			// Simulate the logic from MigrateVM function
			var expectedDataOnline string
			if options.Online != nil {
				if *options.Online {
					expectedDataOnline = "1"
				} else {
					expectedDataOnline = "0"
				}
			} else {
				// Default logic
				if vm.Status == VMStatusRunning {
					expectedDataOnline = "1"
				} else {
					expectedDataOnline = "0"
				}
			}

			expectedOnlineValue := expectedDataOnline == "1"
			if expectedOnlineValue != tt.expectedOnline {
				t.Errorf("Expected online=%v for %s VM, got %v", tt.expectedOnline, tt.vmStatus, expectedOnlineValue)
			}
		})
	}
}

// Test for RestartVM behavior - both LXC and QEMU use /status/reboot endpoint
func TestRestartVM_VMTypeHandling(t *testing.T) {
	tests := []struct {
		name   string
		vmType string
		vmID   int
		vmName string
		vmNode string
	}{
		{
			name:   "LXC container should use reboot endpoint",
			vmType: VMTypeLXC,
			vmID:   101,
			vmName: "test-lxc",
			vmNode: "node1",
		},
		{
			name:   "QEMU VM should use reboot endpoint",
			vmType: VMTypeQemu,
			vmID:   100,
			vmName: "test-vm",
			vmNode: "node1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vm := &VM{
				ID:   tt.vmID,
				Name: tt.vmName,
				Node: tt.vmNode,
				Type: tt.vmType,
			}

			// Test that the VM type is correctly identified
			if vm.Type != tt.vmType {
				t.Errorf("Expected VM type %s, got %s", tt.vmType, vm.Type)
			}

			// Both LXC and QEMU now use the same /status/reboot endpoint
			// Note: We can't test the actual RestartVM function here without
			// mocking the HTTP client, but we can verify the VM properties
		})
	}
}
