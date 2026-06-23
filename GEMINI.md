# Project Context: jiocloud

A minimal, rclone-style Go CLI for the JioAiCloud API. It supports authentication, user information retrieval, listing files, making directories, single-file uploads (with automatic chunking for large files), single-file downloads, one-way directory synchronization (mirroring), full stateful sync, and file/folder deletion.

## Project Overview

- **Language:** Go (1.26.2+)
- **Purpose:** Provide a lightweight command-line interface for interacting with JioAiCloud.
- **Architecture:**
  - `cmd/jiocloud`: CLI entry point and command-line argument parsing.
  - `internal/api`: Core API client implementation. It includes logic for scraping default API keys from the JioAiCloud web application's JavaScript bundles.
  - `internal/config`: Manages credential storage and configuration using XDG-compliant directories.
  - `internal/copier`: Implements the one-way mirroring logic, including state persistence for efficient subsequent runs.

## Key Workflows

### 1. Building and Running

- **Build:**
  ```bash
  go build -o jiocloud ./cmd/jiocloud
  ```
- **Test:**
  ```bash
  go test ./...
  ```

### 2. Authentication

The tool requires a specific cookie format for login:
`{{USER_ID}}:Basic {{AUTH_CODE}}:{{APP_SECRET}}:{{DEVICE_KEY}}`

On login, it automatically scrapes the web app for `X-Api-Key` and `X-App-Secret`. Credentials are saved to `$XDG_CONFIG_HOME/jiocloud/credentials.json` with `0600` permissions.

### 3. File Operations

- **List:** Lists files and directories in a given remote path. Defaults to root if no path is provided.
- **Make Directory:** Creates a new directory at the specified remote path, creating intermediate folders if necessary.
- **Upload:** Automatically switches between single multipart requests (for files < 10 MB) and a chunked protocol (4 MB chunks) for larger files.
- **Download:** Downloads a single remote file or an entire remote folder recursively to the local filesystem.
- **Delete:** Moves a remote file or folder to the trash using the path.
- **Copy:** Performs a one-way sync from local to remote. It uses MD5 hashes to skip identical files and persists folder keys and file hashes in `$XDG_CONFIG_HOME/jiocloud/copy/`.
- **Sync:** Like copy, but also deletes remote files and folders that are no longer present locally.

## Development Conventions

- **Standard Toolchain:** Use standard Go tools (`go build`, `go test`, `go fmt`).
- **Configuration:** Always use `internal/config` for accessing credentials or configuration paths.
- **API Client:** Use `internal/api` for all interactions with JioAiCloud.
- **Releases:** Handled via GitHub Actions (`.github/workflows/release.yml`) triggered by pushing a version tag (e.g., `v1.0.0`).
- **Permissions:** Ensure sensitive files (like credentials) are handled with appropriate file permissions.

## Key Files

- `cmd/jiocloud/main.go`: Main dispatcher.
- `internal/api/client.go`: API client structure and common logic.
- `internal/api/upload.go`: Single-shot and chunked upload implementation.
- `internal/api/scrape.go`: Logic for extracting API keys from the web app.
- `internal/copier/copy.go`: Main loop for the directory sync operation.
- `install.sh`: Installer script for Linux systems.
