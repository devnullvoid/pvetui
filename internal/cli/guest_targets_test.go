package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/devnullvoid/pvetui/internal/adapters"
	"github.com/devnullvoid/pvetui/internal/cache"
	"github.com/devnullvoid/pvetui/internal/config"
	"github.com/devnullvoid/pvetui/pkg/api"
	"github.com/devnullvoid/pvetui/pkg/api/interfaces"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func TestParseGuestTargetID(t *testing.T) {
	for _, tc := range []struct {
		target  string
		id      int
		invalid bool
	}{
		{"177", 177, false}, {"00177", 177, false}, {"docker-test", 0, false},
		{"177-test", 0, false}, {"", 0, true}, {"0", 0, true}, {"-1", 0, true},
		{"999999999999999999999999999999", 0, true},
	} {
		t.Run(tc.target, func(t *testing.T) {
			id, err := parseGuestTargetID(tc.target)
			if tc.invalid {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.id, id)
			}
		})
	}
}

func TestMatchGuestName(t *testing.T) {
	guests := []*api.VM{
		nil,
		{ID: 177, Name: "docker-test", Node: "mars", Type: "lxc", SourceProfile: "prod"},
		{ID: 200, Name: "docker-test", Node: "pve1", Type: "qemu", SourceProfile: "test"},
	}
	_, err := matchGuestName(guests, "docker-test", "")
	require.ErrorContains(t, err, "ambiguous")
	for _, detail := range []string{"177", "200", "mars", "pve1", "prod", "test"} {
		require.Contains(t, err.Error(), detail)
	}
	vm, err := matchGuestName(guests, "docker-test", "lxc")
	require.NoError(t, err)
	require.Equal(t, 177, vm.ID)
	for _, name := range []string{"docker", "DOCKER-test", "missing"} {
		_, err := matchGuestName(guests, name, "")
		require.ErrorContains(t, err, "not found")
	}
}

func TestGuestCompletions(t *testing.T) {
	guests := []*api.VM{
		{ID: 177, Name: "docker-test", Node: "mars", Type: "lxc"},
		{ID: 200, Name: "docker-test", Node: "pve1", Type: "qemu"},
		{ID: 201, Name: "177", Node: "pve1", Type: "qemu"},
	}
	values := guestCompletions(guests, "dock", "")
	require.Len(t, values, 1)
	require.True(t, strings.HasPrefix(values[0], "docker-test\t"))
	require.Contains(t, values[0], "ID 177")
	require.Contains(t, values[0], "ID 200")
	values = guestCompletions(guests, "dock", "lxc")
	require.Len(t, values, 1)
	require.NotContains(t, values[0], "ID 200")
	values = guestCompletions(guests, "177", "")
	require.Len(t, values, 1)
	require.Contains(t, values[0], "docker-test")
}

func guestTargetServer(t *testing.T, node string, id int, rejectInventory bool) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/access/ticket"):
			_, _ = w.Write([]byte(authResponse))
		case strings.HasSuffix(r.URL.Path, "/cluster/resources"):
			if rejectInventory {
				http.Error(w, "inventory unavailable", http.StatusServiceUnavailable)
				return
			}
			_, _ = fmt.Fprintf(w, `{"data":[{"type":"node","node":%q,"status":"online"},{"type":"node","node":%q,"status":"online"},{"vmid":%d,"name":"docker-test","type":"lxc","node":%q,"status":"running"},{"vmid":178,"name":"stopped-test","type":"lxc","node":%q,"status":"stopped"}]}`, node, node+"-dest", id, node, node)
		case strings.HasSuffix(r.URL.Path, "/cluster/status"):
			ip := "192.0.2.1"
			if id != 177 {
				ip = "192.0.2.2"
			}
			_, _ = fmt.Fprintf(w, `{"data":[{"type":"node","name":%q,"online":1,"ip":%q},{"type":"node","name":%q,"online":1,"ip":"192.0.2.3"}]}`, node, ip, node+"-dest")
		case r.URL.Path == "/api2/json/nodes":
			_, _ = fmt.Fprintf(w, `{"data":[{"node":%q,"status":"online"}]}`, node)
		case r.URL.Path == "/api2/json/nodes/"+node+"/lxc":
			_, _ = fmt.Fprintf(w, `{"data":[{"vmid":%d,"name":"docker-test","status":"running"}]}`, id)
		case r.URL.Path == "/api2/json/nodes/"+node+"/qemu":
			_, _ = w.Write([]byte(`{"data":[]}`))
		case strings.HasSuffix(r.URL.Path, "/status/current"):
			if strings.Contains(r.URL.Path, "/178/") {
				_, _ = w.Write([]byte(`{"data":{"name":"stopped-test","status":"stopped"}}`))
				return
			}
			_, _ = w.Write([]byte(`{"data":{"name":"docker-test","status":"running"}}`))
		case strings.HasSuffix(r.URL.Path, "/config"):
			_, _ = w.Write([]byte(`{"data":{"hostname":"docker-test"}}`))
		case strings.HasSuffix(r.URL.Path, "/interfaces"):
			_, _ = w.Write([]byte(`{"data":[]}`))
		case r.Method == http.MethodPost:
			_, _ = w.Write([]byte(`{"data":"UPID:mars:00000001:00000001:00000001:vzstart:177:root@pam:"}`))
		case r.Method == http.MethodPut || r.Method == http.MethodDelete:
			_, _ = w.Write([]byte(`{"data":null}`))
		default:
			http.NotFound(w, r)
		}
	}))
}

