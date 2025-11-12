# Third-Party Licenses

pvetui includes redistributed components whose licenses require attribution or redistribution of their terms. This file summarizes each dependency and where to find the corresponding license text in this repository so they are included with any binary release artifacts.

| Component / Files | Source | License | Notes |
|-------------------|--------|---------|-------|
| noVNC core library (`internal/vnc/novnc/core/**/*.js`, `internal/vnc/novnc/app/*.js`, `internal/vnc/novnc/test/playback.js`, plus vendored third-party JS such as `vendor/pako/`) | https://github.com/novnc/noVNC | MPL-2.0 (core JS), MIT (`vendor/pako`) | License text: `internal/vnc/novnc/LICENSE.txt` (aggregates the MPL notice) and `internal/vnc/novnc/vendor/pako/LICENSE`. Any modifications must remain under MPL-2.0 and be distributed alongside these files. |
| noVNC HTML/CSS (`internal/vnc/novnc/*.html`, `internal/vnc/novnc/app/styles/*.css`) | https://github.com/novnc/noVNC | BSD 2-Clause | License text for the bundled assets is included within `internal/vnc/novnc/LICENSE.txt`. |
| Orbitron font (`internal/vnc/novnc/app/styles/Orbitron*`) | https://github.com/novnc/noVNC | SIL Open Font License 1.1 | License text: `internal/vnc/novnc/LICENSE.txt` (contains the OFL terms). |
| noVNC images (`internal/vnc/novnc/app/images/*`) | https://github.com/novnc/noVNC | Creative Commons Attribution-ShareAlike 3.0 | License text: `internal/vnc/novnc/LICENSE.txt`. Attribution: “noVNC images © the noVNC authors, CC BY-SA 3.0”. |
| Other noVNC assets not explicitly listed | https://github.com/novnc/noVNC | See `internal/vnc/novnc/LICENSE.txt` | The upstream LICENSE describes each sub-license. |

## Distribution guidance

When creating release artifacts (tarballs, container images, etc.), include this `THIRD_PARTY_LICENSES.md` file, `internal/vnc/novnc/LICENSE.txt`, and `internal/vnc/novnc/vendor/pako/LICENSE` so recipients have access to the MPL, BSD, OFL, MIT, and CC-BY-SA terms as required. If the binary distribution cannot practically include the entire subtree, provide the license texts next to the binary or in the release notes and host the modified source (if any) in a publicly accessible repository (e.g., this GitHub repo).

If you modify MPL-licensed noVNC files, publish those modifications under MPL-2.0 and keep their original copyright notices. If you modify CC BY-SA images, the derivatives must also remain under CC BY-SA 3.0 and retain attribution.
