# Contributing to Dakota Go SDK

Thank you for your interest in contributing to the Dakota Go SDK! We welcome contributions from the community.

## Code of Conduct

Please be respectful and constructive in all interactions. We expect all contributors to:

- Be welcoming and inclusive
- Be respectful of differing viewpoints
- Accept constructive criticism gracefully
- Focus on what is best for the community

## How to Contribute

### Reporting Bugs

Before submitting a bug report:

1. Check existing [issues](https://github.com/dakota-xyz/go-sdk/issues) to avoid duplicates
2. Update to the latest version to see if the issue persists

When submitting a bug report, include:

- **Clear title** describing the issue
- **Steps to reproduce** the problem
- **Expected behavior** vs **actual behavior**
- **Code samples** or minimal reproduction
- **Environment details**:
  ```bash
  go version
  go env GOOS GOARCH
  ```

### Suggesting Enhancements

Enhancement suggestions are welcome! Please include:

- **Clear title** describing the enhancement
- **Detailed description** of the proposed feature
- **Use case** explaining why this would be useful
- **Alternative solutions** you've considered

### Pull Requests

1. **Fork** the repository
2. **Clone** your fork:
   ```bash
   git clone https://github.com/YOUR_USERNAME/go-sdk.git
   cd go-sdk
   ```
3. **Create a branch**:
   ```bash
   git checkout -b feature/your-feature-name
   ```
4. **Make your changes** and add tests
5. **Run tests and checks**:
   ```bash
   go test ./...
   go vet ./...
   go fmt ./...
   ```
6. **Commit** with a descriptive message (see commit format below)
7. **Push** and create a Pull Request

## Development Setup

### Prerequisites

- Go 1.21 or later
- Make (optional, for convenience commands)

### Commands

```bash
# Run tests
go test ./...

# Run tests with coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Format code
go fmt ./...

# Vet code
go vet ./...

# Build
go build ./...

# Regenerate client from OpenAPI spec
make generate-client
# or
./scripts/generate-client.sh
```

### Project Structure

```
client/              # Main package users import
├── client.go        # Client initialization & options
├── environment.go   # Sandbox/Production environments
├── errors.go        # Error handling
├── pagination.go    # Iterator helpers
├── parsers.go       # Model converters
└── gen/             # Generated from OpenAPI (don't edit manually)
    ├── client.gen.go
    └── openapi.yaml

webhook/             # Webhook signature verification
errors/              # SDK error types
log/                 # Logging abstraction
```

## Coding Standards

### Go Conventions

- Follow [Effective Go](https://golang.org/doc/effective_go)
- Use `gofmt` for formatting
- Add GoDoc comments for exported types and functions
- Keep functions small and focused
- Handle errors explicitly

### Style

- Run `go fmt ./...` before committing
- Run `go vet ./...` to catch common mistakes
- Use meaningful variable and function names
- Prefer explicit over implicit

### Testing

- Write tests for all new features and bug fixes
- Use table-driven tests where appropriate
- Mock external dependencies
- Aim for good coverage on critical paths

## Commit Message Format

We follow [Conventional Commits](https://www.conventionalcommits.org/):

```
<type>(<scope>): <description>

[optional body]

[optional footer]
```

### Types

- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation only
- `style`: Code style (formatting, etc.)
- `refactor`: Code change that neither fixes a bug nor adds a feature
- `test`: Adding or updating tests
- `chore`: Build process, dependencies, or tooling

### Examples

```
feat(client): add retry logic for rate-limited requests
fix(webhook): handle missing timestamp header
docs(readme): add webhook setup instructions
test(pagination): add coverage for iterator edge cases
```

## Documentation

- Update README.md for user-facing changes
- Update AGENTS.md for API reference changes
- Add GoDoc comments for new public functions
- Include code examples where helpful

## Release Process

Releases are managed by maintainers. Version bumps follow [Semantic Versioning](https://semver.org/):

- **MAJOR**: Breaking changes
- **MINOR**: New features (backwards compatible)
- **PATCH**: Bug fixes (backwards compatible)

## Questions?

- Check the [documentation](https://docs.dakota.xyz)
- Open a [discussion](https://github.com/dakota-xyz/go-sdk/discussions)
- Contact: support@dakota.xyz

## License

By contributing, you agree that your contributions will be licensed under the [MIT License](LICENSE).
