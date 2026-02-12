package mockpve

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
)

func HandleClusterResources(state *MockState) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		resources := state.GetClusterResources()

		response := map[string]interface{}{
			"data": resources,
		}

		writeJSON(w, http.StatusOK, response)
	}
}

func HandleClusterStatus(state *MockState) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Mock cluster status
		state.mu.RLock()
		nodes := make([]map[string]interface{}, 0)
		for _, n := range state.Nodes {
			nodes = append(nodes, map[string]interface{}{
				"id":     n.ID,
				"name":   n.Name,
				"type":   "node",
				"ip":     n.IP,
				"online": n.Online,
				"local":  1,
			})
		}
		state.mu.RUnlock()

		response := map[string]interface{}{
			"data": nodes,
		}

		writeJSON(w, http.StatusOK, response)
	}
}

func HandleNodeStatus(state *MockState) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		nodeName := vars["node"]

		state.mu.RLock()
		var node *MockNode
		for _, n := range state.Nodes {
			if n.Name == nodeName {
				node = n
				break
			}
		}
		state.mu.RUnlock()

		if node == nil {
			http.Error(w, "Node not found", http.StatusNotFound)
			return
		}

		response := map[string]interface{}{
			"data": map[string]interface{}{
				"status":     "online",
				"cpu":        0.1,
				"uptime":     node.Uptime,
				"kversion":   node.KernelVersion,
				"pveversion": node.PVEVersion,
				"cpuinfo": map[string]interface{}{
					"cores":   node.CPUCores,
					"cpus":    node.CPUCores * node.CPUSockets,
					"sockets": node.CPUSockets,
					"model":   node.CPUModel,
				},
				"memory": map[string]interface{}{
					"total": node.MaxMem,
					"used":  node.MaxMem / 4,
				},
				"rootfs": map[string]interface{}{
					"total": node.MaxDisk,
					"used":  node.MaxDisk / 4,
				},
				"loadavg": []string{"0.1", "0.2", "0.3"},
			},
		}

		writeJSON(w, http.StatusOK, response)
	}
}

func HandleVMStatusAction(state *MockState) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		vmid := vars["vmid"]
		action := vars["action"]

		upid, err := state.UpdateVMStatus(vmid, action)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}

		// Return UPID (task ID) as Proxmox does
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"data": upid,
		})
	}
}

func HandleDeleteVM(state *MockState) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		vmid := vars["vmid"]

		err := state.DeleteVM(vmid)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"data": "UPID:pve:00000000:00000000:00000000:task:id:root@pam:",
		})
	}
}

func HandleVMStatusCurrent(state *MockState) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		vmid := vars["vmid"]

		state.mu.RLock()
		vm := state.VMs[vmid]
		state.mu.RUnlock()

		if vm == nil {
			http.Error(w, "VM not found", http.StatusNotFound)
			return
		}

		response := map[string]interface{}{
			"data": map[string]interface{}{
				"status":    vm.Status,
				"vmid":      vm.ID,
				"name":      vm.Name,
				"cpus":      vm.CPUs,
				"maxmem":    vm.MaxMem,
				"maxdisk":   vm.MaxDisk,
				"uptime":    vm.Uptime,
				"ha":        map[string]interface{}{"managed": 0},
				"qmpstatus": vm.Status,
			},
		}

		writeJSON(w, http.StatusOK, response)
	}
}

func HandleVMConfig(state *MockState) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		vmid := vars["vmid"]

		state.mu.RLock()
		vm := state.VMs[vmid]
		state.mu.RUnlock()

		if vm == nil {
			http.Error(w, "VM not found", http.StatusNotFound)
			return
		}

		if r.Method == "GET" {
			response := map[string]interface{}{
				"data": vm.Config,
			}
			writeJSON(w, http.StatusOK, response)
			return
		}

		if r.Method == "POST" || r.Method == "PUT" {
			var updates map[string]interface{}
			contentType := r.Header.Get("Content-Type")

			if strings.Contains(contentType, "application/json") {
				_ = json.NewDecoder(r.Body).Decode(&updates)
			} else {
				if err := r.ParseForm(); err == nil {
					updates = make(map[string]interface{})
					for k, v := range r.Form {
						if len(v) > 0 {
							updates[k] = v[0]
						}
					}
				}
			}

			if updates != nil {
				if err := state.UpdateVMConfig(vmid, updates); err != nil {
					http.Error(w, err.Error(), http.StatusNotFound)
					return
				}
			}

			writeJSON(w, http.StatusOK, map[string]interface{}{
				"data": nil,
			})
		}
	}
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	if status > 0 {
		w.WriteHeader(status)
	}
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("mock-api: failed to encode JSON response: %v", err)
	}
}

func HandleVzdump(state *MockState) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var params map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&params)

		vmidStr := ""
		if v, ok := params["vmid"].(string); ok {
			vmidStr = v
		}
		storage := ""
		if v, ok := params["storage"].(string); ok {
			storage = v
		}
		mode := ""
		if v, ok := params["mode"].(string); ok {
			mode = v
		}
		notes := ""
		if v, ok := params["notes-template"].(string); ok {
			notes = v
		}

		var vmid int
		_, _ = fmt.Sscanf(vmidStr, "%d", &vmid)

		upid := state.CreateBackup(vmid, storage, mode, notes)

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"data": upid,
		})
	}
}

func HandleStorageContent(state *MockState) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		storage := vars["storage"]
		content := r.URL.Query().Get("content")

		if content == "backup" {
			backups := state.GetBackups(storage)

			var data []map[string]interface{}
			for _, b := range backups {
				data = append(data, map[string]interface{}{
					"volid":        b.VolID,
					"vmid":         b.VMID,
					"size":         b.Size,
					"ctime":        b.Date,
					"format":       b.Format,
					"notes":        b.Notes,
					"content":      b.Content,
					"verification": "ok",
				})
			}

			writeJSON(w, http.StatusOK, map[string]interface{}{
				"data": data,
			})
			return
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"data": []interface{}{},
		})
	}
}

func HandleDeleteStorageContent(state *MockState) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		volume := vars["volume"]

		err := state.DeleteBackup(volume)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"data": "UPID:pve:00000000:00000000:00000000:task:delete:root@pam:",
		})
	}
}

func HandleRestore(state *MockState) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"data": "UPID:pve:00000000:00000000:00000000:task:qmrestore:root@pam:",
		})
	}
}

func HandleTaskStatus(state *MockState) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		upid := vars["upid"]

		state.mu.RLock()
		task, ok := state.Tasks[upid]
		state.mu.RUnlock()

		if !ok {
			http.Error(w, "Task not found", http.StatusNotFound)
			return
		}

		data := map[string]interface{}{
			"status":    task.Status,
			"pid":       1234,
			"starttime": task.StartTime,
			"id":        task.ID,
			"type":      task.Type,
			"user":      task.User,
			"node":      task.Node,
			"upid":      task.UPID,
		}

		if task.Status == taskStatusStopped {
			data["exitstatus"] = task.ExitStatus
			data["endtime"] = task.EndTime
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"data": data,
		})
	}
}

func HandleStopTask(state *MockState) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		upid := vars["upid"]

		state.CompleteTask(upid, "ERROR") // Stopped tasks usually have exit status "ERROR" or similar, or just stopped.

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"data": nil,
		})
	}
}
