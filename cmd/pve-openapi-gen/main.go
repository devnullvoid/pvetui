package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// PathEntry represents one API path with HTTP methods.
type PathEntry struct {
	Path string
	Info map[string]map[string]interface{}
}

// MethodSpec represents a single HTTP method description.
type MethodSpec struct {
	Description string
	Parameters  map[string]interface{}
	Returns     map[string]interface{}
}

func main() {
	var input, output, outJSON, indexOut, version, url, include, sourceVersion string
	var useLocal bool
	flag.StringVar(&input, "in", "docs/local/apidoc.js", "Path to apidoc.js (used when --use-local)")
	flag.StringVar(&output, "out", "docs/api/pve-openapi.yaml", "Output OpenAPI YAML file")
	flag.StringVar(&outJSON, "out-json", "", "Optional JSON output path for the OpenAPI spec")
	flag.StringVar(&indexOut, "index-out", "", "Optional JSON index of paths (path, method, summary, operationId)")
	flag.StringVar(&version, "version", "generated", "API version label to embed in spec")
	flag.StringVar(&url, "url", "https://pve.proxmox.com/pve-docs/api-viewer/apidoc.js", "Remote apidoc.js URL to fetch")
	flag.StringVar(&include, "include-prefix", "", "Comma-separated path prefixes to include (defaults to all)")
	flag.StringVar(&sourceVersion, "source-version", "", "Optional Proxmox source version label (e.g., 'PVE 9.x')")
	flag.BoolVar(&useLocal, "use-local", false, "Read apidoc.js from --in instead of downloading from --url")
	flag.Parse()

	if err := run(input, output, outJSON, indexOut, version, url, useLocal, include, sourceVersion); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

const objectType = "object"

func run(input, output, outJSON, indexOut, version, url string, useLocal bool, include string, sourceVersion string) error {
	var data []byte
	var err error

	if useLocal {
		data, err = os.ReadFile(input) //nolint:gosec // path is user-provided CLI flag
		if err != nil {
			return err
		}
	} else {
		data, err = fetchURL(url)
		if err != nil {
			return fmt.Errorf("download %s: %w", url, err)
		}
	}

	arrayText, err := extractSchemaArray(string(data))
	if err != nil {
		return fmt.Errorf("extract schema: %w", err)
	}

	apiNodes, err := parseSchemaJSON(arrayText)
	if err != nil {
		return fmt.Errorf("eval schema: %w", err)
	}

	paths := buildOpenAPIPaths(apiNodes, include)

	sourceLabel := url
	if useLocal {
		sourceLabel = input
	}
	desc := fmt.Sprintf("Generated from %s on %s", sourceLabel, time.Now().UTC().Format(time.RFC3339))
	if sourceVersion != "" {
		desc += " (source " + sourceVersion + ")"
	}
	desc += ". Paths/params are best-effort; regenerate after upgrades."

	spec := map[string]interface{}{
		"openapi": "3.0.3",
		"info": map[string]interface{}{
			"title":       "Proxmox VE API (generated from apidoc.js)",
			"version":     version,
			"description": desc,
		},
		"paths": paths,
	}

	if err := os.MkdirAll(filepath.Dir(output), 0o750); err != nil {
		return err
	}

	outData, err := yaml.Marshal(spec)
	if err != nil {
		return err
	}

	if err := os.WriteFile(output, outData, 0o600); err != nil {
		return err
	}

	if outJSON != "" {
		if err := writeJSON(outJSON, spec); err != nil {
			return err
		}
	}

	if indexOut != "" {
		idx := buildPathsIndex(paths)
		if err := writeJSON(indexOut, idx); err != nil {
			return err
		}
	}

	fmt.Printf("wrote %s (paths: %d)\n", output, len(paths))
	if outJSON != "" {
		fmt.Printf("wrote %s (json)\n", outJSON)
	}
	if indexOut != "" {
		fmt.Printf("wrote %s (paths index)\n", indexOut)
	}
	return nil
}

// extractSchemaArray pulls the `[...]` that defines apiSchema.
func extractSchemaArray(content string) (string, error) {
	idx := strings.Index(content, "const apiSchema")
	if idx == -1 {
		return "", errors.New("apiSchema not found")
	}
	// find first '[' after equals
	start := strings.Index(content[idx:], "[")
	if start == -1 {
		return "", errors.New("schema '[' not found")
	}
	start += idx

	depth := 0
	end := -1
	for i := start; i < len(content); i++ {
		switch content[i] {
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				end = i
			}
		}
		if end != -1 {
			break
		}
	}
	if end == -1 {
		return "", errors.New("schema closing ']' not found")
	}
	return content[start : end+1], nil
}

