package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/devnullvoid/pvetui/pkg/api/testutils"
)

const (
	testAPIClusterResourcesPath = "/api2/json/cluster/resources"
	testAPIClusterStatusPath    = "/api2/json/cluster/status"
	testAPINodesPath            = "/api2/json/nodes"
)

func newSingleNodeClusterServer(ticket, csrfToken, nodeName string, cpu float64) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == testAPITicketPath {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"data": map[string]interface{}{
					"ticket":              ticket,
					"CSRFPreventionToken": csrfToken,
				},
			})
			return
		}
		if r.URL.Path == testAPIClusterResourcesPath {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"data": []interface{}{
					map[string]interface{}{
						"type":    "node",
						"node":    nodeName,
						"cpu":     cpu,
						"maxcpu":  8,
						"mem":     1024.0,
						"maxmem":  2048.0,
						"disk":    1024.0,
						"maxdisk": 2048.0,
						"uptime":  100,
					},
				},
			})
			return
		}
		if r.URL.Path == testAPIClusterStatusPath {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"data": []interface{}{
					map[string]interface{}{
						"id":     "node/" + nodeName,
						"name":   nodeName,
						"type":   "node",
						"online": 1,
					},
				},
			})
			return
		}
		if r.URL.Path == testAPINodesPath {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"data": []interface{}{
					map[string]interface{}{
						"node":   nodeName,
						"status": "online",
					},
				},
			})
			return
		}
	}))
}

func TestGroupClientManager_GetGroupNodes_WithOfflineProfile(t *testing.T) {
	// 1. Setup mock servers

	// Server 1: Online
	server1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == testAPITicketPath {
			// Auth
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"data": map[string]interface{}{
					"ticket":              "ticket1",
					"CSRFPreventionToken": "csrf1",
				},
			})
			return
		}
		if r.URL.Path == testAPIClusterResourcesPath {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"data": []interface{}{
					map[string]interface{}{
						"type":    "node",
						"node":    "node1",
						"cpu":     0.5,
						"maxcpu":  8,
						"mem":     8589934592,
						"maxmem":  17179869184,
						"disk":    10737418240,
						"maxdisk": 107374182400,
						"uptime":  123456,
					},
				},
			})
			return
		}
		if r.URL.Path == testAPIClusterStatusPath {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"data": []interface{}{
					map[string]interface{}{
						"id":     "node/node1",
						"name":   "node1",
						"type":   "node",
						"online": 1,
					},
				},
			})
			return
		}
		if r.URL.Path == testAPINodesPath {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"data": []interface{}{
					map[string]interface{}{
						"node":   "node1",
						"status": "online",
					},
				},
			})
			return
		}
	}))
	defer server1.Close()

	// Server 2: Offline (returns error)
	server2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server2.Close()

	// 2. Setup GroupClientManager
	logger := testutils.NewTestLogger()

	profiles := []ProfileEntry{
		{
			Name: "profile1",
			Config: &MockConfig{
				Addr:     server1.URL,
				User:     "user",
				Password: "password",
			},
		},
		{
			Name: "profile2",
			Config: &MockConfig{
				Addr:     server2.URL,
				User:     "user",
				Password: "password",
			},
		},
	}

	manager := NewGroupClientManager("testgroup", logger, &MockCache{})

	// 3. Initialize
	// Initialize checks connectivity. For server2 it will fail.
	// But Initialize returns error only if ALL fail.
	err := manager.Initialize(context.Background(), profiles)
	require.NoError(t, err)

	// Verify server2 is disconnected
	client2, exists := manager.GetClient("profile2")
	require.True(t, exists)
	status, _ := client2.GetStatus()
	require.NotEqual(t, ProfileStatusConnected, status)

	// 4. GetGroupNodes
	nodes, err := manager.GetGroupNodes(context.Background())
	require.NoError(t, err)

	// 5. Verify results
	// Should have node1 from profile1, and placeholder from profile2.
	assert.Len(t, nodes, 2)

	// Check node1
	var foundNode1, foundPlaceholder bool
	for _, node := range nodes {
		if node.SourceProfile == "profile1" {
			foundNode1 = true
			assert.Equal(t, "node1", node.Name)
			// Online status check might depend on how ListNodes parses it.
			// In mock response "status": "online" -> Online: true.
			assert.True(t, node.Online)
		} else if node.SourceProfile == "profile2" {
			foundPlaceholder = true
			assert.Equal(t, "profile2", node.Name)
			assert.False(t, node.Online)
			// The version field is used for error message
			assert.Contains(t, node.Version, "Connection Failed")
		}
	}
	assert.True(t, foundNode1, "Should find node from online profile")
	assert.True(t, foundPlaceholder, "Should find placeholder from offline profile")
}

// MockCache implementation
type MockCache struct{}

func (m *MockCache) Get(key string, dest interface{}) (bool, error)             { return false, nil }
func (m *MockCache) Set(key string, value interface{}, ttl time.Duration) error { return nil }
func (m *MockCache) Delete(key string) error                                    { return nil }
func (m *MockCache) Clear() error                                               { return nil }

func TestDeduplicateGroupNodes(t *testing.T) {
	nodes := []*Node{
		{Name: "pve1", IP: "10.0.0.1", SourceProfile: "a", Online: true},
		{Name: "pve1", IP: "10.0.0.1", SourceProfile: "b", Online: true},
		{Name: "pve2", IP: "10.0.0.2", SourceProfile: "a", Online: true},
		{ID: "offline-x", Name: "x", SourceProfile: "x", Online: false},
		{ID: "offline-y", Name: "y", SourceProfile: "y", Online: false},
	}

	got := deduplicateGroupNodes(nodes)
	require.Len(t, got, 4)
}

func TestDeduplicateGroupVMs(t *testing.T) {
	vms := []*VM{
		{ID: 100, Type: VMTypeQemu, Node: "pve1", Name: "vm100", SourceProfile: "a"},
		{ID: 100, Type: VMTypeQemu, Node: "pve1", Name: "vm100", SourceProfile: "b"},
		{ID: 101, Type: VMTypeQemu, Node: "pve1", Name: "vm101", SourceProfile: "a"},
	}

	got := deduplicateGroupVMs(vms)
	require.Len(t, got, 2)
}

func TestGroupClientManager_GetGroupNodes_DedupedProfilesDoNotCreateFakePlaceholders(t *testing.T) {
	server1 := newSingleNodeClusterServer("ticket1", "csrf1", "pve-main", 0.1)
	defer server1.Close()

	server2 := newSingleNodeClusterServer("ticket2", "csrf2", "pve-main", 0.2)
	defer server2.Close()

	logger := testutils.NewTestLogger()
	manager := NewGroupClientManager("all", logger, &MockCache{})
	profiles := []ProfileEntry{
		{
			Name: "default",
			Config: &MockConfig{
				Addr:     server1.URL,
				User:     "user",
				Password: "password",
			},
		},
		{
			Name: "backup",
			Config: &MockConfig{
				Addr:     server2.URL,
				User:     "user",
				Password: "password",
			},
		},
	}

	err := manager.Initialize(context.Background(), profiles)
	require.NoError(t, err)

	nodes, err := manager.GetGroupNodes(context.Background())
	require.NoError(t, err)

	// One real deduplicated node; no fake "default"/"backup" placeholder nodes.
	require.Len(t, nodes, 1)
	assert.Equal(t, "pve-main", nodes[0].Name)
	assert.NotEqual(t, "default", nodes[0].Name)
	assert.NotEqual(t, "backup", nodes[0].Name)
}
