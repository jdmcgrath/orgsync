# Contributing to orgsync

## Commit Message Format

This project uses [Conventional Commits](https://www.conventionalcommits.org/) for automated versioning and changelog generation.

### Commit Message Format

```
<type>[optional scope]: <description>

[optional body]

[optional footer(s)]
```

### Types

- **feat**: A new feature (triggers minor version bump)
- **fix**: A bug fix (triggers patch version bump)
- **docs**: Documentation only changes
- **style**: Changes that do not affect the meaning of the code
- **refactor**: A code change that neither fixes a bug nor adds a feature
- **perf**: A code change that improves performance (triggers patch version bump)
- **test**: Adding missing tests or correcting existing tests
- **build**: Changes that affect the build system or external dependencies
- **ci**: Changes to our CI configuration files and scripts
- **chore**: Other changes that don't modify src or test files
- **revert**: Reverts a previous commit

### Breaking Changes

To trigger a major version bump, add `BREAKING CHANGE:` in the footer or add `!` after the type:

```
feat!: remove deprecated API endpoint

BREAKING CHANGE: The /old-api endpoint has been removed. Use /new-api instead.
```

### Examples

```
feat: add support for private repositories
fix: handle rate limiting correctly
docs: update installation instructions
perf: optimize repository cloning process
feat!: change CLI argument structure
```

## Automated Releases

When you push to the `main` branch:

1. GitHub Actions will analyze your commit messages
2. Determine the next version based on conventional commits
3. Create a release PR with updated changelog
4. Once merged, automatically create a Git tag and GitHub release
5. The new version will be available via `go install github.com/jdmcgrath/orgsync@latest` 