# BookBeam CLI (`bookbeam`)

The official command-line interface for [BookBeam](https://bookbeam.app) — the author platform for book distribution, lead magnets, and reader delivery.

Built in Go as a standalone binary with zero dependencies. The CLI orchestrates BookBeam's public REST API to automate catalog management, asset uploads, reader signup links, subscriber exports, and telemetry.

## Installation

### Homebrew (macOS and Linux)

```bash
brew install jonbaldie/tap/bookbeam
```

This uses the [jonbaldie Homebrew tap](https://github.com/jonbaldie/homebrew-tap), alongside ProSie, and installs the `bookbeam` command on Apple Silicon, Intel Macs, and ARM64 or AMD64 Linux.

### Binary Download

Download pre-compiled binaries for Linux, macOS, and Windows from the [Releases](https://github.com/jonbaldie/bookbeam-cli/releases) page.

```bash
# macOS (Apple Silicon)
curl -sSL https://github.com/jonbaldie/bookbeam-cli/releases/latest/download/bookbeam-darwin-arm64.tar.gz | tar -xz
sudo mv bookbeam-darwin-arm64 /usr/local/bin/bookbeam
```

### Go Install

```bash
go install github.com/jonbaldie/bookbeam-cli@latest
```

Go names the installed binary `bookbeam-cli`. Rename it to `bookbeam` (or `bookbeam.exe` on Windows) and add your Go bin directory to `PATH` to use the examples below.

## Authentication

### Browser Login (OAuth 2.0 Device Flow)

Run `bookbeam auth login` to authenticate securely via browser without manual token copying:

```bash
bookbeam auth login
```

The CLI requests a one-time user code, opens `https://bookbeam.app/device`, and automatically completes authentication once approved.

### Direct Token Authentication

For CI/CD or headless servers, provide your Personal Access Token directly:

```bash
# Save to ~/.config/bookbeam/config.json
bookbeam auth login --token bb_pat_1234567890

# Or via environment variable
export BOOKBEAM_TOKEN="bb_pat_1234567890"
```

To verify your active identity:

```bash
bookbeam whoami
```

To log out:

```bash
bookbeam auth logout
```

## Command Reference

### Book Projects

```bash
# List all book projects with pagination
bookbeam projects list [--page 1]

# View project details
bookbeam projects get <project-id>

# Create a new project (with optional cover image upload)
bookbeam projects create --title "The Quantum Paradox" --description "Sci-fi novel" [--cover ./cover.jpg]

# Update an existing project
bookbeam projects update <project-id> --title "New Title" [--remove-cover]

# Assign mailing list and tags
bookbeam projects newsletter <project-id> --list-id "lst_123" --tags "sci-fi,readers"

# Delete a project and attached assets
bookbeam projects delete <project-id> [--force]
```

### Book Files

```bash
# List attached files (EPUB, MOBI, PDF)
bookbeam files list <project-id>

# Upload a file with progress tracking
bookbeam files upload <project-id> ./book.epub

# Download a book file to disk
bookbeam files download <project-id> <file-id> [--output ./downloaded.epub]

# Delete a file
bookbeam files delete <project-id> <file-id> [--force]
```

### Signup Links & Landing Pages

```bash
# List all signup links for a project
bookbeam links list <project-id>

# Create a new reader signup link
bookbeam links create <project-id> --title "Sample Chapter" --consent "Join my newsletter"

# Update signup link copy
bookbeam links update <project-id> <link-id> --title "Updated Chapter"

# Delete a link
bookbeam links delete <project-id> <link-id> [--force]
```

### Subscriber Records & CSV Export

```bash
# View recent downloader signups
bookbeam downloaders list <project-id> [--page 1]

# Export sanitized CSV list of subscribers
bookbeam downloaders export <project-id> [--output ./subscribers.csv]
```

### Newsletter Settings

```bash
# View active provider configuration
bookbeam newsletter status

# Connect a provider (MailerLite, Kit, Mailcoach)
bookbeam newsletter configure --provider mailerlite --api-key "ml_secret_key"

# Query available mailing lists and tags
bookbeam newsletter lists

# Set or clear subscriber webhook URL
bookbeam newsletter webhook --url "https://hooks.zapier.com/..."

# Disconnect active provider
bookbeam newsletter disconnect
```

### Telemetry & Analytics

```bash
# View chronological reader activity feed (limit is capped at 100)
bookbeam logs [--limit 50] [--event signup]

# View catalog metrics and download totals
bookbeam metrics
```

`--event` filters the table locally, since the API has no event parameter; `--json` returns the API's paginated response untouched.

### Billing & Subscription

```bash
# Check team entitlement and active offer
bookbeam billing status

# View or open the checkout link
bookbeam billing checkout
```

## Global Flags

Every command supports:

- `--json`: Output raw structured JSON for scripts, jq, and automation.
- `--quiet, -q`: Suppress informational messages and progress output.
- `--host`: Override target API host (defaults to `https://bookbeam.app`).
- `--token`: Override API authentication token for single commands.

## Environment Variables

- `BOOKBEAM_HOST`: Override the target API host.
- `BOOKBEAM_TOKEN`: Override the API authentication token.
- `BOOKBEAM_CONFIG_DIR`: Directory holding `config.json`. Defaults to
  `~/.config/bookbeam`. Point it elsewhere to keep a sandbox — a test suite, a
  CI job, a second account — away from your real configuration.

## Shell Autocompletion

Generate shell completion scripts:

```bash
# Bash
source <(bookbeam completion bash)

# Zsh
bookbeam completion zsh > "${fpath[1]}/_bookbeam"

# Fish
bookbeam completion fish | source
```

## Documentation

See [`docs/`](docs/README.md) for project documentation, including the [2026-09-17 Exploratory Testing Pass](docs/exploratory-testing/2026-09-17-bookbeam-cli.md).

## License

Subject Zero Ltd. Open source under the MIT License.
