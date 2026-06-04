# Use alecthomas/kong for CLI parsing

We parse the command line with `github.com/alecthomas/kong` and deliberately reject the Go-default `cobra` (and `viper`, `koanf`, `urfave/cli`). Kong's struct-tag-driven model keeps the surface tiny for a single-command tool, and using its low-level `parser.Parse` lets us own error reporting and exit-code mapping (usage errors → 2, runtime errors → 1) instead of inheriting cobra's command-tree machinery and `os.Exit` behavior.
