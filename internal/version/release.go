package version

// releaseVersion, releaseCommit, and releaseDate are updated by the release
// script (scripts/create-release.sh) and committed as part of each release tag.
// They serve as a fallback for builds that lack ldflags injection and VCS info,
// most notably `go install github.com/devnullvoid/pvetui/cmd/pvetui@latest`.
const (
	releaseVersion = "1.4.1"
	releaseCommit  = "1fdf077"
	releaseDate    = "2026-06-08T00:00:00Z"
)
