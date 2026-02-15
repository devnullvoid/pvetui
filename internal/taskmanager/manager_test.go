package taskmanager

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
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

func TestTaskManager_RespectsGlobalMaxRunningLimit(t *testing.T) {
	const maxRunning = 2
	tm := NewTaskManagerWithMaxRunning(func(nodeName string) (*api.Client, error) {
		return nil, nil
	}, nil, maxRunning)
	defer tm.Stop()

	var currentRunning int32
	var maxObserved int32
	var once sync.Once
	startedTwo := make(chan struct{})
	release := make(chan struct{})

	makeTask := func(vmid int) *Task {
		return &Task{
			TargetVMID: vmid,
			TargetNode: "pve",
			Type:       "Start",
			Operation: func() (string, error) {
				running := atomic.AddInt32(&currentRunning, 1)
				for {
					prev := atomic.LoadInt32(&maxObserved)
					if running <= prev || atomic.CompareAndSwapInt32(&maxObserved, prev, running) {
						break
					}
				}
				if running == maxRunning {
					once.Do(func() { close(startedTwo) })
				}
				<-release
				atomic.AddInt32(&currentRunning, -1)
				return "", nil
			},
		}
	}

	tm.Enqueue(makeTask(101))
	tm.Enqueue(makeTask(102))
	tm.Enqueue(makeTask(103))
	tm.Enqueue(makeTask(104))

	select {
	case <-startedTwo:
	case <-time.After(2 * time.Second):
		t.Fatal("expected two tasks to start running")
	}

	require.LessOrEqual(t, atomic.LoadInt32(&maxObserved), int32(maxRunning))
	require.Equal(t, 4, len(tm.GetAllTasks())) // 2 running + 2 queued

	close(release)

	assert.Eventually(t, func() bool {
		return len(tm.GetAllTasks()) == 0
	}, 5*time.Second, 50*time.Millisecond)
}

func TestTaskManager_CancelQueuedTaskBeforeStart(t *testing.T) {
	tm := NewTaskManagerWithMaxRunning(func(nodeName string) (*api.Client, error) {
		return nil, nil
	}, nil, 1)
	defer tm.Stop()

	release := make(chan struct{})
	var task2Started atomic.Bool

	task1 := &Task{
		TargetVMID: 201,
		TargetNode: "pve",
		Type:       "Start",
		Operation: func() (string, error) {
			<-release
			return "", nil
		},
	}
	task2 := &Task{
		TargetVMID: 202,
		TargetNode: "pve",
		Type:       "Start",
		Operation: func() (string, error) {
			task2Started.Store(true)
			return "", nil
		},
	}

	tm.Enqueue(task1)
	tm.Enqueue(task2)

	assert.Eventually(t, func() bool {
		active1 := tm.GetActiveTaskForVM("pve", 201)
		all := tm.GetAllTasks()
		return active1 != nil && active1.Status == StatusRunning && len(all) == 2
	}, 1*time.Second, 20*time.Millisecond)

	err := tm.CancelTask(task2.ID)
	require.NoError(t, err)

	close(release)

	assert.Eventually(t, func() bool {
		return tm.GetActiveTaskForVM("pve", 201) == nil
	}, 2*time.Second, 20*time.Millisecond)

	require.False(t, task2Started.Load(), "queued task should not start after cancellation")
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
