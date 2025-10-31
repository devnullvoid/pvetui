# VNC Integration with noVNC

This directory contains the VNC integration for pvetui, including an embedded noVNC client.

## noVNC Subtree

The noVNC client is vendored via a git subtree rooted at [`internal/vnc/novnc/`](https://github.com/novnc/noVNC) and embedded directly into the compiled binary using Go's `embed` directive. No extra Git configuration is required when cloning the repository.

### Current Source

- **Upstream repository**: https://github.com/novnc/noVNC.git
- **Location in tree**: `internal/vnc/novnc/`
- **Embedding**: Files are embedded at compile time using `//go:embed novnc`

### Updating the Subtree

The preferred workflow is:

```bash
make update-novnc
```

This target runs `git subtree pull --prefix=internal/vnc/novnc https://github.com/novnc/noVNC.git master --squash`, then executes `scripts/prune_novnc.sh` to drop development-only assets such as tests, docs, npm tooling, and upstream workflow files.

If you need to update manually:

1. Run the `git subtree pull` command above.
2. Execute `./scripts/prune_novnc.sh` to remove non-runtime files.
3. Stage the cleaned subtree: `git add internal/vnc/novnc`.

### Checking the Embedded Version

Because the subtree history is squashed, pvetui tracks the upstream commit in the merge message. To see the latest synced commit:

```bash
git log --oneline --grep="Squashed 'internal/vnc/novnc/'" | head -n 1
```

For a deeper diff against upstream, split the subtree onto a temporary branch and compare:

```bash
git subtree split --prefix=internal/vnc/novnc -b novnc-split
git log -1 novnc-split
# ...perform comparisons...
git branch -D novnc-split
```

### Integration Details

The noVNC client files are embedded directly into the compiled binary using Go's `embed` directive (`//go:embed novnc`). This means:

- **Self-Contained**: The binary includes all noVNC files and can run without external dependencies
- **No Runtime Filesystem Access**: Files are served from memory, not from disk
- **Deployment Simplicity**: Only the binary needs to be distributed

The VNC server (`internal/vnc/server.go`) serves the embedded files using Go's `http.FS` with the embedded filesystem.

### Benefits of Vendoring via Subtree

1. **Zero extra setup**: Contributors clone the repository without special flags.
2. **Reproducible builds**: All required runtime assets live in-tree and are embedded during compilation.
3. **Controlled footprint**: `scripts/prune_novnc.sh` keeps only the assets needed at runtime.
4. **Straightforward updates**: A single Make target manages pulling, pruning, and staging newer upstream versions.
5. **Upstream traceability**: Subtree commits retain upstream history in merge messages for auditability.

### Testing

Run tests to verify the embedding works correctly:

```bash
go test -v ./internal/vnc -run TestNoVNC
```
