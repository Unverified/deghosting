# Repository Guidelines

## Project Structure & Module Organization

This repository is an early Go CLI scaffold for `deghosting`. The executable entrypoint lives in `cmd/cli/main.go`, with the binary built to `bin/deghosting`. Keep command-line wiring in `cmd/cli`; move reusable domain logic into new internal packages as functionality grows, for example `internal/deghosting` or `internal/config`. Add tests beside the package they cover using Go's `*_test.go` convention. Do not commit generated binaries or other `bin/` artifacts.

## Build, Test, and Development Commands

Use `just` as the task runner. Run `just` to list recipes.

- `nix develop`: enter the optional dev shell with Go, `just`, `golangci-lint`, and editor tooling.
- `just build`: compile `./cmd/cli` into `./bin/deghosting`.
- `just run -- [args...]`: run the CLI locally and pass through arguments.
- `just test`: run `go test ./...`.
- `just test-race`: run tests with race detection and coverage.
- `just fmt-check`: show formatting diffs without writing.
- `just fmt`: format Go files via `golangci-lint fmt`.
- `just lint`: run `golangci-lint run`.
- `just check`: run format checks, linting, and tests.
- `just tidy`: run `go mod tidy` and `go mod verify`.

## Coding Style & Naming Conventions

Target Go `1.26.4` as declared in `go.mod`. Format Go code with `golangci-lint fmt`, which includes `gofumpt` and `goimports`; use tabs as produced by `gofmt`. Keep package names short, lowercase, and singular where practical. Prefer explicit error handling, small functions, and dependency injection through parameters such as `context.Context` or `io.Writer`, matching the current `run(ctx, args, stdout, stderr)` pattern.

## Agent Skill Usage

Use the local Go skills when a task matches their scope. Start with `golang-how-to` for broad Go coding, debugging, review, or setup work, then load focused skills such as `golang-cli`, `golang-testing`, `golang-lint`, `golang-error-handling`, `golang-security`, or `golang-documentation` as needed. Keep skill use proportional: read only the relevant `SKILL.md` guidance and any directly referenced material needed for the change.

## Testing Guidelines

Use Go's standard `testing` package unless a stronger need appears. Name test files `*_test.go` and test functions `TestName`. Prefer table-driven CLI tests with a `name` field for every case, and call `run(ctx, args, stdout, stderr)` instead of shelling out when possible. Test observable behavior and public contracts instead of implementation details; keep each test deterministic and independently runnable. Use `t.Parallel()` for isolated tests, separate integration tests with `//go:build integration`, and add `goleak` or `testing/synctest` when concurrent code needs leak checks or deterministic time. Run `just test` before submitting changes; use `just test-race` for concurrency, shared state, or I/O-heavy code.

## Commit & Pull Request Guidelines

The repository currently has only an initial commit, so follow a simple imperative style: `Add CLI argument parsing`, `Fix config loading`, `Document build workflow`. Keep commits focused and include tests or docs with the behavior they support. Pull requests should explain the change, list verification commands run, link related issues, and include terminal output or screenshots when user-visible CLI behavior changes.

## Security & Configuration Tips

Do not commit secrets, local environment files, or generated binaries. Keep dependency changes intentional: run `just tidy` after editing imports or module metadata, and review `go.mod` changes before opening a PR.
