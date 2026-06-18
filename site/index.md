---
title: jiocloud
---

**jiocloud** is a small, [rclone](https://rclone.org/)-style command-line tool for
the JioAiCloud API. It can authenticate with your account, list files, create directories, upload and download single files,
do one-way (local → remote) folder copy, sync, and delete files/folders.

> Unofficial. Not affiliated with or endorsed by Jio. Use with your own account.

## Install (Linux)

```sh
curl -fsSL https://raw.githubusercontent.com/AmanDevelops/jiocloud/main/install.sh | sh
```

This downloads the latest release for your architecture (`amd64`/`arm64`), verifies
its checksum, and installs `jiocloud` into `~/.local/bin`. If that directory isn't on
your `PATH`, the script tells you exactly what to add (or offers to add it when run
interactively).

Install a specific version or location:

```sh
JIOCLOUD_VERSION=v1.0.0 JIOCLOUD_INSTALL_DIR=/usr/local/bin \
  sh -c "$(curl -fsSL https://raw.githubusercontent.com/AmanDevelops/jiocloud/main/install.sh)"
```

Or build from source (Go 1.26+):

```sh
go install github.com/AmanDevelops/jiocloud/cmd/jiocloud@latest
```

## Authenticate

Authentication uses a single **cookie string** copied from a logged-in web session:

```
{{USER_ID}}:Basic {{AUTH_CODE}}:{{APP_SECRET}}:{{DEVICE_KEY}}
```

### Getting the string

1. Log in at [jioaicloud.com](https://www.jioaicloud.com/) in your browser.
2. Open DevTools → **Network**, trigger any action (e.g. open a folder), and inspect
   a request to `jmng2-api.jioaicloud.com`. From its request headers read:
   - `X-User-Id` → `{{USER_ID}}`
   - `Authorization: Basic …` → the `Basic {{AUTH_CODE}}` part
   - `X-App-Secret` → `{{APP_SECRET}}`
   - `X-Device-Key` → `{{DEVICE_KEY}}`
3. Join them with colons in the order shown above.

### Logging in

```sh
# paste inline...
jiocloud login '{{USER_ID}}:Basic {{AUTH_CODE}}:{{APP_SECRET}}:{{DEVICE_KEY}}'

# ...or be prompted (input not echoed to history)
jiocloud login
```

On login, `jiocloud` also scrapes the public `X-Api-Key` from the web app once, then
stores everything under `~/.config/jiocloud/credentials.json` (mode `0600`). Your app
secret is taken **only** from your cookie — never a baked-in default.

Confirm it worked:

```sh
jiocloud whoami
# User:    Your Name (xxxx…)
# Root:    548484EB…
# Storage: 9059260 / 53687091200 bytes used
```

## Usage

### Upload a single file

```sh
# into the account root
jiocloud upload ./report.pdf

# into a specific folder (by its objectKey)
jiocloud upload ./big.iso -folder 545CA841D1BA1906E063C00B10AC6C35
```

Files under 10 MB use one multipart request; larger files automatically switch to the
chunked protocol and resume from the server-reported offset.

### Copy a folder (one-way)

```sh
# mirror ./photos into a remote folder "Backups/Photos"
jiocloud copy ./photos Backups/Photos

# preview only — no folders created, nothing uploaded
jiocloud copy ./photos Backups/Photos -dry-run
```

`copy` walks your local directory, recreates the tree as remote folders (created on
demand, their keys cached), and uploads every file that is missing or whose contents
changed. A file is **skipped** when the remote folder already has one with the same
name and the same MD5. Output marks each file:

```
+ photos/2024/img001.jpg (2.1 MB)   uploaded
= photos/2024/img002.jpg            unchanged, skipped
```

It is strictly one-way: **remote-only files are never deleted.** Per-source state
(folder keys + uploaded hashes) is kept under `~/.config/jiocloud/copy/`, so re-runs
remember the folders they created.

## Command reference

| Command | Description |
|---------|-------------|
| `jiocloud login [cookie]` | Authenticate; prompts if the cookie is omitted. |
| `jiocloud whoami` | Show the user, root folder key, and storage quota. |
| `jiocloud ls [remotePath]` | List files and directories (defaults to root). |
| `jiocloud mkdir <remotePath>` | Make the path if it doesn't already exist. |
| `jiocloud upload <file> [-folder KEY]` | Upload one file (auto small/chunked). |
| `jiocloud delete <remotePath>` | Move a file or folder to the trash. |
| `jiocloud copy <dir> [remotePath] [-dry-run]` | One-way local → remote folder copy. |
| `jiocloud sync <dir> [remotePath] [-dry-run]` | Like copy, but deletes remote files/folders not present locally. |
| `jiocloud version` | Print the version. |

## Links

- Source: [github.com/AmanDevelops/jiocloud](https://github.com/AmanDevelops/jiocloud)
- Releases: [github.com/AmanDevelops/jiocloud/releases](https://github.com/AmanDevelops/jiocloud/releases)
b.com/AmanDevelops/jiocloud/releases)
