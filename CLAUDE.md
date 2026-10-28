# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

A Go HTTP server, packaged with Docker, that acts as a Tinfoil/CyberFoil custom shop on a home LAN. It lists and serves Switch game files (`.nsp`, `.nsz`, `.xci`, `.xcz`) from a read-only mounted folder. No auth and no HTTPS by design (LAN-only); no third-party Go dependencies.

## Commands

Go may not be installed on the host; run Go commands through the Go image (`docker run --rm -v "$PWD":/src -w /src golang:1.25 <cmd>`):

```sh
go test -race ./...                                      # all tests
go test ./internal/shop -run TestServeFileSupportsRange -v   # single test
gofmt -l . && go vet ./...                               # format check + vet
make up                                                  # build & run with git version (port 8080, mounts $GAMES_DIR or /games)
```

When testing the container on macOS, the mounted folder must be under a path Docker Desktop shares (e.g. inside the project under `/Users`); the session scratchpad under `/private/tmp` mounts as an empty directory.

## Architecture

- `cmd/shop/main.go` — reads config from env (`SHOP_PORT`, `SHOP_GAMES_DIR`, `SHOP_MESSAGE`) and starts the server. Wraps the shop handler with request logging (health checks excluded) and `GET /version`. Also implements `-healthcheck`, which the Dockerfile `HEALTHCHECK` uses because the distroless runtime image has no shell or curl.
- `internal/shop` — all behavior:
  - `GET /` builds the index by walking the folder **on every request** (no cache), so new files appear without a restart. Index URLs are absolute (`http://<request Host>/files/<escaped path>`), so they match whatever address the Switch used.
  - `GET /files/{path...}` serves via `http.ServeContent`, which provides Range support that Tinfoil relies on for resumed and multi-part downloads.
  - Path safety rests on `os.Root`: every open, stat and walk goes through it, which blocks `..` and symlinks that escape the folder. Keep new file access on `s.root`, never `os.Open`. Dotfiles and dot-directories are skipped; only game extensions are listed or served.
- Versioning: `main.version` is set via `-ldflags -X` from the `VERSION` build arg. `.dockerignore` excludes `.git`, so the `Makefile` computes `git describe --tags --always --dirty` on the host and compose passes it in; without it the version is `dev`.
- Git flow: feature branch → PR into `development` → PR into `main`. Both are protected by repository rulesets (no direct pushes, CI checks `test` and `docker` required), so never commit to them directly.
- Docker: multi-stage build (`golang:1.25` → `distroless/static-debian12:nonroot`). `docker-compose.yml` maps `${SHOP_PORT}` → 8080 and mounts `${GAMES_DIR:-/games}` at `/games:ro`. Settings live in `.env` (see `.env.example`).
