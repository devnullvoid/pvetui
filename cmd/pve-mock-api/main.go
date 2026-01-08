package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/routers/gorillamux"
	"github.com/gorilla/mux"
)

func main() {
	var specFile string
	var port int

	flag.StringVar(&specFile, "spec", "docs/api/pve-openapi.yaml", "Path to OpenAPI spec file")
	flag.IntVar(&port, "port", 8080, "Port to listen on")
	flag.Parse()

	ctx := context.Background()
	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromFile(specFile)
	if err != nil {
		log.Fatalf("Failed to load spec file %s: %v", specFile, err)
	}

	if err := doc.Validate(ctx); err != nil {
		log.Printf("Warning: Spec validation failed: %v", err)
	}

	router, err := gorillamux.NewRouter(doc)
	if err != nil {
		log.Fatalf("Failed to create openapi router: %v", err)
	}

	r := mux.NewRouter()

	// Initialize State
	state := NewMockState()

	// Stateful Handlers (Priority)
	r.HandleFunc("/cluster/resources", handleClusterResources(state)).Methods("GET")
	r.HandleFunc("/cluster/status", handleClusterStatus(state)).Methods("GET")

	// Node
	r.HandleFunc("/nodes/{node}/status", handleNodeStatus(state)).Methods("GET")
	r.HandleFunc("/nodes/{node}/disks/list", handleNodeDisksList(state)).Methods("GET")
	r.HandleFunc("/nodes/{node}/disks/smart", handleNodeDiskSmart(state)).Methods("GET")
	r.HandleFunc("/nodes/{node}/apt/update", handleNodeUpdates(state)).Methods("GET")

	// VM/CT
	r.HandleFunc("/nodes/{node}/{type:qemu|lxc}/{vmid:[0-9]+}/status/current", handleVMStatusCurrent(state)).Methods("GET")
	r.HandleFunc("/nodes/{node}/{type:qemu|lxc}/{vmid:[0-9]+}/status/{action}", handleVMStatusAction(state)).Methods("POST")
	r.HandleFunc("/nodes/{node}/{type:qemu|lxc}/{vmid:[0-9]+}/config", handleVMConfig(state)).Methods("GET", "POST", "PUT")
	r.HandleFunc("/nodes/{node}/{type:qemu|lxc}/{vmid:[0-9]+}", handleDeleteVM(state)).Methods("DELETE")

	// Backups
	r.HandleFunc("/nodes/{node}/vzdump", handleVzdump(state)).Methods("POST")
	r.HandleFunc("/nodes/{node}/storage/{storage}/content", handleStorageContent(state)).Methods("GET")
	r.HandleFunc("/nodes/{node}/storage/{storage}/content/{volume:.+}", handleDeleteStorageContent(state)).Methods("DELETE")

	// Restore (Create VM/CT)
	r.HandleFunc("/nodes/{node}/qemu", handleRestore(state)).Methods("POST")
	r.HandleFunc("/nodes/{node}/lxc", handleRestore(state)).Methods("POST")

	// Generic Fallback
	r.PathPrefix("/").HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		log.Printf("%s %s", req.Method, req.URL.Path)

		route, _, err := router.FindRoute(req)
		if err != nil {
			log.Printf("Route not found: %v", err)
			http.Error(w, "Not Found", http.StatusNotFound)
			return
		}

		if route.Operation == nil {
			log.Printf("Operation is nil")
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}

		statusCode := 200
		var respSchema *openapi3.Schema

		// Deterministic response selection
		responses := route.Operation.Responses.Map()

		if ref, ok := responses["200"]; ok && ref.Value != nil {
			if content := ref.Value.Content.Get("application/json"); content != nil {
				if content.Schema != nil {
					respSchema = content.Schema.Value
				}
			}
		} else {
			var keys []string
			for k := range responses {
				if strings.HasPrefix(k, "2") && k != "default" {
					keys = append(keys, k)
				}
			}
			sort.Strings(keys)

			if len(keys) > 0 {
				status := keys[0]
				if _, err := fmt.Sscanf(status, "%d", &statusCode); err != nil {
					log.Printf("mock-api: failed to parse status code %s: %v", status, err)
					statusCode = 200
				}
				if ref := responses[status]; ref != nil && ref.Value != nil {
					if content := ref.Value.Content.Get("application/json"); content != nil {
						if content.Schema != nil {
							respSchema = content.Schema.Value
						}
					}
				}
			} else {
				if def := route.Operation.Responses.Default(); def != nil && def.Value != nil {
					if content := def.Value.Content.Get("application/json"); content != nil {
						if content.Schema != nil {
							respSchema = content.Schema.Value
						}
					}
				}
			}
		}

		if respSchema == nil {
			w.WriteHeader(statusCode)
			if _, err := w.Write([]byte("{}")); err != nil {
				log.Printf("mock-api: failed to write empty response: %v", err)
			}
			return
		}

		data := generateMockData(respSchema, 0)

		// Wrap in "data" property as Proxmox API does
		response := map[string]interface{}{
			"data": data,
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Printf("Failed to encode response: %v", err)
		}
	})

	log.Printf("Starting mock server on :%d using spec %s", port, specFile)

	// Handle /api2/json prefix
	finalHandler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		req.URL.Path = strings.TrimPrefix(req.URL.Path, "/api2/json")
		r.ServeHTTP(w, req)
	})

	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      finalHandler,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  30 * time.Second,
	}
	log.Fatal(server.ListenAndServe())
}

