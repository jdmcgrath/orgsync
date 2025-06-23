# OrgSync

A command-line tool that keeps all repositories in a GitHub organization or from a GitHub User up-to-date. Simply provide the GitHub organization/username, and OrgSync will clone new repositories and fetch changes for already cloned ones.

![OrgSync Demo](./orgsync.gif)

## Features
- **Clone New Repos:** Clones all repositories that are not yet present locally.
- **Fetch Changes:** Fetches changes from the `origin` remote for already cloned repositories.
- **Concurrency:** Syncs all repositories concurrently for speed.

## Prerequisites
- [Go](https://golang.org/dl/) (version 1.22.2 or later)
- [GitHub CLI (`gh`)](https://cli.github.com/)
- Git (installed and available in your PATH)

## Installation

### Quick Install (Recommended)
```bash
go install github.com/jdmcgrath/orgsync@latest
```

### Prerequisites
- [GitHub CLI (`gh`)](https://cli.github.com/) - Install and authenticate:
   ```bash
   brew install gh
   gh auth login
   ```

### Manual Installation
If you prefer to build from source:
1. Clone this repository:
   ```bash
   git clone https://github.com/jdmcgrath/orgsync.git
   cd orgsync 
   ```
2. Build and install:
   ```bash
   go install .
   ```

## Usage
### Basic Usage
```bash
orgsync <your-github-org>
```
### Example
```bash
orgsync openai
```

### Test Mode
Run OrgSync in test mode to see the UI without creating actual repositories:
```bash
# Default test mode (20 repos)
orgsync --test test-org

# Custom test scenarios
orgsync --test --test-repos=50 --test-fail-rate=0.15 test-org

# Run the interactive test script
./test-ui.sh
```

#### Notes
- The tool will display progress in your terminal
- Press 'q' to quit at any time
- Press 'c' to toggle showing completed repositories

## Development
### Running locally
1. Clone this repository
```bash
git clone https://github.com/jdmcgrath/orgsync.git
cd orgsync 
```
2. Install dependencies
```bash
go get ./...
```
3. Build and run
```bash
go run ./cmd/orgsync <your-github-org>
```

### Contributing
We welcome contributions! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for details on:
- Conventional commit format for automated releases
- Development setup
- Pull request process

Quick start:
1. Fork this repository
2. Create a feature branch: `git checkout -b my-new-feature`
3. Commit your changes using [conventional commits](https://www.conventionalcommits.org/): `git commit -m 'feat: add some feature'`
4. Push to your branch: `git push origin my-new-feature`
5. Create a new Pull Request