func TestNodeScopedGuestTarget(t *testing.T) {
	server := guestTargetServer(t, "mars", 177, true)
	defer server.Close()
	session := &cliSession{single: newTestClient(t, server.URL)}
	cmd := newGuestsShowCmd()
	require.NoError(t, cmd.Flags().Set("node", "mars"))
	vm, err := resolveGuestTarget(context.Background(), cmd, session, "docker-test", false)
	require.NoError(t, err)
	require.Equal(t, 177, vm.ID)
	require.Equal(t, "lxc", vm.Type, "the default qemu flag must not override name inference")
	require.NoError(t, cmd.Flags().Set("type", "qemu"))
	_, err = resolveGuestTarget(context.Background(), cmd, session, "docker-test", false)
	require.ErrorContains(t, err, "not found")
	require.NoError(t, cmd.Flags().Set("type", "lxc"))
	vm, err = resolveGuestTarget(context.Background(), cmd, session, "177", true)
	require.NoError(t, err)
	require.Equal(t, 177, vm.ID)
}

func TestGroupGuestNameAmbiguityAndIncompleteInventory(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(fmt.Sprint(fail), func(t *testing.T) {
			one := guestTargetServer(t, "mars", 177, false)
			defer one.Close()
			two := guestTargetServer(t, "pve1", 200, fail)
			defer two.Close()
			manager := api.NewGroupClientManager("test", &interfaces.NoOpLogger{}, &interfaces.NoOpCache{})
			defer manager.Close()
			var profiles []api.ProfileEntry
			for i, url := range []string{one.URL, two.URL} {
				cfg := &config.Config{Addr: url, User: "root", Password: "test", Realm: "pam"}
				profiles = append(profiles, api.ProfileEntry{Name: fmt.Sprintf("pve%d", i+1), Config: adapters.NewConfigAdapter(cfg)})
			}
			require.NoError(t, manager.Initialize(context.Background(), profiles))
			session := &cliSession{group: manager}
			_, err := resolveGuestTarget(context.Background(), newGuestsShowCmd(), session, "docker-test", false)
			if fail {
				require.ErrorContains(t, err, "incomplete guest inventory")
			} else {
				require.ErrorContains(t, err, "ambiguous")
			}
			cmd := newGuestsShowCmd()
			require.NoError(t, cmd.Flags().Set("node", "mars"))
			vm, err := resolveGuestTarget(context.Background(), cmd, session, "docker-test", false)
			require.NoError(t, err)
			require.Equal(t, "pve1", vm.SourceProfile)
			client, err := session.clientForVM(vm)
			require.NoError(t, err)
			pc, _ := manager.GetClient("pve1")
			require.Same(t, pc.Client, client)
		})
	}
}

func guestTargetTestRoot(path string) *cobra.Command {
	root := &cobra.Command{Use: "pvetui", SilenceUsage: true, SilenceErrors: true}
	addPersistentFlags(root)
	root.PersistentFlags().String("output", "json", "Output format")
	root.AddCommand(newGuestsCmd(), newCommunityScriptsCmd())
	root.SetArgs([]string{"--config", path, "--no-cache"})
	return root
}