func generateMockData(schema *openapi3.Schema, depth int) interface{} {
	if schema == nil || depth > 10 {
		return nil
	}

	if len(schema.OneOf) > 0 {
		if schema.OneOf[0].Value != nil {
			return generateMockData(schema.OneOf[0].Value, depth+1)
		}
	}
	if len(schema.AnyOf) > 0 {
		if schema.AnyOf[0].Value != nil {
			return generateMockData(schema.AnyOf[0].Value, depth+1)
		}
	}

	if len(schema.AllOf) > 0 {
		result := make(map[string]interface{})
		foundObject := false
		for _, subRef := range schema.AllOf {
			if subRef.Value != nil {
				subData := generateMockData(subRef.Value, depth+1)
				if subMap, ok := subData.(map[string]interface{}); ok {
					foundObject = true
					for k, v := range subMap {
						result[k] = v
					}
				}
			}
		}
		if len(schema.Properties) > 0 {
			localData := generateProperties(schema, depth)
			if localMap, ok := localData.(map[string]interface{}); ok {
				foundObject = true
				for k, v := range localMap {
					result[k] = v
				}
			}
		}
		if foundObject {
			return result
		}
		if len(schema.AllOf) > 0 && schema.AllOf[0].Value != nil {
			return generateMockData(schema.AllOf[0].Value, depth+1)
		}
	}

	if schema.Type != nil {
		if schema.Type.Is("boolean") {
			return true
		}
		if schema.Type.Is("integer") {
			return 1
		}
		if schema.Type.Is("number") {
			return 1.5
		}
		if schema.Type.Is("string") {
			if len(schema.Enum) > 0 {
				return schema.Enum[0]
			}
			if schema.Format == "date-time" {
				return "2024-01-01T00:00:00Z"
			}
			return "mock_string"
		}
		if schema.Type.Is("array") {
			if schema.Items != nil && schema.Items.Value != nil {
				return []interface{}{generateMockData(schema.Items.Value, depth+1)}
			}
			return []interface{}{}
		}
		if schema.Type.Is("object") {
			return generateProperties(schema, depth)
		}
	}

	if len(schema.Properties) > 0 {
		return generateProperties(schema, depth)
	}

	return nil
}

func generateProperties(schema *openapi3.Schema, depth int) interface{} {
	res := make(map[string]interface{})
	for name, propRef := range schema.Properties {
		if propRef.Value != nil {
			res[name] = generateMockData(propRef.Value, depth+1)
		}
	}
	return res
}
