package taskmanager

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/devnullvoid/pvetui/pkg/api"
	"github.com/devnullvoid/pvetui/pkg/api/interfaces"
	"github.com/devnullvoid/pvetui/pkg/mockpve"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockLogger struct{}

func (m *mockLogger) Debug(format string, args ...interface{}) {}
func (m *mockLogger) Info(format string, args ...interface{})  {}
func (m *mockLogger) Warn(format string, args ...interface{})  {}
func (m *mockLogger) Error(format string, args ...interface{}) {}

func TestTaskManager(t *testing.T) {
	// Setup Mock Server
	state := mockpve.NewMockState()
	r := mux.NewRouter()

	// Register relevant handlers
	r.HandleFunc("/api2/json/nodes/{node}/tasks/{upid}/status", mockpve.HandleTaskStatus(state)).Methods("GET")
	r.HandleFunc("/api2/json/nodes/{node}/tasks/{upid}", mockpve.HandleStopTask(state)).Methods("DELETE")

	// Mock auth endpoint to avoid 401
	r.HandleFunc("/api2/json/access/ticket", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"data": {"ticket": "dummy", "CSRFPreventionToken": "dummy"}}`)
	})

	server := httptest.NewServer(r)
	defer server.Close()

	// Setup Client
	config := &mockConfig{addr: server.URL}
	opts := api.ClientOption(func(c *api.ClientOptions) {
		c.Logger = &mockLogger{}
		c.Cache = &interfaces.NoOpCache{}
	})

	client, err := api.NewClient(config, opts)
	require.NoError(t, err)

	// Setup TaskManager
	var updated atomic.Bool
	notify := func() {
		updated.Store(true)
	}
	resolver := func(nodeName string) (*api.Client, error) {
		return client, nil
	}
	tm := NewTaskManager(resolver, notify)
	defer tm.Stop()

	// Create a dummy task that is already "running" on the server side
	// We use the state helper to create a task in the mock state
	upid := state.CreateTask("pve", "qmstart", "100", "root@pam")

	// Enqueue a task
	task := &Task{
		TargetVMID: 100,
		TargetNode: "pve",
		Type:       "Start",
		Operation: func() (string, error) {
			return upid, nil
		},
	}

	tm.Enqueue(task)

	// Wait for task to be picked up
	assert.Eventually(t, func() bool {
		t := tm.GetActiveTaskForVM("pve", 100)
		return t != nil && t.Status == StatusRunning
	}, 1*time.Second, 100*time.Millisecond)

	// Wait for task to complete (MockState auto-completes in 5s)
	// We can manually complete it to speed up test
	state.CompleteTask(upid, "OK")

	assert.Eventually(t, func() bool {
		// Task should be removed from active and queue
		return tm.GetActiveTaskForVM("pve", 100) == nil
	}, 5*time.Second, 100*time.Millisecond)

	assert.True(t, updated.Load())
}

func TestTaskManager_Cancel(t *testing.T) {
	// Setup Mock Server
	state := mockpve.NewMockState()
	r := mux.NewRouter()
	r.HandleFunc("/api2/json/nodes/{node}/tasks/{upid}/status", mockpve.HandleTaskStatus(state)).Methods("GET")
	r.HandleFunc("/api2/json/nodes/{node}/tasks/{upid}", mockpve.HandleStopTask(state)).Methods("DELETE")
	r.HandleFunc("/api2/json/access/ticket", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"data": {"ticket": "dummy", "CSRFPreventionToken": "dummy"}}`)
	})
	server := httptest.NewServer(r)
	defer server.Close()

	config := &mockConfig{addr: server.URL}
	opts := api.ClientOption(func(c *api.ClientOptions) {
		c.Logger = &mockLogger{}
		c.Cache = &interfaces.NoOpCache{}
	})
	client, _ := api.NewClient(config, opts)

	resolver := func(nodeName string) (*api.Client, error) {
		return client, nil
	}
	tm := NewTaskManager(resolver, nil)
	defer tm.Stop()

	upid := state.CreateTask("pve", "qmstart", "101", "root@pam")
	task := &Task{
		TargetVMID: 101,
		TargetNode: "pve",
		Type:       "Start",
		Operation: func() (string, error) {
			return upid, nil
		},
	}
	tm.Enqueue(task)

	// Wait for running
	assert.Eventually(t, func() bool {
		running := tm.GetActiveTaskForVM("pve", 101)
		return running != nil && running.Status == StatusRunning
	}, 1*time.Second, 10*time.Millisecond)

	// Cancel
	err := tm.CancelTask(task.ID)
	assert.NoError(t, err)

	// Wait for status to become failed/completed (since StopTask in mock just errors it out or stops it)
	// Mock HandleStopTask calls CompleteTask(..., "ERROR")

	assert.Eventually(t, func() bool {
		return tm.GetActiveTaskForVM("pve", 101) == nil
	}, 6*time.Second, 100*time.Millisecond)
}

