---
name: pvetui-cli
description: Use when querying or managing a Proxmox VE cluster via the pvetui CLI — listing nodes, guests, and tasks; creating and migrating VMs and LXC containers; managing storage content, downloading templates and OCI images, restoring backups, and installing Proxmox Community Scripts when the plugin is enabled. Requires pvetui to be installed and configured.
license: MIT
metadata:
  author: github.com/devnullvoid/pvetui
  version: "1.2"
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
- Creating VMs and LXC containers
- Migrating guests between nodes
- Executing commands inside a running guest without needing SSH to it
- Opening interactive shells on nodes or inside guests
- Listing and managing storage content (ISOs, templates, backups, disk images)
- Downloading ISOs, appliance templates, or OCI images into storage
- Restoring guests from vzdump backups
- Searching, inspecting, and installing Proxmox Community Scripts when `community-scripts` is enabled
- Listing recent cluster task history
- Scripting or automating Proxmox operations without root access to a node

## Prerequisites

- `pvetui` installed and on `$PATH`
- Config at `~/.config/pvetui/config.yml` with at least one profile
- API token or password auth configured in the profile
- For `guests exec` / `guests shell` on LXC containers: `ssh_user` configured in the profile
- For `guests shell` on QEMU VMs: `ssh_user` (and optionally `vm_ssh_user`) configured
- For `community-scripts install`: `plugins.enabled` includes `community-scripts`, and node SSH settings are configured

## Quick Reference

| Command | Purpose |
|---------|---------|
| `pvetui nodes list` | List all cluster nodes |
| `pvetui nodes show <node>` | Show details for one node |
| `pvetui nodes shell <node>` | Open an interactive SSH shell on a node |
| `pvetui guests list` | List all VMs and containers |
| `pvetui guests show <vmid>` | Show details for one guest |
| `pvetui guests start <vmid>` | Start a guest |
| `pvetui guests stop <vmid>` | Force stop a guest (power off) |
| `pvetui guests shutdown <vmid>` | Graceful ACPI shutdown |
| `pvetui guests restart <vmid>` | Graceful restart |
| `pvetui guests delete <vmid>` | Permanently delete a guest and its disks |
| `pvetui guests exec <vmid> <cmd>` | Run a command inside a guest |
| `pvetui guests shell <vmid>` | Open an interactive shell inside a guest |
| `pvetui guests create vm` | Create a QEMU VM |
| `pvetui guests create lxc` | Create an LXC container |
| `pvetui guests migrate <vmid> <node>` | Migrate a guest to another node |
| `pvetui tasks list` | List recent cluster tasks |
| `pvetui storage list` | List storages across the cluster |
| `pvetui storage show <node> <storage>` | Show details for a storage |
| `pvetui storage content list <node> <storage>` | List content in a storage |
| `pvetui storage content delete <node> <storage> <volid>` | Delete a content item |
| `pvetui storage download url <node> <storage> <url>` | Download from a URL |
| `pvetui storage download template <node> <storage> <template>` | Download an appliance template |
| `pvetui storage download oci <node> <storage> <reference>` | Pull an OCI image |
| `pvetui storage restore <node> <storage> <volid> <vmid>` | Restore a guest from backup |
| `pvetui community-scripts search <query>` | Search available Proxmox Community Scripts |
| `pvetui community-scripts show <slug-or-name>` | Show Community Script metadata |
| `pvetui community-scripts plan <slug-or-name> --node <node>` | Preview a Community Script install command |
| `pvetui community-scripts install <slug-or-name> --node <node>` | Install a Community Script on a node over SSH |
| `pvetui community-scripts deploy <slug-or-name> --node <node>` | Alias for `install`, useful for agent deployment wording |

## Global Flags

These flags work with every subcommand:

| Flag | Short | Description |
|------|-------|-------------|
| `--profile <name>` | `-p` | Use a specific connection profile or aggregate group |
| `--output <format>` | `-o` | Output format: `json` or `table`; overrides `cli.default_output` |
| `--config <path>` | `-c` | Path to config file (default: `~/.config/pvetui/config.yml`) |
| `--no-cache` | `-n` | Disable caching |