// parseSchemaJSON converts the apiSchema JS array into Go data. The snippet in
// apidoc.js is valid JSON (keys and strings are quoted), so we can unmarshal
// directly once we've sliced out the array literal.
func parseSchemaJSON(arrayText string) ([]interface{}, error) {
	var out []interface{}
	if err := json.Unmarshal([]byte(arrayText), &out); err != nil {
		return nil, err
	}
	return out, nil
}

// fetchURL downloads the content at the given URL with a short timeout.
func fetchURL(url string) ([]byte, error) {
	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Get(url) //nolint:gosec // URL is user-provided CLI flag
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %s", resp.Status)
	}
	return io.ReadAll(resp.Body)
}

// writeJSON writes data to a path with secure permissions.
func writeJSON(path string, data interface{}) error {
	bytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, bytes, 0o600)
}

// buildOpenAPIPaths converts apiSchema nodes to OpenAPI paths object.
func buildOpenAPIPaths(nodes []interface{}, include string) map[string]map[string]interface{} {
	paths := make(map[string]map[string]interface{})
	filters := parseInclude(include)
	walk(nodes, func(path string, info map[string]interface{}) {
		if path == "" || info == nil {
			return
		}
		if len(filters) > 0 && !hasPrefix(path, filters) {
			return
		}
		// ensure map exists
		if _, ok := paths[path]; !ok {
			paths[path] = make(map[string]interface{})
		}
		for method, raw := range info {
			rawMap, ok := raw.(map[string]interface{})
			if !ok {
				continue
			}
			op := buildOperation(method, path, rawMap)
			paths[path][strings.ToLower(method)] = op
		}
	})
	return paths
}

// buildPathsIndex returns a lightweight index useful for quick lookup or embedding.
func buildPathsIndex(paths map[string]map[string]interface{}) []map[string]string {
	entries := []map[string]string{}
	for p, ops := range paths {
		for method, raw := range ops {
			op, ok := raw.(map[string]interface{})
			if !ok {
				continue
			}
			summary := stringOr(op["summary"])
			opID := stringOr(op["operationId"])
			entries = append(entries, map[string]string{
				"path":        p,
				"method":      method,
				"summary":     summary,
				"operationId": opID,
			})
		}
	}
	// sort for stability
	sort.Slice(entries, func(i, j int) bool {
		if entries[i]["path"] == entries[j]["path"] {
			return entries[i]["method"] < entries[j]["method"]
		}
		return entries[i]["path"] < entries[j]["path"]
	})
	return entries
}

func walk(nodes []interface{}, fn func(path string, info map[string]interface{})) {
	for _, n := range nodes {
		obj, ok := n.(map[string]interface{})
		if !ok {
			continue
		}
		path, _ := obj["path"].(string)
		info, _ := obj["info"].(map[string]interface{})
		if path != "" {
			fn(path, info)
		}
		if children, ok := obj["children"].([]interface{}); ok {
			walk(children, fn)
		}
	}
}

var pathParamRE = regexp.MustCompile(`\{([^}]+)\}`)

func buildOperation(method, path string, info map[string]interface{}) map[string]interface{} {
	description, _ := info["description"].(string)
	params := extractParameters(path, info)
	responses := extractResponses(info)
	if strings.HasSuffix(path, "/file-restore/download") && strings.EqualFold(method, "GET") {
		responses = map[string]interface{}{
			"200": map[string]interface{}{
				"description": "File download",
				"content": map[string]interface{}{
					"application/octet-stream": map[string]interface{}{
						"schema": map[string]interface{}{
							"type":   "string",
							"format": "binary",
						},
					},
				},
			},
		}
	}

	op := map[string]interface{}{
		"summary":     description,
		"operationId": makeOperationID(method, path),
		"responses":   responses,
	}
	if len(params) > 0 {
		op["parameters"] = params
	}

	// Some Proxmox POST/PUT endpoints accept body-like params; keep simple requestBody with same schema.
	if strings.EqualFold(method, "POST") || strings.EqualFold(method, "PUT") {
		if body := buildRequestBody(info); body != nil {
			op["requestBody"] = body
		}
	}
	return op
}

func makeOperationID(method, path string) string {
	cleaned := strings.ReplaceAll(path, "/", "_")
	cleaned = strings.ReplaceAll(cleaned, "{", "")
	cleaned = strings.ReplaceAll(cleaned, "}", "")
	cleaned = strings.ReplaceAll(cleaned, "-", "_")
	return strings.ToLower(method) + cleaned
}

