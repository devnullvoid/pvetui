# Proxmox Mock API

This tool provides a mock API server based on the generated OpenAPI spec for Proxmox VE. It supports stateful interactions for core resources and generic mock responses for other endpoints.

## Usage

First, ensure you have generated the OpenAPI spec:

```bash
make gen-openapi
```

Then run the mock server:

```bash
go run ./cmd/pve-mock-api -spec docs/api/pve-openapi.yaml -port 8080
```

## Features

### Stateful Mocking
The server maintains an in-memory state for:
- **Nodes**: Basic stats (CPU, Memory, Disk) and detailed status.
- **VMs/CTs**: Status (running/stopped), configuration, and resources.
- **Storage**: Capacity and status.

Supported stateful operations:
- **Listing Resources**: `GET /cluster/resources` returns the current state.
- **Cluster Status**: `GET /cluster/status` returns node information.
- **Node Status**: `GET /nodes/{node}/status` returns detailed node metrics.
- **VM Status**: `GET /nodes/{node}/{type}/{vmid}/status/current`.
- **VM Config**: `GET/POST/PUT /nodes/{node}/{type}/{vmid}/config` to read/update configuration.
- **VM Actions**: `POST /nodes/{node}/{type}/{vmid}/status/{action}` (start, stop, shutdown, reboot).
- **VM Deletion**: `DELETE /nodes/{node}/{type}/{vmid}`.

### Generic Mocking
For any endpoint not explicitly handled by the stateful logic, the server uses the OpenAPI spec to generate a valid mock response based on the defined schema.
- **Route Matching**: Uses `kin-openapi` router.
- **Response Generation**: Generates dummy data (e.g. "mock_string", 1, true) matching the response schema.

## Limitations

- State is in-memory only and resets on restart.
- "Generic" responses are static and randomized/default values.
- Task handling (`UPID`) returns success but does not spawn actual background tasks.
