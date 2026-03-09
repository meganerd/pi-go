# pi-go

Go rewrite of the [pi coding agent](https://github.com/badlogic/pi-mono/tree/main/packages/coding-agent) — an LLM-powered coding assistant.

## Why

The original pi is written in TypeScript/Node.js. This is a from-scratch Go implementation preserving the core architecture: pluggable tools, multi-provider LLM support, session persistence, context compaction, and extensibility.

Single binary. No npm. No Node.js.

## Build

```bash
make build        # Build for host (dynamic)
make static       # Build for host (static, CGO_ENABLED=0)
make test         # Run tests
make lint         # Run linter
make install      # Install to $GOPATH/bin
```

### Cross-compilation

Build for all supported platforms at once:

```bash
make all          # All platforms (dynamic)
make all-static   # All platforms (static — recommended for distribution)
make dist         # Static builds + tarballs + sha256 checksums
```

Build for a specific platform:

```bash
make linux-amd64-static
make darwin-arm64-static
make windows-amd64-static
```

#### Supported platforms

| OS | Architecture | Target |
|----|-------------|--------|
| Linux | amd64 | `linux-amd64` |
| Linux | arm64 | `linux-arm64` |
| Linux | riscv64 | `linux-riscv64` |
| macOS | amd64 | `darwin-amd64` |
| macOS | arm64 (Apple Silicon) | `darwin-arm64` |
| Windows | amd64 | `windows-amd64` |
| Windows | arm64 | `windows-arm64` |

> Go does not support RISC-V 32-bit (`riscv32`). Only `riscv64` is available.

## Usage

```bash
pi-go --version
pi-go --model claude-sonnet-4-20250514 --provider anthropic
```

## Architecture

```
cmd/pi-go/         CLI entry point
internal/
  tool/             Tool interface + implementations (read, write, edit, bash, grep, find, ls)
  provider/         LLM provider abstraction
  session/          Session persistence (append-only JSONL)
  config/           Configuration layering
  message/          Conversation message types
  et/               Electrictown integration (optional)
```

## Electrictown Integration

pi-go can optionally delegate subtasks to [electrictown](https://github.com/meganerd/electrictown) workers for parallel code generation. Configure in your pi-go config:

```yaml
et:
  enabled: true
  config_path: ~/electrictown.yaml
  output_dir: /tmp/pi-go-et
```

When enabled, pi-go can route appropriate subtasks (file generation, test writing, refactoring) to et workers using local Ollama models, keeping cloud API costs low.

## Development

This project uses test-driven development. Every tool implementation has corresponding tests. Run the full suite:

```bash
make test
```

## License

MIT
