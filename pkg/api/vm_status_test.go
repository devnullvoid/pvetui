package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/devnullvoid/pvetui/pkg/api/interfaces"
	"github.com/stretchr/testify/require"
)

func TestGetVmStatusParsesCompoundQEMUAgentSetting(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case testAPITicketPath:
			_, _ = w.Write([]byte(`{"data":{"ticket":"ticket","CSRFPreventionToken":"token","username":"user@pam"}}`))
		case "/api2/json/nodes/pve1/qemu/100/status/current":
			_, _ = w.Write([]byte(`{"data":{"name":"nixos-vm","status":"running","cpu":0,"mem":0,"maxmem":1073741824,"disk":0,"maxdisk":10737418240}}`))
		case "/api2/json/nodes/pve1/qemu/100/config":
			_, _ = w.Write([]byte(`{"data":{"agent":"enabled=1,fstrim_cloned_disks=0,type=virtio","net0":"virtio=AA:BB:CC:DD:EE:FF,bridge=vmbr0"}}`))
		case "/api2/json/nodes/pve1/qemu/100/agent/network-get-interfaces":
			_, _ = w.Write([]byte(`{"data":{"result":[{"name":"ens18","hardware-address":"aa:bb:cc:dd:ee:ff","ip-addresses":[{"ip-address":"192.0.2.50","ip-address-type":"ipv4","prefix":24}]}]}}`))
		case "/api2/json/nodes/pve1/qemu/100/agent/get-fsinfo":
			_, _ = w.Write([]byte(`{"data":{"result":[]}}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client, err := NewClient(&MockConfig{
		Addr:     server.URL,
		User:     "user",
		Password: "password",
		Realm:    "pam",
		Insecure: true,
	}, WithCache(&interfaces.NoOpCache{}))
	require.NoError(t, err)

	vm := &VM{ID: 100, Node: "pve1", Type: VMTypeQemu}
	require.NoError(t, client.GetVmStatus(vm))

	require.True(t, vm.AgentEnabled)
	require.True(t, vm.AgentRunning)
	require.Equal(t, "192.0.2.50", vm.IP)
}
