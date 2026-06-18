# jiocloud

A minimal Go CLI for the JioAiCloud API, supporting **login** and **file upload**.

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

## Layout

```
cmd/jiocloud      CLI entrypoint and command dispatch
internal/config   credential parsing + on-disk storage
internal/api      HTTP client, credential scraping, login, upload
```
