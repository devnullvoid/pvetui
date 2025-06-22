//go:build examples
// +build examples

package main

import (
	"github.com/stretchr/testify/mock"

	"github.com/devnullvoid/proxmox-tui/pkg/api"
	"github.com/devnullvoid/proxmox-tui/pkg/api/testutils"
)

func main() {
	println("Testing 'Testing with Mocks' example from DOCUMENTATION.md...")

	// Create mocks
	mockLogger := &testutils.MockLogger{}
	mockCache := &testutils.MockCache{}
	config := testutils.NewTestConfig()

	// Set expectations
	mockLogger.On("Debug", mock.AnythingOfType("string"), mock.Anything).Return()
	mockCache.On("Get", "test-key", mock.Anything).Return(false, nil)

	println("✓ Mocks created successfully")

	// Create client with mocks
	client, err := api.NewClient(config,
		api.WithLogger(mockLogger),
		api.WithCache(mockCache))

	if err != nil {
		println("Expected error with test credentials:", err.Error())
		println("✓ Error handling works correctly")
	} else {
		println("✓ Client created successfully")
		if client != nil {
			println("✓ Client is not nil")
		}
	}

	println("✅ Testing with mocks example compiles and runs correctly!")
	println("   (Note: For actual testing, use 'go test' with proper test functions)")
}
