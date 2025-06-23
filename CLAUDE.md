# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

OrgSync is a Go-based CLI tool that synchronizes all repositories from a GitHub organization or user. It uses the Bubble Tea framework for a modern terminal UI with real-time progress tracking.

## Development Commands

### Building and Running
```bash
# Install dependencies
go get ./...

# Run the application
go run . <org_name>

# Build the binary
go build -v ./...

# Install locally
go install
```

### Testing
```bash
# Run all tests
go test -v ./...

# Run tests for specific package
go test -v ./sync

# Run benchmarks
go test -bench=. ./sync
```

## Architecture

### Core Components
1. **main.go**: CLI entry point, argument parsing, and help documentation
2. **sync/sync.go**: Core synchronization logic with Bubble Tea TUI model
3. **sync/sync_test.go**: Unit tests using table-driven test patterns

### Key Dependencies
- **charmbracelet/bubbletea**: Terminal UI framework
- **charmbracelet/bubbles**: UI components (progress bars, spinners)
- **charmbracelet/lipgloss**: Terminal styling
- **GitHub CLI (gh)**: Required external dependency for API access

### Synchronization Flow
1. Fetches repository list using `gh api` commands
2. Processes repos concurrently (max 5 by default)
3. Detects existing repos vs new ones
4. Executes `git clone` or `git fetch` as appropriate
5. Implements retry logic (2 attempts) for failures
6. Updates TUI in real-time with status changes

## Important Patterns

### Bubble Tea Model Structure
- Model implements `tea.Model` interface with `Init()`, `Update()`, and `View()` methods
- Uses message types for async communication (e.g., `repositoriesMsg`, `statusMsg`)
- Maintains repository status states: Pending, Cloning, Fetching, Completed, Failed

### Error Handling
- Categorizes errors as network vs non-network
- Implements retry logic for network errors only
- 30-second timeout per operation
- Context-based cancellation support

### Testing Approach
- Table-driven tests for multiple scenarios
- Mock error types for error categorization testing
- Benchmarks for performance-critical functions
- Tests cover status strings, configuration, and model updates

## Release Process

This project uses conventional commits and automated releases:

1. **Commit Format**: `<type>(<scope>): <subject>`
   - Types: feat, fix, docs, style, refactor, perf, test, build, ci, chore, revert
   - Breaking changes: Add `!` after type or `BREAKING CHANGE:` in footer

2. **Automated Release**: Push to main triggers Release Please action
   - Creates release PR automatically
   - Updates version in `.release-please-manifest.json`
   - Generates CHANGELOG.md
   - Creates GitHub release with binaries

## Development Reminders

- Always ensure GitHub CLI (`gh`) is installed and authenticated before testing
- The entry point is `main.go`, not `cmd/orgsync` as incorrectly mentioned in README
- Default concurrency is 5 repos - can be adjusted in `config` struct
- UI width adapts between 80-140 characters
- Color gradients are defined in `getGradientColors()` for visual appeal
- Test mode allows UI testing without creating actual repositories