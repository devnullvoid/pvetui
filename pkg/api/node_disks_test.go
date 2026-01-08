package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/devnullvoid/pvetui/pkg/api/interfaces"
)

func TestGetNodeDisks(t *testing.T) {
	mockResponse := `{
		"data": [
			{
				"devpath": "/dev/sda",
				"health": "PASSED",
				"model": "Samsung SSD 860",
				"serial": "S4XBNF0M123456",
				"size": 500107862016,
				"type": "ssd",
				"used": "partitions"
			}
		]
	}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api2/json/access/ticket" {
			w.Write([]byte(`{"data":{"ticket":"ticket","CSRFPreventionToken":"token","username":"user@pam"}}`))
			return
		}
		if r.URL.Path != "/api2/json/nodes/pve1/disks/list" {
			t.Errorf("Expected path /api2/json/nodes/pve1/disks/list, got %s", r.URL.Path)
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
	// Important: Provide a NoOpCache to avoid nil pointer panic
	client, err := NewClient(mockConfig, WithCache(&interfaces.NoOpCache{}))
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	disks, err := client.GetNodeDisks("pve1")
	if err != nil {
		t.Fatalf("GetNodeDisks failed: %v", err)
	}

	if len(disks) != 1 {
		t.Fatalf("Expected 1 disk, got %d", len(disks))
	}

	if disks[0].Model != "Samsung SSD 860" {
		t.Errorf("Expected model 'Samsung SSD 860', got '%s'", disks[0].Model)
	}

	if disks[0].Size != 500107862016 {
		t.Errorf("Expected size 500107862016, got %d", disks[0].Size)
	}
}

func TestGetNodeDiskSmart(t *testing.T) {
	mockResponse := `{
		"data": {
			"health": "PASSED",
			"type": "ssd",
			"text": "Self-test passed",
			"attributes": [
				{
					"id": 5,
					"name": "Reallocated_Sector_Ct",
					"value": 100,
					"worst": 100,
					"thresh": 10,
					"fail": false,
					"raw": "0"
				}
			]
		}
	}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api2/json/access/ticket" {
			w.Write([]byte(`{"data":{"ticket":"ticket","CSRFPreventionToken":"token","username":"user@pam"}}`))
			return
		}
		expectedPath := "/api2/json/nodes/pve1/disks/smart"
		if r.URL.Path != expectedPath {
			t.Errorf("Expected path %s, got %s", expectedPath, r.URL.Path)
		}
		if r.URL.Query().Get("disk") != "/dev/sda" {
			t.Errorf("Expected disk query param /dev/sda, got %s", r.URL.Query().Get("disk"))
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

	smart, err := client.GetNodeDiskSmart("pve1", "/dev/sda")
	if err != nil {
		t.Fatalf("GetNodeDiskSmart failed: %v", err)
	}

	if smart.Health != "PASSED" {
		t.Errorf("Expected health PASSED, got %s", smart.Health)
	}

	if len(smart.Attributes) != 1 {
		t.Fatalf("Expected 1 attribute, got %d", len(smart.Attributes))
	}

	if smart.Attributes[0].Name != "Reallocated_Sector_Ct" {
		t.Errorf("Expected attribute name Reallocated_Sector_Ct, got %s", smart.Attributes[0].Name)
	}
}
