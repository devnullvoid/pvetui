# pve-openapi-gen

Generate an OpenAPI 3.0 spec from Proxmox VE's `apidoc.js` (the file used by the official API viewer).

## Install (no repo clone needed)

```bash
# latest from the repo
go install github.com/devnullvoid/pvetui/cmd/pve-openapi-gen@latest

# or pin to a commit/tag
# go install github.com/devnullvoid/pvetui/cmd/pve-openapi-gen@<commit>
```

## Quick use

```bash
pve-openapi-gen \
  -out pve-openapi.yaml \
  -version "pve-9.x"   # label in info.version
```

Defaults:
- Fetches `https://pve.proxmox.com/pve-docs/api-viewer/apidoc.js`.
- Writes `docs/api/pve-openapi.yaml` if `-out` not given.

## Options

- `--use-local --in <file>`: Use a local `apidoc.js` instead of downloading.
- `--url <custom>`: Point at another apidoc.js location.
- `--include-prefix=/nodes,/cluster`: Only include matching path prefixes (handy for smaller slices or agent ingestion).
- `-version <label>`: Sets `info.version` in the output.
- `--source-version <label>`: Adds a human note about the source PVE version (e.g., `PVE 9.1`)
- `--out-json <file>`: Also emit JSON OpenAPI.
- `--index-out <file>`: Emit a lightweight paths index (`path`, `method`, `summary`, `operationId`) for fast lookup/agents.

## Viewing (without the repo)

- Redoc (recommended):
  ```bash
  npx redoc-cli serve pve-openapi.yaml --port 8080
  ```
  Open http://localhost:8080

- Swagger UI (watcher):
  ```bash
  npx swagger-ui-watcher pve-openapi.yaml --port 8080
  ```

- Online editors: import the file/URL into https://editor.swagger.io.

## Notes

- Upstream `apidoc.js` is unversioned—regenerate after upgrading Proxmox.
- Unknown/null responses are normalized to a nullable object; file download endpoints are mapped to `application/octet-stream`.
- Licensing: `apidoc.js` ships with Proxmox VE docs (AGPLv3). Generated specs are derivative; keep them for internal use or comply with AGPL sharing requirements.
