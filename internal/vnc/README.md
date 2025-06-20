# VNC Integration with noVNC

This directory contains the VNC integration for Proxmox TUI, including an embedded noVNC client.

## noVNC Submodule

The noVNC client is included as a git submodule from the official [noVNC repository](https://github.com/novnc/noVNC).

### Current Version

- **noVNC Version**: v1.6.0 (latest stable release)
- **Location**: `internal/vnc/novnc/`
- **Repository**: https://github.com/novnc/noVNC.git

### Working with the Submodule

#### Initial Setup (for new clones)

When cloning the repository, initialize the submodule:

```bash
git submodule update --init --recursive
```

#### Updating noVNC to a New Version

To update to a newer version of noVNC:

1. Navigate to the submodule directory:
   ```bash
   cd internal/vnc/novnc
   ```

2. Fetch the latest tags:
   ```bash
   git fetch --tags
   ```

3. Check available versions:
   ```bash
   git tag | grep "^v" | sort -V | tail -10
   ```

4. Checkout the desired version (e.g., v1.7.0):
   ```bash
   git checkout v1.7.0
   ```

5. Return to the project root and commit the submodule update:
   ```bash
   cd ../../..
   git add internal/vnc/novnc
   git commit -m "feat: update noVNC to v1.7.0"
   ```

#### Checking Current Version

To see which version is currently checked out:

```bash
cd internal/vnc/novnc
git describe --tags
```

### Integration Details

The noVNC client is served directly from the filesystem (not embedded) to allow for easy updates via the submodule. The VNC server (`internal/vnc/server.go`) serves files from the `internal/vnc/novnc` directory.

### Benefits of Using Submodules

1. **Easy Updates**: Update to new noVNC versions with simple git commands
2. **Version Control**: Track exactly which version of noVNC is being used
3. **Upstream Tracking**: Stay connected to the official noVNC repository
4. **No Manual Copying**: No need to manually download and copy files
5. **Reproducible Builds**: Anyone cloning the repository gets the exact same noVNC version

### Migration from Manual Copy

This setup replaces the previous manual copy approach where noVNC files were manually downloaded and copied into the repository. The submodule approach provides better maintainability and version tracking. 