## Output Format

**Default (JSON unless configured):** structured output to stdout; errors as JSON to stderr; non-zero exit on failure.

**Table (`--output table`):** aligned human-readable output to stdout.

Set `cli.default_output: table` in config or `PVETUI_CLI_DEFAULT_OUTPUT=table` to make table output the default. Prefer `--output json` explicitly when parsing output in scripts or agents, because users may configure table as their default.

## Task-Producing Commands

Commands that trigger a Proxmox task (`guests create`, `guests migrate`, `storage content delete`, `storage download *`, `storage restore`) block until the task completes by default, then include `status` and `exit_status` in the output. Pass `--no-wait` to return the task UPID immediately without waiting.

```json
{
  "vmid": 105,
  "node": "pve01",
  "upid": "UPID:pve01:...",
  "status": "complete",
  "exit_status": "OK"
}
```

A non-`"OK"` `exit_status` causes a non-zero process exit.

## Community Scripts

The `community-scripts` command group is available only when the opt-in plugin is enabled:

```yaml
plugins:
  enabled:
    - community-scripts
```

Use `search` before installing and `show` to inspect the upstream metadata, source repo, script path, and documentation links.

```bash
# Search by name, slug, or description
pvetui community-scripts search nextcloud
pvetui community-scripts search docker --output table

# Show one script by exact slug or name
pvetui community-scripts show nextcloud

# Preview and then deploy with validated var_* overrides
pvetui community-scripts plan grafana --node pve01 --yes \
  --set var_hostname=grafana --set var_cpu=2 --set var_ram=2048 \
  --set var_container_storage=local-lvm --set var_template_storage=local

pvetui community-scripts deploy grafana --node pve01 --yes \
  --set var_hostname=grafana --set var_brg=vmbr0 --set var_net=dhcp \
  --set var_cpu=2 --set var_ram=2048 --set var_disk=8 \
  --set var_ssh=yes --set 'var_tags=monitoring;grafana' \
  --set var_container_storage=local-lvm --set var_template_storage=local
```

`install`/`deploy` SSHes to the selected node and runs the same Community Scripts installer flow as the TUI. It resolves SSH settings from the node source profile, active profile, or global `ssh_user`; set `--ssh-user` when needed. Upstream installer output is streamed to stderr so stdout can contain the final JSON/table result.

For agent-driven deployments, prefer `plan` first, then `deploy --yes` with explicit `--set var_*=value` overrides. For unattended LXC deploys, include `var_container_storage` and `var_template_storage`; otherwise upstream scripts can open a storage picker and fail without a TTY. `var_container_storage` selects the LXC rootfs storage, `var_template_storage` selects where the OS template is downloaded, `var_disk` is the LXC rootfs size in GB, `var_brg` selects the bridge, `var_net=dhcp` requests DHCP, and `var_vlan` sets the Proxmox `net0` VLAN tag. When both storage overrides are present, pvetui temporarily seeds `/usr/local/community-scripts/default.vars` on the target node so upstream first-run defaults do not prompt; the file is restored if it existed and removed if pvetui created it. `--yes` disables TTY allocation, selects the upstream default preset, skips the Community Scripts host-update prompt, and feeds empty stdin so unexpected prompts fail instead of hanging. Supported overrides are the Community Scripts allowlisted variables, including `var_hostname`, `var_cpu`, `var_ram`, `var_disk`, `var_brg`, `var_net`, `var_gateway`, `var_vlan`, `var_container_storage`, `var_template_storage`, `var_ssh`, `var_tags`, `var_unprivileged`, `var_nesting`, `var_fuse`, `var_tun`, and related `var_*` settings.

## Nodes

