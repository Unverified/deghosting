# deghosting

To make the blog :100: less haunted.

## Requirements

- [Go](https://go.dev/) 1.26+
- [just](https://github.com/casey/just) for the task recipes
- [golangci-lint](https://github.com/golangci/golangci-lint) v2 for linting and formatting

If you use [Nix](https://nixos.org/), a flake-based dev shell provides all of the
above (plus `gopls`, `air` for hot reloading, and editor language servers):

```sh
nix develop
```

## Build & run

```sh
just build          # build the binary into ./bin/deghosting
just run [args...]   # build and run, passing through arguments
just install         # install into $GOBIN / $GOPATH/bin
```

## Development

```sh
just fmt        # format with golangci-lint formatters (gofumpt + goimports)
just fmt-check  # show formatting diffs without writing
just lint       # run golangci-lint
just test       # run the test suite
just test-race  # run tests with the race detector and coverage
just tidy       # go mod tidy + verify
just check      # fmt-check + lint + test
just clean      # remove build artifacts
```

Run `just` with no arguments to list all available recipes.

## Testing

Tests use Go's standard `testing` package. Prefer table-driven tests with named
cases, keep CLI tests close to the `run(args, stdin, stdout, stderr)` entrypoint,
and reserve shelling out for behavior that cannot be covered in-process. Run
`just test` for normal changes and `just test-race` when touching concurrency,
shared state, or I/O-heavy code.

## Project layout

```
cmd/deghosting/       CLI entrypoint
internal/cli/         CLI parsing and input/output validation
internal/deghosting/  conversion orchestration and Ghost-to-Zola workflow
internal/download/    concurrent asset download worker pool
internal/ghost/       Ghost export parsing and source data model
internal/zola/        generated Zola post model and Markdown rendering
```

## License

[MIT](./LICENSE) © 2026
