# jtpost

CLI editor for managing your content pipeline (Telegram).

> 🇬🇧 English | [🇷🇺 Русский](./README.md)

[![Go Version](https://img.shields.io/badge/Go-1.25.5+-00ADD8?style=flat&logo=go)](https://go.dev)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/jtprogru/jtpost)](https://goreportcard.com/report/github.com/jtprogru/jtpost)
[![CI](https://github.com/jtprogru/jtpost/actions/workflows/ci.yml/badge.svg)](https://github.com/jtprogru/jtpost/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/jtprogru/jtpost)](https://github.com/jtprogru/jtpost/releases)

**Version:** 0.3.0 | **Status:** Active development

---

## Overview

**jtpost** is a command-line tool for managing the full lifecycle of posts: from idea to publication in a Telegram channel.

### Features

- ✅ Post creation with YAML frontmatter + Markdown body
- ✅ Status pipeline: `idea` → `draft` → `ready` → `scheduled` → `published`
- ✅ Publishing to Telegram (text, single image, media groups, MarkdownV2 / HTML)
- ✅ Filtering and search
- ✅ Publication scheduling
- ✅ Importing existing Markdown files
- ✅ Multiple storage backends: filesystem, SQLite, Git, PostgreSQL
- ✅ HTTP API + Web UI v2 (htmx + templ, dark/light theme, PWA)
- ✅ Multi-tenant + RBAC (users, PAT tokens, sessions, OAuth GitHub)
- ✅ Audit log, structured logging, rate limiting middleware
- ✅ UUID v7 identifiers

## Installation

### Homebrew (macOS / Linux)

```bash
brew install jtprogru/tap/jtpost
```

### Docker

Multi-arch images (linux/amd64, linux/arm64) are published to GHCR:

```bash
docker pull ghcr.io/jtprogru/jtpost:latest
docker run --rm -v "$PWD:/data" ghcr.io/jtprogru/jtpost list
```

### From source

```bash
git clone https://github.com/jtprogru/jtpost.git
cd jtpost
go install ./cmd/jtpost
```

### `go install`

```bash
go install github.com/jtprogru/jtpost/cmd/jtpost@latest
```

### Pre-built binaries

Download a binary from the [Releases](https://github.com/jtprogru/jtpost/releases) page. The `checksums.txt` GPG signature can be verified with `gpg --verify checksums.txt.sig checksums.txt`.

## Requirements

- Go 1.25.5+
- macOS, Linux, Windows

## Quickstart

### 1. Initialize the project

```bash
jtpost init
```

Creates a `.jtpost.yaml` file with default configuration.

### 2. Create a new post

```bash
jtpost new "My first post"
```

Creates a Markdown file with frontmatter inside the posts directory.

### 3. List posts

```bash
jtpost list
jtpost list --status draft
jtpost list --tag golang
```

### 4. Change status

```bash
jtpost status <id> --set ready
```

---

## Usage examples

### 📝 Full create-and-publish cycle

```bash
# 1. Create a post with title and tags
jtpost new "How to optimise Go code" --tag go --tag performance

# 2. Edit the body in your editor
jtpost edit <id>

# 3. Start working on it (idea → draft)
jtpost status <id> --set draft

# 4. Mark as ready (draft → ready)
jtpost status <id> --set ready

# 5. Publish to Telegram
jtpost publish <id>

# 6. Mark as published
jtpost status <id> --set published
```

### 🔍 Search and filtering

```bash
# All drafts
jtpost list --status draft

# Posts tagged "golang"
jtpost list --tag golang

# Combined filters
jtpost list --status draft --tag go --platform telegram

# Search by title
jtpost list --search "optimisation"

# JSON output
jtpost list --format json
```

### 📅 Content planning

```bash
# Monthly publication plan
jtpost plan

# Weekly plan
jtpost plan --days 7

# Get the recommended next post to work on
jtpost next

# Stats over your post pipeline
jtpost stats
```

### 📥 Importing existing posts

```bash
# Dry-run preview
jtpost import --dry-run

# Import all Markdown files from content/posts/
jtpost import

# Interactive mode with per-file confirmation
jtpost import --interactive
```

### 🗄️ SQLite storage

```bash
# Migrate from filesystem to SQLite
jtpost migrate

# Use SQLite via flag
jtpost list --db .jtpost.db

# Reverse migration (SQLite → FS)
jtpost migrate --to fs
```

### 🌐 HTTP API and Web UI

```bash
# Start the server on localhost:8080
jtpost serve

# Use a different port
jtpost serve --port 3000

# Bind to all interfaces
jtpost serve --addr 0.0.0.0 --port 8080
```

After startup:

- **Web UI:** <http://localhost:8080/ui/>
- **API:** <http://localhost:8080/api/posts>

### 🔧 Scripting examples

**Scheduled auto-publish via cron:**

```bash
# crontab -e
# Publish scheduled posts every hour
0 * * * * jtpost list --status scheduled --format json | jq -r '.[].id' | xargs -I {} jtpost publish {} --to telegram
```

**Bulk post creation:**

```bash
#!/bin/bash
# create-series.sh
for topic in "basics" "intermediate" "advanced"; do
  jtpost new "Go Guide: $topic" --tag go --tag series
done
```

**Stats export:**

```bash
# Export stats to JSON
jtpost stats --format json > stats.json

# Tag analysis
jtpost list --format json | jq '[.[].tags] | flatten | group_by(.) | map({tag: .[0], count: length})'
```

## CLI commands

| Command | Description |
|---------|-------------|
| `jtpost init` | Initialise the project (create `.jtpost.yaml`) |
| `jtpost new <title>` | Create a new post |
| `jtpost list` | List all posts |
| `jtpost show <id>` | Show post details |
| `jtpost status <id> --set <status>` | Change post status |
| `jtpost edit <id>` | Edit a post in `$EDITOR` |
| `jtpost delete <id>` | Delete a post |
| `jtpost import` | Import posts from `content/posts/` |
| `jtpost migrate` | Migrate filesystem → SQLite |
| `jtpost publish <id>` | Publish to Telegram |
| `jtpost plan` | Show publication plan |
| `jtpost stats` | Post pipeline stats |
| `jtpost next` | Show the next post to work on |
| `jtpost serve` | Start the HTTP API server |
| `jtpost worker run` | Run the publish worker (outbox) |
| `jtpost docs` | Generate Markdown CLI reference |
| `jtpost doctor` | Diagnose configuration and dependency availability |
| `jtpost --help` | Show help |

Full Markdown reference: [`docs/cli/`](./docs/cli/jtpost.md). Regenerate with `jtpost docs`.

## Post format

Posts are stored as Markdown with a YAML frontmatter:

```yaml
---
id: "0195e8d4-3c7a-7b2e-8f3a-9c5d6e4f2a1b"
title: "Post title"
slug: "my-first-post"
status: "draft"
deadline: "2026-02-01"
scheduled_at: "2026-02-03T10:00:00+03:00"
tags: ["golang", "cli"]
external:
  telegram_url: ""
---

Post body in Markdown...
```

## Configuration

`.jtpost.yaml` in the project root:

```yaml
# Posts and templates directories
posts_dir: content/posts
templates_dir: templates

# SQLite storage (optional)
sqlite:
  dsn: .jtpost.db

# Telegram settings
telegram:
  bot_token: "${TELEGRAM_BOT_TOKEN}"
  chat_id: "${TELEGRAM_CHAT_ID}"

# Defaults for new posts
defaults:
  status: draft
```

### Environment variables

Every config field can be overridden through `JTPOST_`-prefixed environment variables (nested keys joined with `_`). Priority: env > yaml > defaults.

| Variable | YAML field |
|----------|------------|
| `JTPOST_POSTS_DIR` | `posts_dir` |
| `JTPOST_TEMPLATES_DIR` | `templates_dir` |
| `JTPOST_TELEGRAM_BOT_TOKEN` | `telegram.bot_token` |
| `JTPOST_TELEGRAM_CHAT_ID` | `telegram.chat_id` |
| `JTPOST_SQLITE_DSN` | `sqlite.dsn` |
| `JTPOST_DEFAULTS_STATUS` | `defaults.status` |

### SQLite usage

```bash
# Migrate from filesystem to SQLite
jtpost migrate

# Use SQLite via flag
jtpost list --db .jtpost.db

# Or via configuration (.jtpost.yaml)
sqlite:
  dsn: .jtpost.db
```

More: [docs/sqlite.md](docs/sqlite.md).

## Development

### Build

```bash
task build:bin
# or
go build -o ./dist/jtpost ./cmd/jtpost
```

### Run

```bash
task run:cmd
# or
go run ./cmd/jtpost
```

### Tests

```bash
task test
task test:race
task test:coverage
```

### Lint

```bash
task lint
```

## Project layout

```
jtpost/
├── cmd/jtpost/           # CLI entrypoint
├── internal/
│   ├── core/             # Domain model and interfaces
│   ├── adapters/         # Implementations (fs, sqlite, git, postgres, telegram, http, webui)
│   └── cli/              # Cobra commands
├── templates/            # Post templates
├── testdata/             # Test data
├── docs/                 # Documentation
├── examples/             # Sample config + posts
├── .jtpost.db            # SQLite DB (optional)
└── .jtpost.yaml          # Project configuration
```

---

## 📊 Project stats

| Metric | Value |
|--------|-------|
| **CLI commands** | 15+ |
| **HTTP API endpoints** | 20+ |
| **Storage backends** | 4 (fs, sqlite, git, postgres) |
| **Platforms** | 1 (Telegram, with media groups) |
| **Post statuses** | 5 |
| **ID format** | UUID v7 |
| **Tests** | 100% PASS, race-clean |
| **Linter** | 0 issues |
| **Go version** | 1.25.5+ |

## Post statuses

```
idea → draft → ready → scheduled → published
```

- **idea** — early sketch, needs work
- **draft** — actively being written
- **ready** — ready to publish
- **scheduled** — scheduled for a specific time
- **published** — already out

## License

MIT

## Documentation

### Core

- **[ROADMAP.md](./ROADMAP.md)** — project roadmap (versions 0.2.0, 0.3.0, 0.4.0, 1.0.0)
- **[CHANGELOG.md](./CHANGELOG.md)** — change history
- **[CONTRIBUTING.md](./CONTRIBUTING.md)** — contribution guide
- **[docs/cli/](./docs/cli/jtpost.md)** — auto-generated CLI reference
- **[docs/api.md](./docs/api.md)** — HTTP API documentation
- **[docs/architecture.md](./docs/architecture.md)** — project architecture (Hexagonal)
- **[docs/configuration.md](./docs/configuration.md)** — configuration guide
- **[docs/development.md](./docs/development.md)** — developer guide
- **[docs/sqlite.md](./docs/sqlite.md)** — SQLite storage and migration
- **[docs/logging.md](./docs/logging.md)** — logging and middleware

### For AI assistants

- **[AGENTS.md](./AGENTS.md)** — AI assistant guide
- **[QWEN.md](./QWEN.md)** — project context for AI

---

## 📚 Further reading

- **[ROADMAP.md](./ROADMAP.md)** — detailed roadmap
- **[Effective Go](https://go.dev/doc/effective_go)** — Go style guide
- **[Cobra CLI](https://github.com/spf13/cobra)** — CLI framework
- **[htmx](https://htmx.org)** — Web UI library
- **[Telegram Bot API](https://core.telegram.org/bots/api)** — Telegram API