```bash
# List all nodes (JSON)
pvetui nodes list

# List nodes in human-readable table
pvetui nodes list --output table

# Show a specific node
pvetui nodes show pve01
pvetui nodes show pve01 --output table

# Open an interactive SSH shell on a node
pvetui nodes shell pve01
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

### Guest Delete

Permanently deletes a guest and all its associated disks. The guest must be stopped first unless `--force` is passed.

```bash
# Delete a stopped guest (waits for completion)
pvetui guests delete 108

# Also remove the VMID from backup and replication job configs
pvetui guests delete 108 --purge

# Force-delete a running guest (data loss risk)
pvetui guests delete 108 --force

# Return the task UPID immediately without waiting
pvetui guests delete 108 --no-wait
```

**JSON shape — delete:**
```json
{
  "vmid": 108,
  "node": "pve01",
  "upid": "UPID:pve01:...",
  "status": "complete",
  "exit_status": "OK"
}
```

### Guest Create

```bash
# Create a VM (VMID auto-assigned if omitted)
pvetui guests create vm \
  --node pve01 \
  --name myvm \
  --disk-storage local-zfs \
  --disk-size 32

# With full options
pvetui guests create vm \
  --node pve01 \
  --name myvm \
  --vmid 105 \
  --memory 4096 \
  --cores 4 \
  --disk-storage local-zfs \
  --disk-size 32 \
  --iso local:iso/debian-12.iso \
  --bridge vmbr0 \
  --start

# Create an LXC container (package name resolved to latest template automatically)
pvetui guests create lxc \
  --node pve01 \
  --hostname myct \
  --rootfs-storage local-zfs \
  --template debian-12-standard

# LXC with full options
pvetui guests create lxc \
  --node pve01 \
  --hostname myct \
  --rootfs-storage local-zfs \
  --template debian-12-standard \
  --memory 1024 \
  --swap 512 \
  --cores 2 \
  --rootfs-size 16 \
  --bridge vmbr0 \
  --start

# Return the task UPID immediately without waiting
pvetui guests create vm --node pve01 --name myvm --disk-storage local-zfs --disk-size 32 --no-wait
```

**Key flags — `guests create vm`:**

| Flag | Required | Default | Notes |
|------|----------|---------|-------|
| `--node` | yes | — | Target node |
| `--name` | yes | — | VM name |
| `--disk-storage` | yes | — | Storage for the boot disk |
| `--disk-size` | yes* | — | Disk size in GB; required unless `--import-from` |
| `--vmid` | no | auto | Auto-assigned via GetNextID if omitted |
| `--memory` | no | 2048 | MB |
| `--cores` | no | 2 | vCPU cores |
| `--iso` | no | — | ISO volid for CD-ROM |
| `--bridge` | no | vmbr0 | Network bridge |
| `--start` | no | false | Start after create |
| `--import-from` | no | — | Import disk from volid instead of creating |
| `--no-wait` | no | false | Return UPID immediately |

**Key flags — `guests create lxc`:**

| Flag | Required | Default | Notes |
|------|----------|---------|-------|
| `--node` | yes | — | Target node |
| `--hostname` | yes | — | Container hostname |
| `--rootfs-storage` | yes | — | Storage for root filesystem |
| `--template` | yes | — | Full filename or package name (e.g. `debian-12-standard`) |
| `--vmid` | no | auto | |
| `--memory` | no | 512 | MB |
| `--swap` | no | 512 | MB (0 = no swap) |
| `--cores` | no | 1 | |
| `--rootfs-size` | no | 8 | GB |
| `--bridge` | no | vmbr0 | |
| `--unprivileged` | no | true | |
| `--nesting` | no | true | Enable Docker/nested containers |
| `--start` | no | false | Start after create |
| `--no-wait` | no | false | Return UPID immediately |

**JSON shape — guests create:**
```json
{
  "vmid": 105,
  "name": "myvm",
  "node": "pve01",
  "type": "qemu",
  "upid": "UPID:pve01:...",
  "status": "complete",
  "exit_status": "OK"
}
```

### Guest Migrate

```bash
# Migrate a guest (mode selected automatically)
pvetui guests migrate 100 pve02

