# Plugin Guide

This guide explains how to work with the pvetui plugin system, including enabling existing extensions and authoring new ones.

## Overview

- Plugins are discovered through static registration in `internal/ui/plugins/loader.go`.
- At startup pvetui loads the plugin identifiers listed under `plugins.enabled` in your configuration file.
- When `plugins.enabled` is omitted or empty, no optional functionality is activated.

## Enabling Built-in Plugins

The repository currently ships with two built-in plugins:

- `community-scripts`: exposes the community script installer from the node context menu
- `demo-guest-list`: adds a demo action that lists running guests for the selected node

```yaml
plugins:
  enabled:
    - "community-scripts"
    - "demo-guest-list"
```

Restart pvetui after editing the configuration to apply the change. If an unknown plugin ID is listed, the application prints a warning similar to `⚠️ Unknown plugins requested: my-plugin` during startup.

## Demo Guest List Plugin

The `demo-guest-list` plugin is intentionally small and serves as a reference implementation. When enabled it contributes a node context menu entry labelled **Show Running Guests (Demo)**. Selecting the action opens a modal listing the running guests on the chosen node, including their IDs, types, and discovered IP addresses when available.

## Writing a New Plugin

1. Implement the `components.Plugin` interface (see `internal/ui/components/plugins.go`):
   - `ID() string` must return a stable identifier used in configuration files.
   - `Name()` and `Description()` provide user-facing metadata.
   - `Initialize(ctx, app, registrar)` is called once at startup. Register UI contributions (for example node actions) through the provided `registrar`.
   - `Shutdown(ctx)` should release resources acquired during initialization.
2. Place the implementation in `internal/ui/plugins/<yourplugin>/` and expose a constructor (for example `func New() components.Plugin`).
3. Register the plugin in `internal/ui/plugins/loader.go` by adding an entry to the `registry` map.
4. Add unit tests in `internal/ui/plugins` that cover registration logic and any behaviour that can be exercised without the full TUI runtime.

Plugins may use the `components.App` helper methods passed to `Initialize` to access configuration, API clients, and UI primitives. Keep long-running work cancellable by respecting the provided `context.Context`.

## Testing Plugins

Run `go test ./internal/ui/plugins/...` to execute plugin-level unit tests. For end-to-end validation launch pvetui with a configuration that enables your plugin and verify the contributed UI pieces (such as context menu entries) appear and behave as expected.
