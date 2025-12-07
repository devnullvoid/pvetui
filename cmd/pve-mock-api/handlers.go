package main

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
)

func handleClusterResources(state *MockState) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		resources := state.GetClusterResources()

		response := map[string]interface{}{
			"data": resources,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}
}

func handleClusterStatus(state *MockState) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Mock cluster status
        state.mu.RLock()
        nodes := make([]map[string]interface{}, 0)
        for _, n := range state.Nodes {
            nodes = append(nodes, map[string]interface{}{
                "id": n.ID,
                "name": n.Name,
                "type": "node",
                "ip": n.IP,
                "online": n.Online,
                "local": 1,
            })
        }
        state.mu.RUnlock()

		response := map[string]interface{}{
			"data": nodes,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}
}

func handleNodeStatus(state *MockState) http.HandlerFunc {
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
                "status": "online",
                "cpu": 0.1,
                "uptime": node.Uptime,
                "kversion": node.KernelVersion,
                "pveversion": node.PVEVersion,
                "cpuinfo": map[string]interface{}{
                    "cores": node.CPUCores,
                    "cpus": node.CPUCores * node.CPUSockets,
                    "sockets": node.CPUSockets,
                    "model": node.CPUModel,
                },
                "memory": map[string]interface{}{
                    "total": node.MaxMem,
                    "used": node.MaxMem / 4,
                },
                "rootfs": map[string]interface{}{
                    "total": node.MaxDisk,
                    "used": node.MaxDisk / 4,
                },
                "loadavg": []string{"0.1", "0.2", "0.3"},
            },
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}
}

func handleVMStatusAction(state *MockState) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		vmid := vars["vmid"]
		action := vars["action"]

        err := state.UpdateVMStatus(vmid, action)
        if err != nil {
            http.Error(w, err.Error(), http.StatusNotFound)
            return
        }

        // Return UPID (task ID) as Proxmox does
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(map[string]interface{}{
            "data": "UPID:pve:00000000:00000000:00000000:task:id:root@pam:",
        })
	}
}

func handleDeleteVM(state *MockState) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		vmid := vars["vmid"]

		err := state.DeleteVM(vmid)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(map[string]interface{}{
            "data": "UPID:pve:00000000:00000000:00000000:task:id:root@pam:",
        })
	}
}

func handleVMStatusCurrent(state *MockState) http.HandlerFunc {
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
                "status": vm.Status,
                "vmid": vm.ID,
                "name": vm.Name,
                "cpus": vm.CPUs,
                "maxmem": vm.MaxMem,
                "maxdisk": vm.MaxDisk,
                "uptime": vm.Uptime,
                "ha": map[string]interface{}{"managed": 0},
                "qmpstatus": vm.Status,
            },
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}
}

func handleVMConfig(state *MockState) http.HandlerFunc {
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
            w.Header().Set("Content-Type", "application/json")
            json.NewEncoder(w).Encode(response)
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
                state.UpdateVMConfig(vmid, updates)
            }

             w.Header().Set("Content-Type", "application/json")
             json.NewEncoder(w).Encode(map[string]interface{}{
                "data": nil,
            })
        }
	}
}