# Force online/offline mode for QEMU (not valid for LXC)
pvetui guests migrate 100 pve02 --online
pvetui guests migrate 100 pve02 --offline

# Return task UPID immediately
pvetui guests migrate 100 pve02 --no-wait
```

Migration mode is selected automatically:
- QEMU running → online migration
- QEMU stopped → offline migration
- LXC (any state) → restart migration

**JSON shape — guests migrate:**
```json
{
  "vmid": 100,
  "name": "web-server",
  "source_node": "pve01",
  "target_node": "pve02",
  "mode": "online",
  "upid": "UPID:pve01:...",
  "status": "complete",
  "exit_status": "OK"
}
```

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

A non-zero `exit_code` means the command ran but failed. A transport/SSH error causes a non-zero process exit with a JSON error on stderr instead.

### Guest and Node Shell

Opens an interactive terminal session. These commands take over stdin/stdout and are not suitable for scripting — use `exec` for non-interactive commands in agents.

```bash
pvetui nodes shell pve01          # SSH to a Proxmox node
pvetui guests shell 100           # shell inside a guest (LXC: pct enter; QEMU: SSH to VM IP)
```

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

## Storage

```bash
# List all storages (one row per node/storage pair)
pvetui storage list
pvetui storage list --node pve01
pvetui storage list --output table

# Show a specific storage
pvetui storage show pve01 local-zfs
```

**JSON shape — storage list:**
```json
[
  {
    "name": "local-zfs",
    "node": "pve01",
    "type": "zfspool",
    "content": "images,rootdir",
    "used": 53687091200,
    "total": 214748364800,
    "active": true
  }
]
```

### Storage Content

```bash
# List all content in a storage
pvetui storage content list pve01 local

# Filter by content type: iso, vztmpl, backup, snippets, images
pvetui storage content list pve01 local --type iso
pvetui storage content list pve01 local --type backup

# Delete a content item (explicit volid required)
pvetui storage content delete pve01 local local:iso/old-debian.iso
pvetui storage content delete pve01 local local:iso/old-debian.iso --no-wait
```

**JSON shape — storage content list:**
```json
[
  {
    "volid": "local:iso/debian-12.iso",
    "name": "debian-12.iso",
    "type": "iso",
    "size": 658505728,
    "ctime": 1700000000,
    "vmid": 0
  }
]
```

### Storage Download

```bash
# Download an ISO from a URL (content type inferred from extension)
pvetui storage download url pve01 local https://example.com/debian-12.iso

# Override filename or content type
pvetui storage download url pve01 local https://example.com/image.bin \
  --filename my-image.iso \
  --content-type iso

# Download an appliance template by package name (latest version resolved automatically)
pvetui storage download template pve01 local debian-12-standard

# Or use a full filename to pin a specific version
pvetui storage download template pve01 local debian-12-standard_12.7-1_amd64.tar.zst

# Narrow resolution by section when multiple templates match
pvetui storage download template pve01 local myapp --section turnkeylinux

# Pull an OCI image
pvetui storage download oci pve01 local registry.example.com/myimage:latest

# Any download can return UPID immediately
pvetui storage download url pve01 local https://example.com/debian-12.iso --no-wait
```

**Content type inference from URL extension:**

| Extension | Inferred type |
|-----------|--------------|
| `.iso` | `iso` |
| `.tar.*` (`.tar.zst`, `.tar.gz`, etc.) | `vztmpl` |
| `.img` | `import` |
| other | must supply `--content-type` |

**JSON shape — storage download:**
```json
{
  "node": "pve01",
  "storage": "local",
  "url": "https://example.com/debian-12.iso",
  "filename": "debian-12.iso",
  "upid": "UPID:pve01:...",
  "status": "complete",
  "exit_status": "OK"
}
```

### Storage Restore

Restore a guest from a vzdump backup. **Destructive** — overwrites the target VMID's config and disks.

```bash
# Dry-run: print what would be restored, exit without making changes
pvetui storage restore pve01 local local:backup/vzdump-qemu-100-2024.tar.zst 100

