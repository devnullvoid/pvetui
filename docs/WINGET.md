# WinGet Packaging

Tracking issue: [#170](https://github.com/devnullvoid/pvetui/issues/170).

## Status

The initial candidate is `devnullvoid.pvetui` version `1.4.4`, using the existing
Windows x64 release ZIP as a portable installation. The test commands request
per-user scope with `--scope user`; the manifest must omit `Scope`, which native
WinGet validation rejects for portable installers. It has been
[submitted for review](https://github.com/microsoft/winget-pkgs/pull/474741) but is
not yet available from the WinGet community repository. ARM64/x86 and
release automation are intentionally deferred until the first package is tested.

The three manifests are under:

```text
packaging/winget/manifests/d/devnullvoid/pvetui/1.4.4/
```

The layout mirrors `microsoft/winget-pkgs`, so only that version directory needs
to be copied into an upstream checkout's `manifests/d/devnullvoid/pvetui/`.
Do not copy this guide or other pvetui repository files into the submission.

## Release Verification

The candidate references the version-specific release URL, not a
`latest` URL. The archive contains `pvetui.exe` at its root and includes the
project and third-party licenses. Its SHA-256 was checked against the published
`checksums_1.4.4.txt`:

```text
f6f893f55348413e5e26032ffcf6359dadbaeb62f4d2c806b90e7b912dae8b24
```

To repeat the download and inspection on Linux, use an empty temporary directory:

```bash
gh release download v1.4.4 -R devnullvoid/pvetui \
  -p pvetui_1.4.4_windows_amd64.zip -p checksums_1.4.4.txt
sha256sum pvetui_1.4.4_windows_amd64.zip
rg 'windows_amd64.zip' checksums_1.4.4.txt
unzip -l pvetui_1.4.4_windows_amd64.zip
```

Both checksum values must match `InstallerSha256`. The ZIP is downloaded by
WinGet during installation; it is not committed to this repository.

## Validation Without Windows

`make test` includes offline checks for manifest identity, version agreement,
file names, portable scope rules, executable alias, version-specific URLs, and hash format.
These checks do not download release assets or replace WinGet's own validator.

For JSON Schema validation, the following commands use an isolated tool installed
through [uv](https://docs.astral.sh/uv/). Run them from the repository root:

```bash
uvx --from check-jsonschema==0.37.0 check-jsonschema \
  --schemafile https://raw.githubusercontent.com/microsoft/winget-cli/master/schemas/JSON/manifests/v1.28.0/manifest.version.1.28.0.json \
  packaging/winget/manifests/d/devnullvoid/pvetui/1.4.4/devnullvoid.pvetui.yaml
uvx --from check-jsonschema==0.37.0 check-jsonschema \
  --schemafile https://raw.githubusercontent.com/microsoft/winget-cli/master/schemas/JSON/manifests/v1.28.0/manifest.installer.1.28.0.json \
  packaging/winget/manifests/d/devnullvoid/pvetui/1.4.4/devnullvoid.pvetui.installer.yaml
uvx --from check-jsonschema==0.37.0 check-jsonschema \
  --schemafile https://raw.githubusercontent.com/microsoft/winget-cli/master/schemas/JSON/manifests/v1.28.0/manifest.defaultLocale.1.28.0.json \
  packaging/winget/manifests/d/devnullvoid/pvetui/1.4.4/devnullvoid.pvetui.locale.en-US.yaml
```

## Windows Test Checklist

Prefer a disposable x64 Windows VM with WinGet and a terminal. An existing
Scoop/manual pvetui installation can shadow the WinGet alias; leave it intact
and distinguish the WinGet executable path from other installations when testing
on a shared host. No Proxmox credentials are needed for packaging checks. Do not
disable archive malware scanning or checksum verification to make a test pass.

Native validation passed without warnings on Windows x64 with WinGet
`v1.29.380`. This caught an unsupported portable `Scope` field that JSON Schema
validation did not reject; it has been removed. Per-user installation of the
real `1.4.3` release, upgrade to `1.4.4`, uninstall, and a fresh `1.4.4`
installation/uninstall passed on Windows `10.0.22631.0` (x64). Both versions
passed `--version` and `--help`, and the existing Scoop shim was unchanged.
Interactive TUI and fresh-terminal checks remain manual.

Before submission, the manifests were updated to the currently recommended
schema `1.28.0`. JSON Schema validation, native Windows validation, and a fresh
`1.4.4` install/version/help/uninstall cycle passed again with that schema.
The earlier real two-version upgrade used schema `1.12.0`; installer settings,
URLs, and checksums were unchanged by the schema update.

Two local-testing details matter:

- Before upstream publication, `winget list --id devnullvoid.pvetui --exact`
  did not match. `winget list --name pvetui --exact --scope user` displayed
  the local registration as `ARP\\User\\X64\\devnullvoid.pvetui__DefaultSource`.
  Verify its version and identity rather than treating the ID lookup as an
  installation failure.
- This non-elevated host used the supported package-directory `PATH` fallback
  rather than creating a symlink in `Microsoft\\WinGet\\Links`. After uninstall,
  WinGet removed the registration and files but left that directory on the user
  `PATH`. The test removed only its own stale entry afterward. Do not claim
  complete automatic PATH cleanup or remove unrelated PATH entries.

From a checkout of this repository, record `winget --version` and the Windows
version. Native `winget validate` works without enabling local installation.
For installation testing, enable local manifests in an elevated PowerShell
window only if permitted by the device's administrative policies:

```powershell
winget settings --enable LocalManifestFiles
```

This is an administrative setting, not a per-package option. Do not bypass a
managed restriction. Record its original state; if enabled temporarily for the
test, restore the disabled state with
`winget settings --disable LocalManifestFiles` afterward in an elevated window.

Then return to a normal, non-administrator PowerShell window at the repository
root. Install for the current user:

```powershell
$manifest = '.\packaging\winget\manifests\d\devnullvoid\pvetui\1.4.4'
winget validate --manifest $manifest
winget install --manifest $manifest --scope user --architecture x64 --silent --disable-interactivity --accept-source-agreements --accept-package-agreements
```

- [x] Native WinGet validation succeeds without warnings (WinGet `v1.29.380`, x64).
- [x] Installation completes unattended with installer hash verification.
- [ ] Open a new terminal; `Get-Command pvetui -All` resolves the installed alias.
- [x] The explicitly targeted WinGet executable reports `v1.4.4`; `--help` works.
- [x] Local name-based listing reports version `1.4.4`; public ID lookup is
      deferred until the package is available from the community source.
- [ ] With a dedicated test config (`pvetui --config <path>`), the TUI opens and
      exits normally. A test cluster may be used for read-only checks; do not
      make live guest changes merely to test packaging.
- [ ] SSH features find `ssh.exe` if tested. The package does not install the
      optional Windows OpenSSH client or configure credentials.
- [ ] User configuration and cache remain outside the WinGet package directory.

Record the paths and hashes of any test configuration files before uninstalling.
Never include their contents, API credentials, or SSH keys in test evidence.

```powershell
winget uninstall --id devnullvoid.pvetui --exact --scope user --silent --disable-interactivity
```

Before upstream publication, the ID may not resolve. For the local test only,
first verify the registration's `WinGetPackageIdentifier`, `InstallLocation`, and
`DisplayVersion`, then use its exact recorded product code:

```powershell
winget uninstall --product-code devnullvoid.pvetui__DefaultSource --exact --scope user --silent --disable-interactivity
```

Never use a broad name-based uninstall when another distribution is installed.

- [x] Uninstall removes the test package registration and package directory.
- [ ] WinGet automatically removes its PATH entry (residue observed; see above).
- [ ] A new terminal no longer resolves the WinGet-installed `pvetui` command.
- [ ] Existing user configuration files still exist with unchanged hashes.
- [x] Reinstall using the local manifest succeeds.

### Upgrade Test

A real upgrade needs two real release versions; reinstalling `1.4.4` is not an
upgrade test. In a temporary directory, prepare an older-version manifest set
(for example `1.4.3`) with the same identifier, portable installer layout, alias,
and publisher. Request `--scope user` when installing it. Point
it at that version's actual ZIP and verify its own published checksum. Do not
relabel the `1.4.4` binary or reuse its checksum.

1. Validate and install the older local manifest in a clean test VM.
2. Record `pvetui --version`, installed package registration, and configuration hashes.
3. Run the following from the repository root:

   ```powershell
   winget upgrade --manifest .\packaging\winget\manifests\d\devnullvoid\pvetui\1.4.4 --scope user --silent --disable-interactivity --accept-source-agreements --accept-package-agreements
   ```

4. Confirm `v1.4.4`, a single package registration, the same working alias, and
   unchanged user configuration. Uninstall and verify preservation again.

The real `1.4.3` to `1.4.4` upgrade passed with installer hash verification,
`UpgradeBehavior: uninstallPrevious`, a single updated registration, and working
`--version` / `--help`. The old ZIP's verified SHA-256 was
`55dc7e0fb35af72ec26f409804cbc0a7b325f0451162244fdeca7943fce7b401`.
Temporary older-version manifests are test fixtures, not upstream submissions.
The final uninstall left the existing Scoop shim unchanged and resolving on PATH.
No existing Proxmox configuration or credentials were inspected; configuration
preservation and interactive TUI checks are not claimed as tested.

Record Windows/WinGet versions, commands, outcomes, and sanitized errors in #170
when preparing the submission. Linux schema validation alone is not acceptance.

## Upstream Submission

The initial submission is
[microsoft/winget-pkgs#474741](https://github.com/microsoft/winget-pkgs/pull/474741).
It contains only the three `1.4.4` manifests and records completed Windows checks,
untested interactive behavior, and the observed PATH-cleanup limitation. Follow
its validation and contributor-agreement checks before claiming availability.

For future submissions:

1. Recheck `winget search pvetui --source winget`, the upstream manifest directory,
   and open upstream PRs to avoid duplicate submissions or identifier conflicts.
2. Fork `microsoft/winget-pkgs` and create a submission branch.
3. Copy only the three manifests into
   `manifests/d/devnullvoid/pvetui/1.4.4/` in that fork.
4. Submit one version in a PR titled `New package: devnullvoid.pvetui version 1.4.4`.
5. Include Windows test evidence, address upstream validation/review feedback,
   and complete any contributor agreement requested by the upstream project.
6. Only after acceptance and source availability, document the public command
   `winget install --id devnullvoid.pvetui --exact --source winget` in the README.

No submission, fork push, or release update is automatic in this setup.
Obtain explicit approval before pushing or submitting upstream.

## Future Releases

For a new published release, add a new version directory and update all three
files consistently: version, release ZIP URL, downloaded ZIP checksum,
version-pinned license URL, release notes URL, and release date. Keep package
identity, publisher, portable installer layout, and alias stable. Select scope
at install time rather than adding `Scope` to a portable manifest. Verify the actual archive layout,
repeat schema/native validation and upgrade testing, then submit a version PR.
Never edit an older version to point to newer binaries.

Release automation and additional architectures remain follow-up work in #170.
No new secret is needed for local preparation or testing. Future automated PR
submission will need a separately reviewed credential and permission design.

## References

- [WinGet authoring and repository layout](https://github.com/microsoft/winget-pkgs/blob/master/doc/Authoring.md)
- [Installer schema](https://github.com/microsoft/winget-pkgs/blob/master/doc/manifest/schema/1.28.0/installer.md)
- [Native manifest validation](https://learn.microsoft.com/en-us/windows/package-manager/winget/validate)
- [Local install](https://learn.microsoft.com/en-us/windows/package-manager/winget/install)
- [Local upgrade](https://learn.microsoft.com/en-us/windows/package-manager/winget/upgrade)
- [Uninstall](https://learn.microsoft.com/en-us/windows/package-manager/winget/uninstall)
