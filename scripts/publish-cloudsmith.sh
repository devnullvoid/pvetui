#!/usr/bin/env bash
set -euo pipefail

repo="${CLOUDSMITH_REPOSITORY:-devnullvoid/pvetui}"
deb_target="${CLOUDSMITH_DEB_TARGET:-any-distro/any-version}"
rpm_target="${CLOUDSMITH_RPM_TARGET:-any-distro/any-version}"
component="${CLOUDSMITH_DEB_COMPONENT:-main}"
artifact_dir="${1:-dist}"

auth_args=()
if [[ -n "${CLOUDSMITH_API_KEY:-}" ]]; then
	auth_args=(--api-key "$CLOUDSMITH_API_KEY")
else
	cloudsmith whoami >/dev/null
fi

if [[ ! -d "$artifact_dir" ]]; then
	printf 'Artifact directory not found: %s\n' "$artifact_dir" >&2
	exit 1
fi

shopt -s nullglob
deb_packages=("$artifact_dir"/*.deb)
rpm_packages=("$artifact_dir"/*.rpm)

if (( ${#deb_packages[@]} == 0 && ${#rpm_packages[@]} == 0 )); then
	printf 'No DEB/RPM packages found in %s; skipping Cloudsmith publish.\n' "$artifact_dir"
	exit 0
fi

for package in "${deb_packages[@]}"; do
	printf 'Publishing %s to Cloudsmith DEB repository %s/%s\n' "$package" "$repo" "$deb_target"
	cloudsmith push deb \
		"${auth_args[@]}" \
		--component "$component" \
		--republish \
		"$repo/$deb_target" \
		"$package"
done

for package in "${rpm_packages[@]}"; do
	printf 'Publishing %s to Cloudsmith RPM repository %s/%s\n' "$package" "$repo" "$rpm_target"
	cloudsmith push rpm \
		"${auth_args[@]}" \
		--republish \
		"$repo/$rpm_target" \
		"$package"
done
