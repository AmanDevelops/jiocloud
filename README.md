# jiocloud

A minimal, rclone-style Go CLI for the JioAiCloud API: **login**, **whoami**,
single-file **upload**, and one-way folder **sync**.

## Build

```bash
go build -o jiocloud ./cmd/jiocloud
```

## Login

Authentication needs a per-user cookie string in this format:

```
{{USER_ID}}:Basic {{AUTH_CODE}}:{{APP_SECRET}}:{{DEVICE_KEY}}
```

On login the tool also scrapes the web app's `main.*.js` bundle once to obtain the
default `X-Api-Key` and `X-App-Secret`, then stores everything in
`$XDG_CONFIG_HOME/jiocloud/credentials.json` (mode `0600`).

```bash
# pass the cookie inline...
jiocloud login 'da35...:Basic NDA1...:ODc0...:6e4b...'

# ...or be prompted for it
jiocloud login
```

## Upload

```bash
# upload to the root
jiocloud upload ./README.md

# upload into a specific folder
jiocloud upload ./big.deb -folder 545CA841D1BA1906E063C00B10AC6C35
```

Files under 10 MB use a single multipart request. Larger files automatically use
the chunked protocol (`initiate` + 4 MB `PUT` chunks with per-chunk `Content-MD5`),
resuming from the offset the server reports.

## Whoami

```bash
jiocloud whoami
```

Prints the logged-in user, their root folder key, and storage quota
(from `GET /security/users`).

## Sync (one-way: local -> remote)

```bash
# mirror ./photos into the remote folder "Backups/Photos"
jiocloud sync ./photos Backups/Photos

# preview without uploading or creating folders
jiocloud sync ./photos Backups/Photos -dry-run
```

`sync` walks the local directory, recreates the folder tree remotely (creating
folders as needed and caching their keys), and uploads every file that is missing
or whose content differs. A file is **skipped** when the remote folder already
contains one with the same name and the same MD5 hash. It is strictly one-way:
remote files that don't exist locally are never deleted.

Per-source state (folder keys + uploaded file hashes) is persisted under
`$XDG_CONFIG_HOME/jiocloud/sync/` so folder ids are remembered across runs.

## Layout

```
cmd/jiocloud      CLI entrypoint and command dispatch
internal/config   credential parsing + on-disk storage
internal/api      HTTP client, scraping, login, user info, folders, upload
internal/sync     one-way folder sync engine + state persistence
```
