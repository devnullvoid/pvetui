package version

// releaseVersion, releaseCommit, and releaseDate are updated by the release
// script (scripts/create-release.sh) and committed as part of each release tag.
// They serve as a fallback for builds that lack ldflags injection and VCS info,
// most notably `go install github.com/devnullvoid/pvetui/cmd/pvetui@latest`.
const (
	releaseVersion = "1.4.3"
	releaseCommit  = "c0c2888"
	releaseDate    = "2026-07-16T00:00:00Z"
)
