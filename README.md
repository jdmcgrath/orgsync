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
1. **Install GitHub CLI (`gh`):**
   ```bash
   brew install gh
   gh auth login
   ```
2. **Install OrgSync:**
    - Clone this repository
    ```bash
    git clone https://github.com/yourusername/yourproject.git
    cd yourproject
    ```
    - Build and install the OrgSync tool
    ```bash
    go install ./cmd/orgsync
    ```
    - Ensure that the GOBIN or GOPATH/bin directory is in your system's PATH:
    ```bash
    export PATH=$PATH:$(go env GOPATH)/bin
    ```
    - Verify that OrgSync is installed by running:
    ```bash
    orgsync --version
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
#### Notes
- The tool will display progress in your terminal and allow you to quit with q.

## Development
### Running locally
1. Clone this repository
```bash
git clone https://github.com/yourusername/yourproject.git
cd yourproject
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
We welcome contributions! Here's how you can get involved:

Fork this repository
Create a feature branch: `git checkout -b my-new-feature`
Commit your changes: `git commit -m 'Add some feature'`
Push to your branch: `git push origin my-new-feature`
Create a new Pull Request