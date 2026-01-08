package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/devnullvoid/pvetui/pkg/api/interfaces"
)

func TestGetNodeUpdates(t *testing.T) {
	mockResponse := `{
		"data": [
			{
				"Package": "pve-manager",
				"Title": "Proxmox VE Manager",
				"Version": "7.4-3",
				"OldVersion": "7.3-6",
				"Arch": "amd64",
				"Description": "The Proxmox Virtual Environment Manager",
				"Origin": "Proxmox"
			}
		]
	}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api2/json/access/ticket" {
			w.Write([]byte(`{"data":{"ticket":"ticket","CSRFPreventionToken":"token","username":"user@pam"}}`))
			return
		}
		if r.URL.Path != "/api2/json/nodes/pve1/apt/update" {
			t.Errorf("Expected path /api2/json/nodes/pve1/apt/update, got %s", r.URL.Path)
		}
		w.Write([]byte(mockResponse))
	}))
	defer server.Close()

	mockConfig := &MockConfig{
		Addr:     server.URL,
		User:     "user",
		Password: "password",
		Realm:    "pam",
		Insecure: true,
	}
	client, err := NewClient(mockConfig, WithCache(&interfaces.NoOpCache{}))
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	updates, err := client.GetNodeUpdates("pve1")
	if err != nil {
		t.Fatalf("GetNodeUpdates failed: %v", err)
	}

	if len(updates) != 1 {
		t.Fatalf("Expected 1 update, got %d", len(updates))
	}

	if updates[0].Package != "pve-manager" {
		t.Errorf("Expected package 'pve-manager', got '%s'", updates[0].Package)
	}

	if updates[0].OldVersion != "7.3-6" {
		t.Errorf("Expected old version '7.3-6', got '%s'", updates[0].OldVersion)
	}
}
