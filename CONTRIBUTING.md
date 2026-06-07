# Contributing to Hexyn AWS

First off, thank you for considering contributing to Hexyn AWS! It's people like you that make Hexyn AWS such a great tool.

## Development Workflow

Before you start contributing, please set up your local development environment:

1.  **Install Tools**: Run `make tools` to install the required development tools (GoReleaser, goimports) into the local `bin/` directory.
2.  **Format and Lint**: Run `make fmt` and `make lint` regularly to ensure your code follows the project's standards.
3.  **Run Tests**: Run `make test` to verify your changes.
4.  **Check Config**: Run `make check` to verify the GoReleaser configuration if you've modified it.

## How Can I Contribute?

### Reporting Bugs

- Check for existing issues.
- Provide a clear and descriptive title.
- Describe the exact steps which reproduce the problem.
- Explain which behavior you expected to see and why.

### Suggesting Enhancements

- Use a clear and descriptive title.
- Provide a step-by-step description of the suggested enhancement.
- Explain why this enhancement would be useful.

### Pull Requests

1.  **Fork the repository.**
2.  **Create a new branch** for your feature or bug fix.
3.  **Make your changes.**
4.  **Format, Lint and Test**: Run `make fmt`, `make lint`, and `make test`.
5.  **Submit the PR.**

## Styleguides

### Git Commit Messages

We use [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/) for all commit messages. This allows us to automate our changelog generation.

Format: `<type>(<scope>): <description>`

- **feat**: A new feature
- **fix**: A bug fix
- **docs**: Documentation only changes
- **style**: Changes that do not affect the meaning of the code (white-space, formatting, etc)
- **refactor**: A code change that neither fixes a bug nor adds a feature
- **perf**: A code change that improves performance
- **test**: Adding missing tests or correcting existing tests
- **chore**: Changes to the build process or auxiliary tools and libraries such as documentation generation

Example: `feat(ui): add search functionality to lists`

### Go Styleguide

- All Go code is formatted with `gofmt` and `goimports` (via `make fmt`).
- Follow standard Go idioms and naming conventions.

## Questions?

Feel free to open an issue with the "question" label.
