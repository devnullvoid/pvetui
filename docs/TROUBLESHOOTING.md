# Troubleshooting Guide

This document provides solutions to common issues encountered when installing and running pvetui.

## 🍎 macOS Issues

### Gatekeeper Blocking Binary Execution

**Problem**: When running the pre-built binary on macOS, you encounter errors such as:
- `zsh: killed ./pvetui-darwin-arm64`
- Binary exits with code 137 (SIGKILL)
- macOS shows security warnings about unsigned applications

**Cause**: macOS Gatekeeper blocks unsigned binaries downloaded from the internet by applying the `com.apple.quarantine` attribute.

**Solutions** (in order of recommendation):

#### Option 1: Remove Quarantine Attribute
```bash
xattr -d com.apple.quarantine ./pvetui-darwin-arm64
```

This removes the quarantine attribute that macOS applies to downloaded files, allowing the binary to run normally.

#### Option 2: Use Finder Override
1. Right-click the binary in Finder
2. Select "Open" from the context menu
3. Click "Open" in the security dialog that appears
4. The binary will run and be remembered as trusted

#### Option 3: Build from Source (Most Secure)
```bash
git clone https://github.com/devnullvoid/pvetui.git
cd pvetui
make install
```

Building from source avoids Gatekeeper issues entirely since the binary is compiled locally. The embedded noVNC client is vendored via git subtree, so no additional clone flags are required.

#### Option 4: Use Go Install from Source (Recommended for Developers)
```bash
git clone https://github.com/devnullvoid/pvetui.git
cd pvetui
make install-go
# or: go install ./cmd/pvetui
```

This method compiles the source code directly with the vendored noVNC client.

**Why This Happens**: The pre-built binaries are not code-signed with an Apple Developer certificate ($99/year requirement). Code signing would eliminate these warnings but requires an Apple Developer Program membership.

## 🐧 Linux Issues

### Permission Denied
If you encounter permission issues when running the binary:

```bash
chmod +x ./pvetui-linux-amd64
```

### Missing Dependencies
pvetui is statically compiled and should not require additional dependencies. If you encounter issues, ensure your system is up to date.

## 🪟 Windows Issues

### Windows Defender SmartScreen
Windows may show a SmartScreen warning for unsigned executables:

1. Click "More info" in the SmartScreen dialog
2. Click "Run anyway" to proceed
3. The executable will be remembered as trusted

### Antivirus False Positives
Some antivirus software may flag the binary as suspicious. This is common with unsigned executables. You can:

1. Add an exception for the binary in your antivirus software
2. Build from source using the Go toolchain
3. Use the source installation method (see Go Install Issues section above)

## 🔧 General Issues

### Go Install Issues

**Problem**: Running `go install github.com/devnullvoid/pvetui/cmd/pvetui@latest` fails with:
```
pattern novnc: no matching files found
```

**Cause**: Releases prior to October 2025 embedded noVNC via a git submodule. Current versions vendor the client with a git subtree, so this error usually means you're building an older tag or your checkout predates the subtree migration.

**Solutions**:
- Upgrade to the latest tag or main branch; `go install github.com/devnullvoid/pvetui/cmd/pvetui@latest` now works without extra flags.
- If you must build an older release, clone with `--recurse-submodules` to populate the legacy noVNC dependency.

### Binary Won't Start
1. **Check Architecture**: Ensure you downloaded the correct binary for your system:
   - `darwin-amd64`: Intel Macs
   - `darwin-arm64`: Apple Silicon Macs (M1/M2/M3)
   - `linux-amd64`: 64-bit Linux
   - `linux-arm64`: ARM64 Linux
   - `windows-amd64.exe`: 64-bit Windows

2. **Verify Download**: Re-download the binary if it seems corrupted
3. **Check Permissions**: Ensure the binary has execute permissions (Unix-like systems)

### Configuration Issues
If you encounter configuration-related problems:

1. **Use Config Wizard**: Run with `--config-wizard` flag for interactive setup
2. **Check Config Path**: Verify configuration file location with `--help`
3. **Validate YAML**: Ensure your configuration file has valid YAML syntax

### Connection Issues
For Proxmox connection problems:

1. **Test API Access**: Verify you can reach the Proxmox API from your machine
2. **Check Credentials**: Ensure your username, password, or API tokens are correct
3. **Verify SSL**: Use `--insecure` flag if testing with self-signed certificates (not recommended for production)

## 🆘 Getting Help

If you continue to experience issues:

1. **Check Existing Issues**: Search the [GitHub Issues](https://github.com/devnullvoid/pvetui/issues) for similar problems
2. **Create New Issue**: If your problem isn't covered, create a new issue with:
   - Your operating system and architecture
   - The exact error message
   - Steps to reproduce the issue
   - Your configuration (with sensitive data removed)

3. **Include System Information**:
   ```bash
   # For version information
   ./pvetui --version

   # For Go environment (if building from source)
   go version
   go env GOOS GOARCH
   ```

## 🔐 Security Considerations

### Code Signing Status
Currently, the pre-built binaries are **not code-signed**. This means:

- ✅ The source code is open and auditable
- ✅ You can build from source for maximum trust
- ❌ Pre-built binaries may trigger OS security warnings
- ❌ Some corporate environments may block unsigned binaries

### Recommended Approaches by Security Level

**Highest Security**: Build from source after reviewing the code
```bash
git clone https://github.com/devnullvoid/pvetui.git
cd pvetui
# Review the source code
make install
```

**High Security**: Use Go's built-in installation
```bash
go install github.com/devnullvoid/pvetui/cmd/pvetui@latest
```

**Standard Security**: Use pre-built binaries with OS override (as documented above)
