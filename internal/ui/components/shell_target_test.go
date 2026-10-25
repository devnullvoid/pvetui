package components

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/devnullvoid/pvetui/internal/config"
	"github.com/devnullvoid/pvetui/pkg/api"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/stretchr/testify/require"
)

func TestGuestContextMenuShellTarget(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("mock SSH executable requires a Unix shell")
	}
	bash, err := exec.LookPath("bash")
	require.NoError(t, err)
	dir := t.TempDir()
	stub := "#!" + bash + "\nprintf '%s\\n' \"$@\" > \"$PVETUI_TEST_SSH_ARGS\"\n"
	// #nosec G306 -- private mock executable requires owner execute permission.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ssh"), []byte(stub), 0o700))
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	stdinPath := filepath.Join(dir, "stdin")
	require.NoError(t, os.WriteFile(stdinPath, []byte("\n\n\n\n\n\n"), 0o600))
	// #nosec G304 -- test-owned input fixture in a private temporary directory.
	stdin, err := os.Open(stdinPath)
	require.NoError(t, err)
	originalStdin := os.Stdin
	os.Stdin = stdin
	t.Cleanup(func() {
		os.Stdin = originalStdin
		_ = stdin.Close()
	})

	for _, tc := range []struct {
		name      string
		guestType string
		changed   bool
		filtered  bool
		shortcut  bool
		direct    bool
		missingIP bool
	}{
		{name: "QEMU second guest", guestType: api.VMTypeQemu},
		{name: "QEMU menu shortcut", guestType: api.VMTypeQemu, shortcut: true},
		{name: "QEMU selection changes", guestType: api.VMTypeQemu, changed: true},
		{name: "QEMU shortcut selection changes", guestType: api.VMTypeQemu, changed: true, shortcut: true},
		{name: "QEMU filtered list replaced", guestType: api.VMTypeQemu, filtered: true},
		{name: "QEMU missing IP never falls back to first guest", guestType: api.VMTypeQemu, changed: true, missingIP: true},
		{name: "LXC second guest", guestType: api.VMTypeLXC},
		{name: "LXC selection changes", guestType: api.VMTypeLXC, changed: true},
		{name: "keyboard shell follows current selection", guestType: api.VMTypeQemu, direct: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			argsPath := filepath.Join(t.TempDir(), "ssh-args")
			t.Setenv("PVETUI_TEST_SSH_ARGS", argsPath)
			first := &api.VM{ID: 100, Name: "first", Node: "pve-a", SourceProfile: "profile-a", Type: tc.guestType, Status: api.VMStatusRunning, IP: "192.0.2.10"}
			second := &api.VM{ID: 101, Name: "second", Node: "pve-b", SourceProfile: "profile-b", Type: tc.guestType, Status: api.VMStatusRunning, IP: "192.0.2.11"}
			if tc.missingIP {
				second.IP = ""
			}
			clusterNodes := []*api.Node{
				{Name: "pve-a", IP: "192.0.2.200", VMs: []*api.VM{first}},
				{Name: "pve-b", IP: "192.0.2.201", VMs: []*api.VM{second}},
			}
			guests := NewVMList()
			nodes := NewNodeList()
			nodes.SetNodes(clusterNodes)
			app := &App{
				Application: tview.NewApplication(), ctx: context.Background(),
				config: config.Config{SSHUser: "fallback", VMSSHUser: "fallback", Profiles: map[string]config.ProfileConfig{
					"profile-a": {SSHUser: "root", VMSSHUser: "first-user"},
					"profile-b": {SSHUser: "root", VMSSHUser: "guest-user"},
				}},
				client: &api.Client{Cluster: &api.Cluster{Nodes: clusterNodes}},
				pages:  tview.NewPages(), nodeList: nodes, vmList: guests,
			}
			screen := tcell.NewSimulationScreen("UTF-8")
			app.SetScreen(screen)
			t.Cleanup(screen.Fini)
			guests.SetVMs([]*api.VM{second, first})
			guests.app = app
			guests.SetCurrentItem(1)
			app.pages.AddPage(api.PageGuests, guests, true, true)
			app.SetRoot(app.pages, true).SetFocus(guests)
			if tc.direct {
				app.openVMShell()
			} else {
				app.ShowVMContextMenu()
				require.Equal(t, second, guests.GetSelectedVM())
				if tc.changed {
					guests.SetCurrentItem(0)
				} else if tc.filtered {
					guests.SetVMs([]*api.VM{first})
				}
				menu := app.contextMenu
				event := tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)
				if tc.shortcut {
					event = tcell.NewEventKey(tcell.KeyRune, 's', tcell.ModNone)
				}
				menu.InputHandler()(event, func(tview.Primitive) {})
			}
			// #nosec G304 -- reads the mock SSH recorder's test-owned temporary file.
			args, err := os.ReadFile(argsPath)
			if tc.missingIP {
				require.True(t, os.IsNotExist(err), "SSH must not run without the intended guest IP")
				require.True(t, app.pages.HasPage("shell_error"))
				return
			}
			require.NoError(t, err)
			want := []string{"guest-user@192.0.2.11"}
			if tc.guestType == api.VMTypeLXC {
				want = []string{"root@192.0.2.201", "-t", "pct enter 101"}
			}
			require.Equal(t, want, strings.Split(strings.TrimSpace(string(args)), "\n"))
		})
	}
}
