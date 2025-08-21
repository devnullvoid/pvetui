# Contributing to Proxmox TUI

Thank you for your interest in contributing to Proxmox TUI! This document provides guidelines for contributing to the project.

## Getting Started

1. Fork the repository
2. Clone your fork: `git clone https://github.com/yourusername/proxmox-tui.git`
3. Create a feature branch: `git checkout -b feature/your-feature-name`
4. Make your changes
5. Test your changes
6. Commit your changes: `git commit -m "Add your feature"`
7. Push to your fork: `git push origin feature/your-feature-name`
8. Create a Pull Request

## Development Setup

### Prerequisites
- Go 1.20 or later (1.24+ recommended)
- Access to a Proxmox VE cluster for testing
- golangci-lint for code quality checks

### Building
```bash
# Clone the repository
git clone https://github.com/devnullvoid/proxmox-tui.git
cd proxmox-tui

# Build the application
go build -o proxmox-tui ./cmd/proxmox-tui

# Run with your config
./proxmox-tui --config ./configs/config.yml
```

### Running Tests
```bash
go test ./...
```

### Linting
```bash
# Install golangci-lint if not already installed
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Run linting
golangci-lint run --timeout=5m
```

### Code Formatting
```bash
# Format code
go fmt ./...

# Fix imports
goimports -local github.com/devnullvoid/proxmox-tui -w .
```

## Code Style

- Follow Go conventions and best practices
- Use `gofmt` and `goimports` for formatting
- Write clear, descriptive commit messages
- Add tests for new functionality
- Update documentation as needed
- Follow the existing project structure and patterns

### Project Structure
```
proxmox-tui/
├── cmd/proxmox-tui/     # Application entrypoint
├── internal/            # Internal application code
│   ├── adapters/        # External service adapters
│   ├── cache/           # Caching implementation
│   ├── config/          # Configuration handling
│   ├── logger/          # Logging utilities
│   ├── ssh/             # SSH client implementation
│   ├── ui/              # User interface components
│   └── vnc/             # VNC integration
├── pkg/api/             # Proxmox API client
├── configs/             # Configuration examples
└── assets/              # Static assets (images, etc.)
```

### Commit Message Format
Use conventional commits format:
```
type(scope): description

[optional body]

[optional footer]
```

Types: `feat`, `fix`, `docs`, `style`, `refactor`, `test`, `chore`

Examples:
- `feat(ui): add VM power management controls`
- `fix(api): handle connection timeout errors`
- `docs(readme): update installation instructions`

## Reporting Issues

When reporting issues, please include:
- Go version (`go version`)
- Operating system and version
- Proxmox VE version
- Steps to reproduce the issue
- Expected vs actual behavior
- Relevant logs or error messages
- Configuration file (with sensitive data removed)

### Issue Template
```
**Environment:**
- OS: [e.g., Ubuntu 22.04, Windows 11, macOS 14]
- Go version: [e.g., go1.24.2]
- Proxmox VE version: [e.g., 8.1]

**Description:**
A clear description of the issue.

**Steps to Reproduce:**
1. Step one
2. Step two
3. Step three

**Expected Behavior:**
What you expected to happen.

**Actual Behavior:**
What actually happened.

**Logs/Error Messages:**
```
[paste relevant logs here]
```

**Additional Context:**
Any other relevant information.
```

## Feature Requests

Feature requests are welcome! Please:
- Check existing issues and discussions first
- Describe the use case and benefits clearly
- Consider implementation complexity
- Be open to discussion and feedback
- Provide mockups or examples if applicable

### Feature Request Template
```
**Feature Description:**
A clear description of the feature you'd like to see.

**Use Case:**
Describe the problem this feature would solve.

**Proposed Solution:**
How you envision this feature working.

**Alternatives Considered:**
Other approaches you've considered.

**Additional Context:**
Screenshots, mockups, or examples.
```

## Development Guidelines

### Adding New Features
1. Create an issue to discuss the feature first
2. Follow the existing architecture patterns
3. Add appropriate tests
4. Update documentation
5. Ensure CI passes

### UI Components
- Use the existing tview-based component system
- Follow the established navigation patterns
- Ensure keyboard accessibility
- Test on different terminal sizes

### API Integration
- Use the existing API client patterns
- Implement proper error handling
- Add appropriate caching where beneficial
- Follow the repository pattern for data access

### Testing
- Write unit tests for business logic
- Test error conditions
- Verify cross-platform compatibility
- Test with different Proxmox configurations

## Pull Request Process

1. Ensure your code follows the style guidelines
2. Update documentation as needed
3. Add or update tests as appropriate
4. Ensure all CI checks pass
5. Request review from maintainers
6. Address feedback promptly

### Pull Request Template
```
**Description:**
Brief description of changes.

**Type of Change:**
- [ ] Bug fix
- [ ] New feature
- [ ] Breaking change
- [ ] Documentation update

**Testing:**
- [ ] Unit tests pass
- [ ] Manual testing completed
- [ ] Cross-platform testing (if applicable)

**Checklist:**
- [ ] Code follows style guidelines
- [ ] Self-review completed
- [ ] Documentation updated
- [ ] Tests added/updated
- [ ] CI checks pass
```

## Community

- Be respectful and inclusive
- Help others learn and grow
- Share knowledge and best practices
- Provide constructive feedback

## License

By contributing, you agree that your contributions will be licensed under the MIT License.

## Questions?

Feel free to open an issue for questions or reach out to the maintainers.

Thank you for contributing to Proxmox TUI! 🚀