func TestTaskManager_AllowsSameVMIDOnDifferentNodes(t *testing.T) {
	state := mockpve.NewMockState()
	r := mux.NewRouter()
	r.HandleFunc("/api2/json/nodes/{node}/tasks/{upid}/status", mockpve.HandleTaskStatus(state)).Methods("GET")
	r.HandleFunc("/api2/json/nodes/{node}/tasks/{upid}", mockpve.HandleStopTask(state)).Methods("DELETE")
	r.HandleFunc("/api2/json/access/ticket", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"data": {"ticket": "dummy", "CSRFPreventionToken": "dummy"}}`)
	})
	server := httptest.NewServer(r)
	defer server.Close()

	config := &mockConfig{addr: server.URL}
	opts := api.ClientOption(func(c *api.ClientOptions) {
		c.Logger = &mockLogger{}
		c.Cache = &interfaces.NoOpCache{}
	})
	client, err := api.NewClient(config, opts)
	require.NoError(t, err)

	resolver := func(nodeName string) (*api.Client, error) {
		return client, nil
	}
	tm := NewTaskManager(resolver, nil)
	defer tm.Stop()

	upidNode1 := state.CreateTask("pve", "qmstart", "100", "root@pam")
	upidNode2 := state.CreateTask("pve2", "qmstart", "100", "root@pam")

	taskNode1 := &Task{
		TargetVMID: 100,
		TargetNode: "pve",
		Type:       "Start",
		Operation: func() (string, error) {
			return upidNode1, nil
		},
	}
	taskNode2 := &Task{
		TargetVMID: 100,
		TargetNode: "pve2",
		Type:       "Start",
		Operation: func() (string, error) {
			return upidNode2, nil
		},
	}

	tm.Enqueue(taskNode1)
	tm.Enqueue(taskNode2)

	assert.Eventually(t, func() bool {
		t1 := tm.GetActiveTaskForVM("pve", 100)
		t2 := tm.GetActiveTaskForVM("pve2", 100)
		return t1 != nil && t2 != nil && t1.Status == StatusRunning && t2.Status == StatusRunning
	}, 1*time.Second, 50*time.Millisecond)

	state.CompleteTask(upidNode1, "OK")
	state.CompleteTask(upidNode2, "OK")

	assert.Eventually(t, func() bool {
		return tm.GetActiveTaskForVM("pve", 100) == nil && tm.GetActiveTaskForVM("pve2", 100) == nil
	}, 5*time.Second, 100*time.Millisecond)
}

type mockConfig struct {
	addr string
}

func (m *mockConfig) GetAddr() string        { return m.addr }
func (m *mockConfig) GetUser() string        { return "user" }
func (m *mockConfig) GetPassword() string    { return "pass" }
func (m *mockConfig) GetRealm() string       { return "pam" }
func (m *mockConfig) GetAPIToken() string    { return "" }
func (m *mockConfig) GetTokenID() string     { return "" }
func (m *mockConfig) GetTokenSecret() string { return "" }
func (m *mockConfig) GetInsecure() bool      { return true }
func (m *mockConfig) IsUsingTokenAuth() bool { return false }
