# my-switch-shop

A small self-hosted custom shop for [Tinfoil](https://tinfoil.io) and CyberFoil on
your home network. It serves the `.nsp`, `.nsz`, `.xci` and `.xcz` files in a
folder (including subfolders) as a shop index the Switch can browse and install from.

## Run it

```sh
cp .env.example .env    # optional: change the games folder, port or message
make up                 # docker compose up -d --build, stamped with the git version
```

By default it serves the host folder `/games` (mounted read-only) on port `8080`.
New files show up the next time the Switch loads the shop; no restart needed.

| Variable       | Default                     | Meaning                                    |
| -------------- | --------------------------- | ------------------------------------------ |
| `GAMES_DIR`    | `/games`                    | Host folder with your game files           |
| `SHOP_PORT`    | `8080`                      | Host port the Switch connects to           |
| `SHOP_MESSAGE` | `Welcome to my Switch shop` | Message shown when the shop loads          |

On macOS/Windows with Docker Desktop, `GAMES_DIR` must be in a folder Docker
Desktop shares (Settings → Resources → File sharing), otherwise the shop is empty.

## Connect the Switch

Find the server's LAN IP (e.g. `192.168.1.20`) and add a shop:

- **Tinfoil:** File Browser → `-` (new) → Protocol `http`, Host `192.168.1.20`,
  Port `8080`, Path `/`. No username or password.
- **CyberFoil:** add the shop URL `http://192.168.1.20:8080/`.

## Endpoints

- `GET /` — shop index (JSON: `files[]` with `url` and `size`, plus `success` message)
- `GET /files/<path>` — download a game file (supports HTTP Range / resume)
- `GET /healthz` — health check
- `GET /version` — build version

## Versioning

The version comes from git tags: `make up` passes `git describe --tags --always --dirty`
(e.g. `v0.1.0`, `v0.1.0-3-gabc1234`, `…-dirty`) into the build. A plain
`docker compose up --build` reports `dev`. Check what is running with
`curl http://<host>:8080/version` or the first line of `docker compose logs`.
To release, tag a commit (`git tag v0.2.0`) and run `make up`.

## Branches and CI

- Work happens on feature branches, merged into `development` by pull request;
  `development` is merged into `main` by pull request for releases.
- Nobody can push directly to `main` or `development`; pull requests need the
  `test` and `docker` CI checks to pass.
- CI (`.github/workflows/ci.yml`) runs gofmt, `go vet`, `go test -race` and a
  Docker build on every push and pull request.
- Release (`.github/workflows/release.yml`): pushing a `v*` tag publishes
  `ghcr.io/ryansese/my-switch-shop:<version>` for amd64 and arm64.

## Develop

Requires Go 1.25+, or use the Go Docker image if Go isn't installed:

```sh
go test ./...
# or
docker run --rm -v "$PWD":/src -w /src golang:1.25 go test ./...
```