func TestNamedGuestCommandsDispatch(t *testing.T) {
	server := guestTargetServer(t, "mars", 177, false)
	defer server.Close()
	path := filepath.Join(t.TempDir(), "config.yml")
	require.NoError(t, os.WriteFile(path, []byte(fmt.Sprintf("addr: %s\nuser: root\npassword: test\nrealm: pam\nquiet_startup: true\n", server.URL)), 0o600))
	for _, args := range [][]string{
		{"show", "docker-test"}, {"start", "docker-test"},
		{"stop", "docker-test"}, {"shutdown", "docker-test"},
		{"restart", "docker-test"}, {"delete", "docker-test", "--no-wait"},
		{"resize", "docker-test", "rootfs", "+10G", "--no-wait"},
		{"migrate", "docker-test", "mars-dest", "--no-wait"},
	} {
		t.Run(args[0], func(t *testing.T) {
			root := guestTargetTestRoot(path)
			root.SetArgs(append([]string{"--config", path, "--no-cache", "guests"}, args...))
			var runErr error
			output := captureStdout(t, func() { runErr = root.Execute() })
			require.NoError(t, runErr)
			var data map[string]any
			require.NoError(t, json.Unmarshal([]byte(output), &data), output)
			if args[0] == "show" {
				require.Equal(t, float64(177), data["id"])
			} else {
				require.Equal(t, float64(177), data["vmid"])
			}
		})
	}
	for _, args := range [][]string{
		{"__complete", "guests", "shell", "dock"},
		{"__complete", "community-scripts", "plan", "dockge", "--guest", "dock"},
	} {
		root := guestTargetTestRoot(path)
		root.SetArgs(append([]string{"--config", path, "--no-cache"}, args...))
		var runErr error
		output := captureStdout(t, func() { runErr = root.Execute() })
		require.NoError(t, runErr)
		require.Contains(t, output, "docker-test\tID 177")
	}
	for _, args := range [][]string{{"shell", "stopped-test"}, {"exec", "stopped-test", "uptime"}} {
		root := guestTargetTestRoot(path)
		root.SetArgs(append([]string{"--config", path, "--no-cache", "guests"}, args...))
		var runErr error
		_ = captureStdout(t, func() { runErr = root.Execute() })
		require.ErrorContains(t, runErr, "guest 178 is not running")
	}
}

func TestDirectNumericLifecycleSkipsDiscovery(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/access/ticket"):
			_, _ = w.Write([]byte(authResponse))
		case r.URL.Path == "/api2/json/nodes/mars/lxc/177/status/start" && r.Method == http.MethodPost:
			_, _ = w.Write([]byte(`{"data":"UPID:test"}`))
		default:
			t.Errorf("unexpected discovery request: %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	path := filepath.Join(t.TempDir(), "config.yml")
	require.NoError(t, os.WriteFile(path, []byte(fmt.Sprintf("addr: %s\nuser: root\npassword: test\nrealm: pam\nquiet_startup: true\n", server.URL)), 0o600))
	root := guestTargetTestRoot(path)
	root.SetArgs([]string{"--config", path, "--no-cache", "guests", "start", "177", "--node", "mars", "--type", "lxc"})
	var runErr error
	_ = captureStdout(t, func() { runErr = root.Execute() })
	require.NoError(t, runErr)
}

func TestGuestInventoryCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/access/ticket") {
			_, _ = w.Write([]byte(authResponse))
			return
		}
		<-r.Context().Done()
	}))
	defer server.Close()
	client := newTestClient(t, server.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()
	_, err := client.ListClusterGuests(ctx)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	_, err = client.ListNodeGuestsContext(ctx, "mars")
	require.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestGuestNodeUsesSourceProfile(t *testing.T) {
	one := guestTargetServer(t, "mars", 177, false)
	defer one.Close()
	two := guestTargetServer(t, "mars", 200, false)
	defer two.Close()
	manager := api.NewGroupClientManager("test", &interfaces.NoOpLogger{}, &interfaces.NoOpCache{})
	defer manager.Close()
	var profiles []api.ProfileEntry
	for i, url := range []string{one.URL, two.URL} {
		cfg := &config.Config{Addr: url, User: "root", Password: "test", Realm: "pam"}
		profiles = append(profiles, api.ProfileEntry{Name: fmt.Sprintf("pve%d", i+1), Config: adapters.NewConfigAdapter(cfg)})
	}
	require.NoError(t, manager.Initialize(context.Background(), profiles))
	session := &cliSession{group: manager}
	node, err := session.findGuestNode(&api.VM{ID: 200, Node: "mars", SourceProfile: "pve2"})
	require.NoError(t, err)
	require.Equal(t, "192.0.2.2", node.IP)
	require.Equal(t, "pve2", node.SourceProfile)
	_, err = session.clientForGuestNode(context.Background(), "mars")
	require.ErrorContains(t, err, "ambiguous across profiles")
}

func TestCommunityScriptGuestName(t *testing.T) {
	server := guestTargetServer(t, "mars", 177, false)
	defer server.Close()
	session := &cliSession{single: newTestClient(t, server.URL)}
	node, vm, out, err := resolveCommunityScriptTarget(context.Background(), session, "", "docker-test")
	require.NoError(t, err)
	require.Equal(t, "192.0.2.1", node.IP)
	require.Equal(t, 177, vm.ID)
	require.Equal(t, 177, out.ID)
	_, _, _, err = resolveCommunityScriptTarget(context.Background(), session, "mars", "docker-test")
	require.ErrorContains(t, err, "mutually exclusive")
}

func TestGuestCompletionDeadline(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/access/ticket") {
			_, _ = w.Write([]byte(authResponse))
			return
		}
		<-r.Context().Done()
	}))
	defer server.Close()
	path := filepath.Join(t.TempDir(), "config.yml")
	require.NoError(t, os.WriteFile(path, []byte(fmt.Sprintf("addr: %s\nuser: root\npassword: test\nrealm: pam\nquiet_startup: true\n", server.URL)), 0o600))
	root := guestTargetTestRoot(path)
	shell, _, err := root.Find([]string{"guests", "shell"})
	require.NoError(t, err)
	shell.ValidArgsFunction = func(cmd *cobra.Command, args []string, prefix string) ([]string, cobra.ShellCompDirective) {
		return completeGuestTargetsWithin(cmd, args, prefix, 100*time.Millisecond)
	}
	root.SetArgs([]string{"--config", path, "--no-cache", "__complete", "guests", "shell", "dock"})
	var runErr error
	start := time.Now()
	output := captureStdout(t, func() { runErr = root.Execute() })
	require.NoError(t, runErr)
	require.Contains(t, output, ":4")
	require.Less(t, time.Since(start), 2*time.Second)
}