func extractParameters(path string, info map[string]interface{}) []map[string]interface{} {
	params := []map[string]interface{}{}
	paramNames := pathParamRE.FindAllStringSubmatch(path, -1)
	pathNames := make(map[string]struct{})
	for _, m := range paramNames {
		pathNames[m[1]] = struct{}{}
	}

	pRoot, _ := info["parameters"].(map[string]interface{})
	props := map[string]interface{}{}
	if pRoot != nil {
		if pr, ok := pRoot["properties"].(map[string]interface{}); ok {
			props = pr
		}
	}

	names := make([]string, 0, len(props))
	for k := range props {
		names = append(names, k)
	}
	sort.Strings(names)

	for _, name := range names {
		prop, _ := props[name].(map[string]interface{})
		if prop == nil {
			continue
		}
		schema := schemaFromProp(prop)
		required := false
		loc := "query"
		if _, ok := pathNames[name]; ok {
			required = true
			loc = "path"
		}
		if opt, ok := prop["optional"].(float64); ok && opt == 0 && loc != "path" {
			required = true
		}
		param := map[string]interface{}{
			"name":        name,
			"in":          loc,
			"required":    required,
			"schema":      schema,
			"description": stringOr(prop["description"]),
		}
		params = append(params, param)
	}
	return params
}

func buildRequestBody(info map[string]interface{}) map[string]interface{} {
	pRoot, _ := info["parameters"].(map[string]interface{})
	props := map[string]interface{}{}
	if pRoot != nil {
		if pr, ok := pRoot["properties"].(map[string]interface{}); ok {
			props = pr
		}
	}
	if len(props) == 0 {
		return nil
	}
	schema := map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	}
	required := []string{}
	for name, raw := range props {
		prop, _ := raw.(map[string]interface{})
		if prop == nil {
			continue
		}
		schema["properties"].(map[string]interface{})[name] = schemaFromProp(prop)
		if opt, ok := prop["optional"].(float64); ok && opt == 0 {
			required = append(required, name)
		}
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	return map[string]interface{}{
		"content": map[string]interface{}{
			"application/json": map[string]interface{}{
				"schema": schema,
			},
		},
	}
}

func extractResponses(info map[string]interface{}) map[string]interface{} {
	returns, _ := info["returns"].(map[string]interface{})
	if returns == nil {
		return singleNullResponse()
	}
	schema := schemaFromProp(returns)
	if len(schema) == 0 {
		schema["type"] = objectType
	}
	return map[string]interface{}{
		"200": map[string]interface{}{
			"description": "OK",
			"content": map[string]interface{}{
				"application/json": map[string]interface{}{
					"schema": schema,
				},
			},
		},
	}
}

// singleNullResponse returns a 200 response with a nullable object body to
// satisfy OpenAPI validators when apidoc doesn't specify a return schema.
func singleNullResponse() map[string]interface{} {
	nullSchema := map[string]interface{}{
		"type":     objectType,
		"nullable": true,
	}
	return map[string]interface{}{
		"200": map[string]interface{}{
			"description": "OK",
			"content": map[string]interface{}{
				"application/json": map[string]interface{}{
					"schema": nullSchema,
				},
			},
		},
	}
}

func schemaFromProp(prop map[string]interface{}) map[string]interface{} {
	schema := map[string]interface{}{}
	if t, ok := prop["type"].(string); ok {
		if t == "null" {
			schema["type"] = objectType
			schema["nullable"] = true
		} else {
			schema["type"] = t
		}
	}
	if enumVals, ok := prop["enum"].([]interface{}); ok && len(enumVals) > 0 {
		schema["enum"] = enumVals
	}
	if fmtStr, ok := prop["format"].(string); ok {
		schema["format"] = fmtStr
	}
	if desc := stringOr(prop["description"]); desc != "" {
		schema["description"] = desc
	}
	// Handle nested objects/arrays
	if items, ok := prop["items"].(map[string]interface{}); ok {
		schema["items"] = schemaFromProp(items)
	}
	if props, ok := prop["properties"].(map[string]interface{}); ok && len(props) > 0 {
		nested := map[string]interface{}{}
		for name, raw := range props {
			if m, ok := raw.(map[string]interface{}); ok {
				nested[name] = schemaFromProp(m)
			}
		}
		schema["type"] = objectType
		schema["properties"] = nested
	}
	return schema
}

func stringOr(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func parseInclude(include string) []string {
	if include == "" {
		return nil
	}
	parts := strings.Split(include, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func hasPrefix(path string, prefixes []string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(path, p) {
			return true
		}
	}
	return false
}
