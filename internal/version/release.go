package version

// releaseVersion, releaseCommit, and releaseDate are updated by the release
// script (scripts/create-release.sh) and committed as part of each release tag.
// They serve as a fallback for builds that lack ldflags injection and VCS info,
// most notably `go install github.com/devnullvoid/pvetui/cmd/pvetui@latest`.
const (
	releaseVersion = "1.4.4"
	releaseCommit  = "1049bee"
	releaseDate    = "2026-10-03T00:00:00Z"
)
