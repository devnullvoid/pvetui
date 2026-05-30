package mockpve

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFixtureAndApplyFixture(t *testing.T) {
	t.Parallel()

	fixturePath := filepath.Join(t.TempDir(), "fixture.yml")
	fixtureYAML := []byte(`
replace: true
next_id: 200
task_delay_ms: 10
nodes:
  - name: lab
    id: node/lab
    online: 1
    ip: 192.0.2.10
    maxcpu: 8
    maxmem: 17179869184
    maxdisk: 107374182400
storage:
  - id: local
    node: lab
    type: dir
    content: iso,backup
    maxdisk: 107374182400
    status: active
vms:
  - id: 150
    name: app
    node: lab
    type: lxc
    status: running
    maxmem: 1073741824
    maxdisk: 8589934592
    cpus: 2
storage_content:
  - volid: local:iso/debian.iso
    node: lab
    storage: local
    content: iso
    format: iso
    size: 1024
    used: 1024
tasks:
  - upid: "UPID:lab:00000001:00000001:00000001:vzstart:150:root@pam:"
    node: lab
    type: vzstart
    id: "150"
    user: root@pam
    starttime: 20
    endtime: 25
    status: stopped
    exitstatus: OK
  - upid: "UPID:lab:00000002:00000002:00000002:vzdump:150:root@pam:"
    node: lab
    type: vzdump
    id: "150"
    user: root@pam
    starttime: 10
    endtime: 15
    status: stopped
    exitstatus: OK
`)

	if err := os.WriteFile(fixturePath, fixtureYAML, 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	fixture, err := LoadFixture(fixturePath)
	if err != nil {
		t.Fatalf("LoadFixture failed: %v", err)
	}

	state := NewMockState()
	if err := state.ApplyFixture(fixture); err != nil {
		t.Fatalf("ApplyFixture failed: %v", err)
	}

	if got := len(state.Nodes); got != 1 {
		t.Fatalf("expected 1 node after replace, got %d", got)
	}
	if got := state.Nodes[0].Name; got != "lab" {
		t.Fatalf("expected node lab, got %q", got)
	}
	if got := state.NextID; got != 200 {
		t.Fatalf("expected next id 200, got %d", got)
	}

	vm := state.VMs["150"]
	if vm == nil {
		t.Fatal("expected fixture VM 150")
	}
	if got := vm.Config["hostname"]; got != "app" {
		t.Fatalf("expected LXC hostname to be normalized, got %v", got)
	}

	storage := state.Storage[0]
	if got := storage.Disk; got != 1024 {
		t.Fatalf("expected storage usage to be recalculated, got %d", got)
	}

	tasks := state.ListClusterTasks()
	if got := len(tasks); got != 2 {
		t.Fatalf("expected 2 tasks, got %d", got)
	}
	if got := tasks[0]["type"]; got != "vzstart" {
		t.Fatalf("expected tasks sorted by descending starttime, got first type %v", got)
	}
}