func TestSlowGuestCompletion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/access/ticket") {
			_, _ = w.Write([]byte(authResponse))
			return
		}
		select {
		case <-time.After(3100 * time.Millisecond):
			_, _ = w.Write([]byte(`{"data":[{"vmid":177,"name":"docker-test","type":"lxc","node":"mars"}]}`))
		case <-r.Context().Done():
		}
	}))
	defer server.Close()
	path := filepath.Join(t.TempDir(), "config.yml")
	require.NoError(t, os.WriteFile(path, []byte(fmt.Sprintf("addr: %s\nuser: root\npassword: test\nrealm: pam\nquiet_startup: true\n", server.URL)), 0o600))
	root := guestTargetTestRoot(path)
	root.SetArgs([]string{"--config", path, "--no-cache", "__complete", "guests", "shell", "dock"})
	var runErr error
	output := captureStdout(t, func() { runErr = root.Execute() })
	require.NoError(t, runErr)
	require.Contains(t, output, "docker-test\tID 177")
}

func TestCompletionInventoryCache(t *testing.T) {
	server := guestTargetServer(t, "mars", 177, false)
	defer server.Close()
	cfg := &config.Config{Addr: server.URL + "/api2/json", User: "root", Password: "test", Realm: "pam"}
	client, err := api.NewClient(adapters.NewConfigAdapter(cfg))
	require.NoError(t, err)
	completionCache := cache.NewMemoryCache()
	session := &cliSession{single: client, cfg: cfg, completionCache: completionCache}
	t.Cleanup(func() { _ = completionCache.Close() })
	guests, err := session.completionInventory(context.Background(), "")
	require.NoError(t, err)
	require.NotEmpty(t, guests)
	server.Close()
	// A new completion can use cached suggestions even if inventory is unavailable.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	cached, err := session.completionInventory(ctx, "")
	require.NoError(t, err)
	require.Equal(t, guests, cached)
	// Neither actual targeting nor another node/principal may use those suggestions.
	_, err = session.guestInventory(ctx, "")
	require.Error(t, err)
	_, err = session.completionInventory(ctx, "mars")
	require.Error(t, err)
	cfg.User = "another-user"
	_, err = session.completionInventory(ctx, "")
	require.Error(t, err)
	session.completionCache = nil
	cfg.User = "root"
	_, err = session.completionInventory(ctx, "")
	require.Error(t, err)
}
