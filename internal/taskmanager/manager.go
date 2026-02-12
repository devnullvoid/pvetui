package taskmanager

import (
	"fmt"
	"sync"
	"time"

	"github.com/devnullvoid/pvetui/pkg/api"
	"github.com/google/uuid"
)

type TaskStatus string

const (
	StatusQueued    TaskStatus = "queued"
	StatusRunning   TaskStatus = "running"
	StatusCompleted TaskStatus = "completed"
	StatusFailed    TaskStatus = "failed"
	StatusCancelled TaskStatus = "cancelled"
)

type Task struct {
	ID          string
	Type        string // "Start", "Stop", "Migrate", etc.
	Description string
	Status      TaskStatus
	Progress    int
	CreatedAt   time.Time
	StartedAt   time.Time
	FinishedAt  time.Time
	Error       error

	// Target info
	TargetVMID int
	TargetNode string
	TargetName string

	// Operation to execute. Returns UPID if async, or error.
	Operation func() (string, error)

	// Proxmox UPID
	UPID string

	// Callbacks
	OnComplete func(error)
}

// ClientResolver is a function that returns an API client for a given node.
type ClientResolver func(nodeName string) (*api.Client, error)

type TaskManager struct {
	mu             sync.RWMutex
	queue          map[string][]*Task // Key: node/vmid
	activeTasks    map[string]*Task   // Key: node/vmid
	clientResolver ClientResolver
	updateNotify   func()

	stopChan chan struct{}
}

func NewTaskManager(clientResolver ClientResolver, updateNotify func()) *TaskManager {
	return &TaskManager{
		queue:          make(map[string][]*Task),
		activeTasks:    make(map[string]*Task),
		clientResolver: clientResolver,
		updateNotify:   updateNotify,
		stopChan:       make(chan struct{}),
	}
}

func taskKey(node string, vmid int) string {
	return fmt.Sprintf("%s/%d", node, vmid)
}

func (tm *TaskManager) Enqueue(task *Task) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if task.ID == "" {
		task.ID = uuid.New().String()
	}
	task.Status = StatusQueued
	task.CreatedAt = time.Now()

	key := taskKey(task.TargetNode, task.TargetVMID)

	// Check if there is an active task for this VM on this node
	if _, active := tm.activeTasks[key]; !active {
		// No active task, start immediately
		tm.activeTasks[key] = task
		go tm.runTask(task)
	} else {
		// Active task exists, append to queue
		tm.queue[key] = append(tm.queue[key], task)
	}

	if tm.updateNotify != nil {
		tm.updateNotify()
	}
}

func (tm *TaskManager) runTask(task *Task) {
	// Update status to running
	tm.mu.Lock()
	task.Status = StatusRunning
	task.StartedAt = time.Now()
	tm.mu.Unlock()

	if tm.updateNotify != nil {
		tm.updateNotify()
	}

	// Execute operation
	upid, err := task.Operation()
	if err != nil {
		tm.completeTask(task, StatusFailed, err)
		return
	}

	// If no UPID returned, it was a sync op (unlikely for VM ops but possible)
	if upid == "" {
		tm.completeTask(task, StatusCompleted, nil)
		return
	}

	// Store UPID
	tm.mu.Lock()
	task.UPID = upid
	tm.mu.Unlock()

	// Poll for completion
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-tm.stopChan:
			return
		case <-ticker.C:
			client, err := tm.clientResolver(task.TargetNode)
			if err != nil {
				// Failed to resolve client (maybe node moved or group issue), retry
				continue
			}

			status, err := client.GetTaskStatus(task.TargetNode, upid)
			if err != nil {
				// Log error but continue polling? or fail?
				// Maybe temporary network error.
				continue
			}

			if status.Status == "stopped" {
				if status.ExitStatus == "OK" {
					tm.completeTask(task, StatusCompleted, nil)
				} else {
					tm.completeTask(task, StatusFailed, fmt.Errorf("%s", status.ExitStatus))
				}
				return
			}

			// Optional: Update progress? Proxmox tasks don't always report percentage.
		}
	}
}

func (tm *TaskManager) completeTask(task *Task, status TaskStatus, err error) {
	tm.mu.Lock()
	task.Status = status
	task.Error = err
	task.FinishedAt = time.Now()

	key := taskKey(task.TargetNode, task.TargetVMID)

	// Remove from active
	delete(tm.activeTasks, key)

	// Check queue for next task
	var nextTask *Task
	if queue, ok := tm.queue[key]; ok && len(queue) > 0 {
		nextTask = queue[0]
		tm.queue[key] = queue[1:]
		if len(tm.queue[key]) == 0 {
			delete(tm.queue, key)
		}

		// Set as active
		tm.activeTasks[key] = nextTask
	}
	tm.mu.Unlock()

	if task.OnComplete != nil {
		go task.OnComplete(err)
	}

	if tm.updateNotify != nil {
		tm.updateNotify()
	}

	if nextTask != nil {
		go tm.runTask(nextTask)
	}
}

func (tm *TaskManager) CancelTask(taskID string) error {
	// First check queues and remove if found (fast operation, keep lock)
	tm.mu.Lock()
	for key, queue := range tm.queue {
		for i, task := range queue {
			if task.ID == taskID {
				// Remove from queue
				tm.queue[key] = append(queue[:i], queue[i+1:]...)
				if len(tm.queue[key]) == 0 {
					delete(tm.queue, key)
				}
				task.Status = StatusCancelled
				task.FinishedAt = time.Now()
				tm.mu.Unlock()

				if tm.updateNotify != nil {
					tm.updateNotify()
				}
				return nil
			}
		}
	}

	// Check active tasks
	var targetUPID string
	var targetNode string
	var found bool

	for _, task := range tm.activeTasks {
		if task.ID == taskID {
			if task.UPID != "" {
				targetUPID = task.UPID
				targetNode = task.TargetNode
				found = true
			} else {
				tm.mu.Unlock()
				return fmt.Errorf("task is starting, cannot cancel yet")
			}
			break
		}
	}
	tm.mu.Unlock()

	if found {
		client, err := tm.clientResolver(targetNode)
		if err != nil {
			return err
		}
		// Try to stop in Proxmox (network call outside lock)
		if err := client.StopTask(targetNode, targetUPID); err != nil {
			return err
		}
		// The polling loop will see it stopped
		return nil
	}

	return fmt.Errorf("task not found")
}

func (tm *TaskManager) GetActiveTask(vmid int) *Task {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	for _, task := range tm.activeTasks {
		if task.TargetVMID == vmid {
			return task
		}
	}
	return nil
}

func (tm *TaskManager) GetActiveTaskForVM(node string, vmid int) *Task {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return tm.activeTasks[taskKey(node, vmid)]
}

func (tm *TaskManager) GetAllTasks() []*Task {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	var tasks []*Task
	for _, t := range tm.activeTasks {
		tasks = append(tasks, t)
	}
	for _, queue := range tm.queue {
		tasks = append(tasks, queue...)
	}
	return tasks
}

func (tm *TaskManager) Stop() {
	close(tm.stopChan)
}
