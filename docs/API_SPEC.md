# Proxmox API OpenAPI Spec (generated)

This repo can generate an OpenAPI 3.0 spec from Proxmox VE's official `apidoc.js` and serve a local viewer.

## TL;DR

```bash
# Generate the spec from the latest upstream apidoc.js (default)
make gen-openapi

# Serve Redoc viewer in the foreground (Ctrl+C to stop)
make openapi-serve
# or start/stop in background
make openapi-serve-start
make openapi-serve-stop

# Open http://localhost:8080 in your browser (Redoc)
```

## Generator details
- Tool: `cmd/pve-openapi-gen`
- Default source: `https://pve.proxmox.com/pve-docs/api-viewer/apidoc.js`
- Output: `docs/api/pve-openapi.yaml`
- Version label in `info.version` comes from `make`'s `VERSION` (git describe fallback).
- Optional flags:
  - `--use-local --in docs/local/apidoc.js` to use a pinned copy
  - `--url <custom>` to point at another apidoc.js source
  - `--include-prefix=/nodes,/cluster` to emit a smaller, scoped spec (useful for browsing/agents)
  - `--source-version <label>` to annotate the spec with the PVE release you generated from (e.g., `PVE 9.1`)
  - `--out-json <file>` to also emit JSON OpenAPI
  - `--index-out <file>` to emit a lightweight `paths-index.json` (path/method/summary/operationId)

## Serving / viewing
- Redoc HTML lives at `docs/api/index.html` and references the generated spec.
- `make openapi-serve` runs `http-server` in the foreground on port 8080.
- `make openapi-serve-start` / `make openapi-serve-stop` manage a background server (PID stored in `/tmp/pve-openapi-serve.pid`).

## Validation
- The generator normalizes unspecified/null returns to a nullable object and maps file-download endpoints to `application/octet-stream`.
- The generated spec currently passes `swagger-cli validate docs/api/pve-openapi.yaml`.

## Typical workflow when Proxmox updates
1. Run `make gen-openapi` (fetches latest apidoc.js).
2. Optionally scope: `go run ./cmd/pve-openapi-gen --include-prefix=/nodes -out docs/api/pve-openapi.yaml` for a slimmer slice.
3. Preview with `make openapi-serve` and browse at http://localhost:8080.
4. Commit updated `docs/api/pve-openapi.yaml` if desired.

## Notes / limitations
- Upstream apidoc.js is unversioned; rerun on each Proxmox upgrade to stay in sync.
- Some response schemas are generic when apidoc lacks detail. Adjust the generator if you need stricter shapes.
- Licensing: Proxmox VE docs (including apidoc.js) are AGPLv3; the generated spec is a derivative. Keep it for internal use or comply with AGPL sharing requirements if you distribute it.