# Actually restore (requires --confirm)
pvetui storage restore pve01 local local:backup/vzdump-qemu-100-2024.tar.zst 100 --confirm

# Override inferred guest type (normally inferred from volid prefix)
pvetui storage restore pve01 local local:backup/unknown.tar.zst 101 --confirm --type lxc

# Return UPID immediately
pvetui storage restore pve01 local local:backup/vzdump-qemu-100.tar.zst 100 --confirm --no-wait
```

Guest type is inferred from the volid prefix: `vzdump-qemu-*` → `qemu`, `vzdump-lxc-*` → `lxc`. Use `--type` to override when inference fails.

**JSON shape — storage restore:**
```json
{
  "node": "pve01",
  "storage": "local",
  "volid": "local:backup/vzdump-qemu-100-2024.tar.zst",
  "vmid": 100,
  "type": "qemu",
  "upid": "UPID:pve01:...",
  "status": "complete",
  "exit_status": "OK"
}
```

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

In aggregate group mode, `source_profile` in JSON output identifies which cluster each resource belongs to. All reads fan out concurrently; write operations (lifecycle, exec, create, migrate) are routed to the correct cluster automatically.

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

# Create an LXC and wait for it to finish, then get its VMID
result=$(pvetui guests create lxc --node pve01 --hostname myct --rootfs-storage local-zfs --template debian-12-standard)
vmid=$(echo "$result" | jq -r '.vmid')

# Migrate a guest and verify it landed on the target
pvetui guests migrate 100 pve02
pvetui guests show 100 | jq -r '.node'   # should now be "pve02"

# Download a template and create a container from it in sequence
pvetui storage download template pve01 local debian-12-standard
pvetui guests create lxc --node pve01 --hostname newct --rootfs-storage local-zfs --template debian-12-standard --start

# List all ISO files available on a node
pvetui storage content list pve01 local --type iso | jq '.[].volid'

# Find all backup files for a specific VM
pvetui storage content list pve01 local --type backup | jq '[.[] | select(.volid | contains("vzdump-qemu-100-"))]'

# Run a health check across multiple guests
for vmid in 100 101 102; do
  echo "=== $vmid ==="
  pvetui guests exec $vmid "systemctl is-active nginx"
done

# Find guests with a specific tag
pvetui guests list | jq '[.[] | select(.tags | contains("prod"))]'

# Get total memory across all nodes
pvetui nodes list | jq '[.[].memory_total] | add'

# Check storage usage across the cluster
pvetui storage list | jq '[.[] | {name, node, used_pct: (.used / .total * 100 | round)}]'
```

## Troubleshooting

| Problem | Likely Cause | Fix |
|---------|-------------|-----|
| `{"error": "bootstrap failed: ..."}` | Bad config or credentials | Check `~/.config/pvetui/config.yml`; verify API token |
| `{"error": "guest agent is not available"}` | Agent not running in VM | Install/start `qemu-guest-agent` inside the VM |
| `{"error": "exec failed on LXC ..."}` | SSH to node failed | Verify `ssh_user` is set and SSH key auth works to the node |
| `{"error": "multiple profiles configured; use --profile"}` | No default profile set | Pass `--profile <name>` or set `default_profile` in config |
| `{"error": "failed to delete guest ... locked"}` | Guest is running or locked | Stop the guest first, or pass `--force` |
| `{"error": "cannot infer guest type from volid ..."}` | Backup filename not standard | Pass `--type qemu` or `--type lxc` explicitly |
| `{"error": "cannot infer content type from URL ..."}` | URL extension not recognised | Pass `--content-type iso` (or `vztmpl`/`import`) |
| `ip` field empty in guest list | Agent not running or no IP yet | Check guest agent; `ip` populates only for running QEMU VMs with agent |
| LXC exec fails with permission denied | Non-root user needs sudo | Add `NOPASSWD: /usr/sbin/pct exec *` to sudoers on node |
| Template name not resolved | Package not in aplinfo catalog | Use the full filename from `pvetui storage content list` |
