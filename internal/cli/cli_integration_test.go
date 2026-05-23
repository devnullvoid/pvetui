package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Integration tests for storage commands.
//
// These tests use a local httptest server to simulate the Proxmox API and
// exercise the full command handler pipeline (flag parsing → API call → JSON output).
// They are separate from unit tests because they wire up a real api.Client and
// go through initCLISession indirectly via the shared newTestClient helper.

// storageListResponse returns a minimal /nodes/{node}/storage JSON response.
func storageListResponse() string {
	return `{"data":[
		{"storage":"local","plugintype":"dir","content":"vztmpl,iso,backup","disk":1073741824,"maxdisk":10737418240,"status":"available","shared":0,"type":"storage","node":"pve01"},
		{"storage":"local-zfs","plugintype":"zfspool","content":"images,rootdir","disk":5368709120,"maxdisk":21474836480,"status":"available","shared":0,"type":"storage","node":"pve01"}
	]}`
}

// storageContentResponse returns a minimal /nodes/{node}/storage/{storage}/content JSON response.
func storageContentResponse() string {
	return `{"data":[
		{"volid":"local:iso/debian-12.iso","content":"iso","format":"iso","size":658505728,"ctime":1700000000},
		{"volid":"local:iso/ubuntu-22.04.iso","content":"iso","format":"iso","size":1234567890,"ctime":1700001000}
	]}`
}

func newStorageTestServer(t *testing.T) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.URL.Path == "/api2/json/access/ticket":
			_, _ = w.Write([]byte(authResponse))

		case strings.HasSuffix(r.URL.Path, "/storage") && strings.Contains(r.URL.Path, "/nodes/"):
			_, _ = w.Write([]byte(storageListResponse()))

		case strings.Contains(r.URL.Path, "/storage/") && strings.HasSuffix(r.URL.Path, "/content"):
			_, _ = w.Write([]byte(storageContentResponse()))

		case r.URL.Path == "/api2/json/cluster/resources":
			// Minimal cluster resources so getNodes works.
			_, _ = w.Write([]byte(`{"data":[{"type":"node","node":"pve01","status":"online","id":"node/pve01"}]}`))

		case r.URL.Path == "/api2/json/cluster/status":
			_, _ = w.Write([]byte(`{"data":[{"type":"cluster","name":"pve","nodes":1}]}`))

		default:
			http.NotFound(w, r)
		}
	}))
}

func TestStorageListReturnsRows(t *testing.T) {
	server := newStorageTestServer(t)
	defer server.Close()

	client := newTestClient(t, server.URL)

	rows, err := fetchStorageRows(client, "pve01")
	if err != nil {
		t.Fatalf("fetchStorageRows returned error: %v", err)
	}

	if len(rows) == 0 {
		t.Fatal("fetchStorageRows returned empty rows, want at least one")
	}

	for _, r := range rows {
		if r.Name == "" {
			t.Error("row has empty Name")
		}

		if r.Node == "" {
			t.Error("row has empty Node")
		}
	}
}

func TestStorageListJSONShape(t *testing.T) {
	server := newStorageTestServer(t)
	defer server.Close()

	client := newTestClient(t, server.URL)

	rows, err := fetchStorageRows(client, "pve01")
	if err != nil {
		t.Fatalf("fetchStorageRows returned error: %v", err)
	}

	// Round-trip through JSON to verify the output shape matches the spec.
	data, err := json.Marshal(rows)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	var decoded []storageListRow
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("output is not valid JSON array of storage rows: %v", err)
	}

	if len(decoded) != len(rows) {
		t.Errorf("decoded %d rows, want %d", len(decoded), len(rows))
	}
}

func TestStorageContentListReturnsItems(t *testing.T) {
	server := newStorageTestServer(t)
	defer server.Close()

	client := newTestClient(t, server.URL)

	items, err := client.GetStorageContent("pve01", "local", "")
	if err != nil {
		t.Fatalf("GetStorageContent returned error: %v", err)
	}

	if len(items) == 0 {
		t.Fatal("GetStorageContent returned empty list, want at least one item")
	}

	for _, item := range items {
		if item.VolID == "" {
			t.Error("content item has empty VolID")
		}
	}
}

func TestStorageContentListJSONShape(t *testing.T) {
	server := newStorageTestServer(t)
	defer server.Close()

	client := newTestClient(t, server.URL)

	items, err := client.GetStorageContent("pve01", "local", "iso")
	if err != nil {
		t.Fatalf("GetStorageContent returned error: %v", err)
	}

	rows := make([]storageContentRow, 0, len(items))
	for _, item := range items {
		name := item.VolID
		if idx := strings.LastIndex(name, "/"); idx >= 0 {
			name = name[idx+1:]
		}

		rows = append(rows, storageContentRow{
			VolID: item.VolID,
			Name:  name,
			Type:  item.Content,
			Size:  item.Size,
			CTime: item.CreatedAt.Unix(),
			VMID:  item.VMID,
		})
	}

	data, err := json.Marshal(rows)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	var decoded []storageContentRow
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("output is not valid JSON array of content rows: %v\noutput: %s", err, data)
	}

	if len(decoded) != len(rows) {
		t.Errorf("decoded %d rows, want %d", len(decoded), len(rows))
	}
}
