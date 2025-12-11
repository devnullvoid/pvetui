package taskpoller

import (
	"context"
	"sync"
	"time"

	"github.com/devnullvoid/pvetui/pkg/api"
	"github.com/devnullvoid/pvetui/pkg/api/interfaces"
)

// TaskInfo holds information about a monitored task.
type TaskInfo struct {
	UPID       string
	Node       string
	VMID       int
	Status     string
	ExitStatus string
	StartTime  time.Time
	client     *api.Client
}

// Poller handles background task monitoring.
type Poller struct {
	tasks          map[string]*TaskInfo
	tasksMu        sync.RWMutex
	callbacks      map[int]func(task *TaskInfo)
	nextCallbackID int
	callbackMu     sync.Mutex
	logger         interfaces.Logger
	ctx            context.Context
	cancel         context.CancelFunc
}

// New creates a new task poller.
func New(ctx context.Context, logger interfaces.Logger) *Poller {
	ctx, cancel := context.WithCancel(ctx)
	return &Poller{
		tasks:     make(map[string]*TaskInfo),
		callbacks: make(map[int]func(task *TaskInfo)),
		logger:    logger,
		ctx:       ctx,
		cancel:    cancel,
	}
}

// Stop stops the poller.
func (p *Poller) Stop() {
	p.cancel()
}

// AddTask starts monitoring a task.
func (p *Poller) AddTask(client *api.Client, upid, node string, vmid int) {
	p.tasksMu.Lock()
	defer p.tasksMu.Unlock()

	if _, exists := p.tasks[upid]; exists {
		return
	}

	task := &TaskInfo{
		UPID:      upid,
		Node:      node,
		VMID:      vmid,
		Status:    "running",
		StartTime: time.Now(),
		client:    client,
	}
	p.tasks[upid] = task

	// Notify listeners of start
	p.notifyListeners(task)

	go p.monitor(task)
}

func (p *Poller) monitor(task *TaskInfo) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-p.ctx.Done():
			return
		case <-ticker.C:
			status, err := task.client.GetTaskStatus(task.Node, task.UPID)
			if err != nil {
				// Log error?
				continue
			}

			if status.Status == "stopped" {
				p.tasksMu.Lock()
				// Update info
				task.Status = "stopped"
				task.ExitStatus = status.ExitStatus
				// Remove from active list
				delete(p.tasks, task.UPID)
				p.tasksMu.Unlock()

				p.notifyListeners(task)
				return
			}
		}
	}
}

func (p *Poller) notifyListeners(task *TaskInfo) {
	p.callbackMu.Lock()
	callbacks := make([]func(task *TaskInfo), 0, len(p.callbacks))
	for _, cb := range p.callbacks {
		callbacks = append(callbacks, cb)
	}
	p.callbackMu.Unlock()

	for _, cb := range callbacks {
		go cb(task)
	}
}

// Subscribe adds a callback for task updates and returns an unregister function.
func (p *Poller) Subscribe(cb func(task *TaskInfo)) func() {
	p.callbackMu.Lock()
	defer p.callbackMu.Unlock()
	id := p.nextCallbackID
	p.nextCallbackID++
	p.callbacks[id] = cb

	return func() {
		p.callbackMu.Lock()
		delete(p.callbacks, id)
		p.callbackMu.Unlock()
	}
}

// GetActiveTasksForVM returns a list of active tasks for a specific VM.
func (p *Poller) GetActiveTasksForVM(vmid int) []*TaskInfo {
	p.tasksMu.RLock()
	defer p.tasksMu.RUnlock()
	var result []*TaskInfo
	for _, t := range p.tasks {
		if t.VMID == vmid {
			result = append(result, t)
		}
	}
	return result
}
