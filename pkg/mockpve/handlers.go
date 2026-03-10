package mockpve

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/gorilla/mux"
)

const maxFormBodySize = 1 << 20 // 1 MiB

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

func HandleClusterNextID(state *MockState) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		requestedID := 0
		if raw := r.URL.Query().Get("vmid"); raw != "" {
			parsed, err := strconv.Atoi(raw)
			if err != nil {
				http.Error(w, "invalid vmid", http.StatusBadRequest)
				return
			}
			requestedID = parsed
		}

		nextID, err := state.GetNextID(requestedID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"data": nextID,
		})
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
			updates, err := decodeRequestParams(w, r)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
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
		params, err := decodeRequestParams(w, r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

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
		node := vars["node"]
		storage := vars["storage"]
		content := r.URL.Query().Get("content")
		vmid, _ := strconv.Atoi(r.URL.Query().Get("vmid"))

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"data": state.ListStorageContent(node, storage, content, vmid),
		})
	}
}

func HandleDeleteStorageContent(state *MockState) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		volume := vars["volume"]

		upid, err := state.QueueDeleteStorageContent(volume)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"data": upid,
		})
	}
}

func HandleGuestCreate(state *MockState) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		node := vars["node"]
		vmType := vars["type"]
		if vmType == "" {
			if strings.Contains(r.URL.Path, "/qemu") {
				vmType = "qemu"
			} else {
				vmType = "lxc"
			}
		}

		params, err := decodeRequestParams(w, r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		isRestore := false
		if vmType == "qemu" {
			isRestore = getBoolParam(params, "force", false) && getStringParam(params, "archive", "") != ""
			if getStringParam(params, "archive", "") != "" {
				isRestore = true
			}
		} else {
			isRestore = getBoolParam(params, "restore", false)
		}

		upid, err := state.QueueCreateGuest(node, vmType, params, isRestore)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"data": upid,
		})
	}
}

func HandleNodeStorages(state *MockState) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		node := vars["node"]

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"data": state.ListNodeStorages(node),
		})
	}
}

func HandleGuestIndex(state *MockState) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		node := vars["node"]
		vmType := vars["type"]

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"data": state.ListGuests(node, vmType),
		})
	}
}

func HandleResizeGuestDisk(state *MockState) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		node := vars["node"]
		vmType := vars["type"]
		vmid := vars["vmid"]

		params, err := decodeRequestParams(w, r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		disk := getStringParam(params, "disk", "")
		size := getStringParam(params, "size", "")
		if disk == "" || size == "" {
			http.Error(w, "disk and size are required", http.StatusBadRequest)
			return
		}

		upid, err := state.QueueResizeGuestDisk(node, vmType, vmid, disk, size)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"data": upid,
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

func decodeRequestParams(w http.ResponseWriter, r *http.Request) (map[string]interface{}, error) {
	var params map[string]interface{}
	contentType := r.Header.Get("Content-Type")

	if strings.Contains(contentType, "application/json") {
		if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
			return nil, fmt.Errorf("invalid json payload")
		}
		if params == nil {
			params = make(map[string]interface{})
		}
		return params, nil
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxFormBodySize)
	if err := r.ParseForm(); err != nil {
		return nil, fmt.Errorf("invalid form payload")
	}

	params = make(map[string]interface{}, len(r.Form))
	for k, v := range r.Form {
		if len(v) > 0 {
			params[k] = v[0]
		}
	}

	return params, nil
}
