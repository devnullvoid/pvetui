---
name: pvetui-cli
description: Use when querying or managing a Proxmox VE cluster via the pvetui CLI — listing nodes, guests, and tasks; starting/stopping/restarting guests; executing commands inside VMs (QEMU guest agent) or LXC containers (pct exec over SSH). Requires pvetui to be installed and configured.
license: MIT
metadata:
  author: github.com/devnullvoid/pvetui
  version: "1.0"
---

# pvetui CLI

`pvetui` is a terminal UI and CLI for Proxmox VE. When invoked with a subcommand it runs non-interactively, making it suitable for scripts and AI agent workflows.

## Installation

```bash
# Via Go (recommended)
go install github.com/devnullvoid/pvetui/cmd/pvetui@latest

# Via skill (Claude Code / any skills.sh-compatible agent)
npx skills add devnullvoid/pvetui
```

## When to Use

- Querying node status, resource usage, or uptime
- Listing, filtering, or inspecting VMs and LXC containers
- Starting, stopping, shutting down, or restarting guests
- Executing commands inside a running guest without needing SSH to it
- Listing recent cluster task history
- Scripting or automating Proxmox operations without root access to a node

**Not for:** Storage management, VM creation, networking changes, or anything requiring the Proxmox web UI — use the `proxmox-admin` skill (requires node SSH/root access) for those.

## Prerequisites

- `pvetui` installed and on `$PATH`
- Config at `~/.config/pvetui/config.yml` with at least one profile
- API token or password auth configured in the profile
- For `guests exec` on LXC containers: `ssh_user` configured in the profile

## Quick Reference

| Command | Purpose |
|---------|---------|
| `pvetui nodes list` | List all cluster nodes |
| `pvetui nodes show <node>` | Show details for one node |
| `pvetui guests list` | List all VMs and containers |
| `pvetui guests show <vmid>` | Show details for one guest |
| `pvetui guests start <vmid>` | Start a guest |
| `pvetui guests stop <vmid>` | Force stop a guest (power off) |
| `pvetui guests shutdown <vmid>` | Graceful ACPI shutdown |
| `pvetui guests restart <vmid>` | Graceful restart |
| `pvetui guests exec <vmid> <cmd>` | Run a command inside a guest |
| `pvetui tasks list` | List recent cluster tasks |

## Global Flags

These flags work with every subcommand:

| Flag | Short | Description |
|------|-------|-------------|
| `--profile <name>` | `-p` | Use a specific connection profile or aggregate group |
| `--output <format>` | `-o` | Output format: `json` (default) or `table` |
| `--config <path>` | `-c` | Path to config file (default: `~/.config/pvetui/config.yml`) |
| `--no-cache` | `-n` | Disable caching |

## Output Format

**Default (JSON):** structured output to stdout; errors as JSON to stderr; non-zero exit on failure.

**Table (`--output table`):** aligned human-readable output to stdout.

All commands default to JSON — prefer JSON when parsing output in scripts or agents.

## Nodes

```bash
# List all nodes (JSON)
pvetui nodes list

# List nodes in human-readable table
pvetui nodes list --output table

# Show a specific node
pvetui nodes show pve01
pvetui nodes show pve01 --output table
```

**JSON shape — nodes list:**
```json
[
  {
    "name": "pve01",
    "ip": "10.0.0.10",
    "online": true,
    "cpu_usage": 0.12,
    "memory_used": 7.238216400146484,
    "memory_total": 31.174354553222656,
    "uptime": 864000,
    "version": "8.2.2",
    "kernel_version": "6.8.4-2-pve",
    "source_profile": "prod"
  }
]
```

`cpu_usage` is a fraction (0.0–1.0). Memory values are in GiB. `uptime` is seconds. `source_profile` is populated in aggregate group mode.

## Guests (VMs and Containers)

```bash
# List all guests
pvetui guests list

# Filter by node, status, or type
pvetui guests list --node pve01
pvetui guests list --status running
pvetui guests list --type qemu
pvetui guests list --node pve01 --status running --type lxc

# Show a specific guest by VMID
pvetui guests show 100
```

**JSON shape — guests list:**
```json
[
  {
    "id": 100,
    "name": "web-server",
    "node": "pve01",
    "type": "qemu",
    "status": "running",
    "ip": "10.0.0.100",
    "template": false,
    "tags": "prod;web",
    "source_profile": "prod"
  }
]
```

`type` is `"qemu"` or `"lxc"`. `status` is `"running"`, `"stopped"`, or `"paused"`. `ip` may be empty if the guest agent is not running. `template: true` guests cannot be started/stopped.

### Lifecycle Operations

```bash
pvetui guests start 100       # start a stopped guest
pvetui guests shutdown 100    # graceful ACPI shutdown (preferred)
pvetui guests stop 100        # force power-off (data loss risk)
pvetui guests restart 100     # graceful restart
```

**JSON shape — lifecycle:**
```json
{
  "vmid": 100,
  "operation": "shutdown",
  "upid": "UPID:pve01:00001234:...",
  "node": "pve01"
}
```

The `upid` is a Proxmox task ID. Use `pvetui tasks list` to monitor task completion.

