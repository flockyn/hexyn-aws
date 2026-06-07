# Hexyn AWS

A production-ready, high-performance CLI tool for managing AWS SSM Parameter Store and ECS configurations with a modern Terminal User Interface (TUI).

## Features

- **Interactive TUI:** Polished Terminal UI using Bubble Tea for effortless management.
- **Task Definition Sync:** Automatically discover and fetch exact secrets defined in ECS Task Definitions.
- **Smart Grouping:** Group SSM parameters into separate `.env` files based on their actual SSM paths.
- **A-Z Sorting:** All generated `.env` files are automatically sorted alphabetically and de-duplicated.
- **Integrated Login:** Securely handle temporary AWS tokens with an in-app login screen.
- **Multi-Region Support:** Quickly switch between enabled AWS regions with auto-discovery.
- **Portability:** Self-contained configuration directory (`.hexyn-aws/`).
- **Self-Update:** Easily stay up-to-date with the latest features using the `update` command.

## Installation

Run the installation command from the official landing page:
[https://flockyn.github.io/hexyn-aws](https://flockyn.github.io/hexyn-aws)

### Linux / macOS
```bash
curl -fsSL https://flockyn.github.io/hexyn-aws/install.sh | bash
```

### Windows
```powershell
iwr https://flockyn.github.io/hexyn-aws/install.ps1 | iex
```

> **Note:** Since this is a private tool, the installer will ask for your **GitHub Personal Access Token** to download the binary.

## Updating

You can update Hexyn AWS to the latest version directly from the CLI:

```bash
hexyn-aws update
# or
hexyn-aws --update
```

> **Note:** Since the repository is private, you must have your `GITHUB_TOKEN` environment variable set for the update to work.

## Configuration Directory

By default, Hexyn AWS stores its configuration in `~/.hexyn-aws/`.

### Directory Structure
- **`input/`**: Place `.env` files here that you want to **PUT** into SSM.
- **`output/`**: This is where files retrieved via **GET** are saved.
- **`credentials`**: Stores your temporary AWS session keys.

### Portable Mode
Use the `--init` flag to create and use a local `.hexyn-aws/` folder in your current directory:
```bash
hexyn-aws --init
```
When in portable mode, the tool will prioritize the local `./.hexyn-aws/input/` and `./.hexyn-aws/output/` folders.

### Keyboard Shortcuts
- **L**: Trigger Login / Change Session
- **G**: Change AWS Region
- **Q**: Quit Application
- **ESC**: Go Back / Exit
- **/**: Search in lists
- **TAB**: Move between input fields

## OS Support
Hexyn AWS is written in Go and supports **Linux**, **macOS**, and **Windows**.
