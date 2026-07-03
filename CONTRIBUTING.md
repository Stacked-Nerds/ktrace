# Contributing to ktrace

Thank you for your interest in contributing to ktrace!

## Development Setup

1. Clone the repository:
   ```bash
   git clone https://github.com/Stacked-Nerds/ktrace.git
   cd ktrace
   ```

2. Ensure Go 1.26+ is installed.

3. Download dependencies:
   ```bash
   go mod download
   ```

4. Run tests:
   ```bash
   make test
   ```

## Code Style

- Follow standard Go conventions and idioms.
- Run `make fmt` before committing.
- All code must pass `make test`, `make vet`, and `make lint`.
- Keep functions small and packages cohesive.
- Use `context.Context` for operations that touch the network.
- Wrap errors with `%w` and meaningful context.
- Avoid unnecessary abstractions and interfaces.

## Pull Requests

1. Fork the repository and create a feature branch.
2. Write tests for new behavior.
3. Update documentation if you change user-facing behavior.
4. Ensure CI passes.
5. Open a pull request with a clear description of the change and why it is needed.

## Project Structure

```
cmd/ktrace/          CLI entrypoint
internal/cli/        Cobra commands and output formatting
internal/kubernetes/   Kubernetes client wrapper
internal/collector/  Resource collectors and orchestrator
pkg/models/          Shared domain types
pkg/errors/          Typed errors
pkg/utils/           Small helpers
```

Future packages (`correlator`, `analyzer`, `timeline`, `renderer`) will be added in later phases. See [docs/Architecture.md](docs/Architecture.md).

## Reporting Issues

When filing a bug report, include:

- ktrace version (`ktrace --version`)
- Kubernetes version
- The command you ran
- Expected vs actual behavior

## License

By contributing, you agree that your contributions will be licensed under the Apache License 2.0.