## Guest Exec

Execute a command inside a running guest without SSH access to the guest itself.

```bash
# QEMU VM — uses the QEMU guest agent (API only, no node SSH needed)
pvetui guests exec 100 "uptime"
pvetui guests exec 100 "df -h"
pvetui guests exec 100 "systemctl status nginx"

# LXC container — uses pct exec over SSH to the node (ssh_user must be configured)
pvetui guests exec 200 "uptime"
pvetui guests exec 200 "cat /etc/os-release"

# Custom timeout (default 30s)
pvetui guests exec 100 "apt-get update" --timeout 120s

# Table output (useful for human review)
pvetui guests exec 100 "df -h" --output table
```

**Requirements:**
- QEMU: guest must be running with the QEMU guest agent enabled and responding.
- LXC: guest must be running; `ssh_user` must be configured in the profile; the agent user needs permission to run `pct exec` (or be root).

**OS detection (QEMU only):**
- Windows guests (`ostype` starting with `win`): command is wrapped in `powershell.exe -NoProfile -NonInteractive -ExecutionPolicy Bypass -Command <cmd>`
- All other guests: command is wrapped in `/bin/sh -c <cmd>`
- LXC always uses `/bin/sh -c <cmd>`

**JSON shape — exec:**
```json
{
  "vmid": 100,
  "command": "uptime",
  "stdout": " 10:23:01 up 10 days,  2:34,  0 users,  load average: 0.10, 0.08, 0.05\n",
  "stderr": "",
  "exit_code": 0,
  "duration_ms": 412
}
```

A non-zero `exit_code` means the command ran but failed (e.g., the program returned non-zero). A transport/SSH error causes a non-zero process exit with a JSON error on stderr instead.

## Tasks

```bash
# Last 20 tasks (default)
pvetui tasks list

# Last 50 tasks
pvetui tasks list --recent 50

# Human-readable table
pvetui tasks list --output table
```

**JSON shape — tasks list:**
```json
[
  {
    "upid": "UPID:pve01:00001234:...",
    "node": "pve01",
    "type": "qmstart",
    "status": "OK",
    "user": "root@pam",
    "starttime": 1710000000,
    "endtime": 1710000005,
    "source_profile": "prod"
  }
]
```

`starttime` and `endtime` are Unix timestamps. `status` is `"OK"` on success or an error string on failure. A task with no `endtime` is still running.

## Profiles and Multi-Cluster

All subcommands respect `--profile`. The profile can be a single connection profile or an aggregate group name.

```bash
# Use the "prod" profile
pvetui --profile prod guests list

# Use the "all-clusters" aggregate group (fans out across all member nodes)
pvetui --profile all-clusters guests list --status running

# List available profiles
pvetui --list-profiles
```

In aggregate group mode, `source_profile` in JSON output identifies which cluster each resource belongs to. All reads fan out concurrently; write operations (lifecycle, exec) are routed to the correct cluster automatically.

## Error Handling

Errors are written as JSON to stderr and the process exits non-zero:

```json
{"error": "guest 999 not found"}
```

Check exit code to detect failure; parse stderr JSON for the message. Stdout is always valid JSON (or empty) on success.

```bash
# Example: check exit code in shell
if ! pvetui guests show 100 > /dev/null 2>&1; then
  echo "Guest not found"
fi

# Example: capture error message
err=$(pvetui guests show 999 2>&1 >/dev/null)
```

## Common Agent Patterns

```bash
# Find all running VMs on a specific node
pvetui guests list --node pve01 --status running --type qemu

# Get IP addresses of all running guests
pvetui guests list --status running | jq '.[].ip'

# Check if a specific guest is running
pvetui guests show 100 | jq -r '.status'

# Run a health check across multiple guests
for vmid in 100 101 102; do
  echo "=== $vmid ==="
  pvetui guests exec $vmid "systemctl is-active nginx"
done

# Find guests with a specific tag
pvetui guests list | jq '[.[] | select(.tags | contains("prod"))]'

# Get total memory across all nodes
pvetui nodes list | jq '[.[].memory_total] | add'

# Monitor a task until completion (poll tasks list)
pvetui tasks list --recent 5 | jq '.[] | select(.upid == "UPID:...")'
```

## Troubleshooting

| Problem | Likely Cause | Fix |
|---------|-------------|-----|
| `{"error": "bootstrap failed: ..."}` | Bad config or credentials | Check `~/.config/pvetui/config.yml`; verify API token |
| `{"error": "guest agent is not available"}` | Agent not running in VM | Install/start `qemu-guest-agent` inside the VM |
| `{"error": "exec failed on LXC ..."}` | SSH to node failed | Verify `ssh_user` is set and SSH key auth works to the node |
| `{"error": "multiple profiles configured; use --profile"}` | No default profile set | Pass `--profile <name>` or set `default_profile` in config |
| `ip` field empty in guest list | Agent not running or no IP yet | Check guest agent; `ip` populates only for running QEMU VMs with agent |
| LXC exec fails with permission denied | Non-root user needs sudo | Add `NOPASSWD: /usr/sbin/pct exec *` to sudoers on node |
