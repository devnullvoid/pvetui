package api

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVMConfig_ParseAndBuild(t *testing.T) {
	tests := []struct {
		name     string
		vmType   string
		input    map[string]interface{}
		expected *VMConfig
	}{
		{
			name:   "QEMU VM with name",
			vmType: VMTypeQemu,
			input: map[string]interface{}{
				"name":        "test-vm",
				"cores":       4.0,
				"sockets":     2.0,
				"memory":      "8192",
				"description": "Test VM",
				"onboot":      1.0,
				"cpu":         "host",
				"maxmem":      16384.0,
				"boot":        "order=scsi0;net0",
				"tags":        "prod;db",
			},
			expected: &VMConfig{
				Name:         "test-vm",
				Cores:        4,
				Sockets:      2,
				Memory:       8 * 1024 * 1024 * 1024, // 8GB in bytes
				Description:  "Test VM",
				OnBoot:       &[]bool{true}[0],
				CPUType:      "host",
				MaxMem:       16384,
				BootOrder:    "order=scsi0;net0",
				Tags:         "prod;db",
				TagsExplicit: true,
			},
		},
		{
			name:   "LXC container with hostname",
			vmType: VMTypeLXC,
			input: map[string]interface{}{
				"hostname":    "test-container",
				"cores":       2.0,
				"memory":      4096.0,
				"description": "Test Container",
				"onboot":      0.0,
				"swap":        1024.0,
			},
			expected: &VMConfig{
				Hostname:    "test-container",
				Cores:       2,
				Memory:      4 * 1024 * 1024 * 1024, // 4GB in bytes
				Description: "Test Container",
				OnBoot:      &[]bool{false}[0],
				Swap:        1 * 1024 * 1024 * 1024, // 1GB in bytes
			},
		},
		{
			name:   "QEMU VM without name",
			vmType: VMTypeQemu,
			input: map[string]interface{}{
				"cores":  2.0,
				"memory": "4096",
				"onboot": "yes",
			},
			expected: &VMConfig{
				Cores:  2,
				Memory: 4 * 1024 * 1024 * 1024, // 4GB in bytes
				OnBoot: &[]bool{true}[0],
			},
		},
		{
			name:   "LXC container without hostname",
			vmType: VMTypeLXC,
			input: map[string]interface{}{
				"cores":  1.0,
				"memory": 2048.0,
				"onboot": "1",
			},
			expected: &VMConfig{
				Cores:  1,
				Memory: 2 * 1024 * 1024 * 1024, // 2GB in bytes
				OnBoot: &[]bool{true}[0],
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test parsing
			result := parseVMConfig(tt.vmType, tt.input)
			assert.Equal(t, tt.expected.Name, result.Name)
			assert.Equal(t, tt.expected.Hostname, result.Hostname)
			assert.Equal(t, tt.expected.Cores, result.Cores)
			assert.Equal(t, tt.expected.Sockets, result.Sockets)
			assert.Equal(t, tt.expected.Memory, result.Memory)
			assert.Equal(t, tt.expected.Description, result.Description)
			assert.Equal(t, tt.expected.OnBoot, result.OnBoot)
			assert.Equal(t, tt.expected.Tags, result.Tags)
			assert.Equal(t, tt.expected.TagsExplicit, result.TagsExplicit)

			if tt.vmType == VMTypeQemu {
				assert.Equal(t, tt.expected.CPUType, result.CPUType)
				assert.Equal(t, tt.expected.MaxMem, result.MaxMem)
				assert.Equal(t, tt.expected.BootOrder, result.BootOrder)
			}

			if tt.vmType == VMTypeLXC {
				assert.Equal(t, tt.expected.Swap, result.Swap)
			}

			// Test building payload
			payload := buildConfigPayload(tt.vmType, result)

			if tt.expected.Name != "" {
				assert.Equal(t, tt.expected.Name, payload["name"])
			}
			if tt.expected.Hostname != "" {
				assert.Equal(t, tt.expected.Hostname, payload["hostname"])
			}
			if tt.expected.Cores > 0 {
				assert.Equal(t, tt.expected.Cores, payload["cores"])
			}
			if tt.expected.Sockets > 0 {
				assert.Equal(t, tt.expected.Sockets, payload["sockets"])
			}
			if tt.expected.Memory > 0 {
				assert.Equal(t, tt.expected.Memory/1024/1024, payload["memory"])
			}
			if tt.expected.Description != "" {
				assert.Equal(t, tt.expected.Description, payload["description"])
			}
			if tt.expected.OnBoot != nil {
				if *tt.expected.OnBoot {
					assert.Equal(t, 1, payload["onboot"])
				} else {
					assert.Equal(t, 0, payload["onboot"])
				}
			}
			if tt.expected.TagsExplicit {
				assert.Equal(t, tt.expected.Tags, payload["tags"])
			}

			if tt.vmType == VMTypeQemu {
				if tt.expected.CPUType != "" {
					assert.Equal(t, tt.expected.CPUType, payload["cpu"])
				}
				if tt.expected.MaxMem > 0 {
					assert.Equal(t, tt.expected.MaxMem, payload["maxmem"])
				}
				if tt.expected.BootOrder != "" {
					assert.Equal(t, tt.expected.BootOrder, payload["boot"])
				}
			}

			if tt.vmType == VMTypeLXC {
				if tt.expected.Swap > 0 {
					assert.Equal(t, tt.expected.Swap/1024/1024, payload["swap"])
				}
			}
		})
	}
}
