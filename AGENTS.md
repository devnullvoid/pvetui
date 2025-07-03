# AGENT INSTRUCTIONS

The following conventions must be followed for any changes in this repository.

## Workflow
1. Ensure git submodules are initialized: `git submodule update --init --recursive`.
2. Confirm the application builds with `make build`.
3. Run linters using `make lint`. Capture and report any failures.
4. Run `go vet ./...` followed by `make test` to execute all tests.
5. Keep the working tree clean before finishing.

## Style
- Follow idiomatic Go and clean architecture practices.
- Prefer small interfaces and dependency injection via constructors or options.
- Document exported identifiers with GoDoc comments.
- Update `CHANGELOG.md` under **[Unreleased]** with a short bullet for user visible changes.
- Format code using `go fmt` and `goimports`.
- Write concise commit messages (imperative mood, present tense).

