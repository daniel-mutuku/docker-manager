# Docker Manager

A self-hosted, single-binary web UI for managing Docker containers on a single host written in Go.

## Status

Implemented:

- **Auth**: session-based login, single admin user, bcrypt password hash.
- **Containers**: list, inspect, start/stop/restart/remove, with a live-polling table.
- **Logs**: live-follow or tailed log viewer, streamed to the browser over Server-Sent Events.
- **Stats**: live CPU%, memory, and network I/O per container, streamed over SSE.
- **Images**: list, pull by name/tag with streamed progress, remove, prune dangling images.
- **Volumes**: list (with which containers mount each one), create, remove.
- **Networks**: list, inspect (connected containers), create, remove.
- **Stacks**: paste or upload a `docker-compose.yml`, save it, deploy/tear down via `docker compose`, view live container status per stack.
- **Dashboard**: running/stopped container counts, image count, and image disk usage.

## Requirements

- Go 1.24+ (only needed to build from source)
- Access to a Docker daemon socket (typically `/var/run/docker.sock`)
- For the Stacks feature specifically: the `docker` CLI with the Compose plugin available on `PATH` (already included in the provided `Dockerfile`)

## Configuration

Configuration is read from flags, environment variables, or a `.env` file in the working directory (flags win, then real env vars, then `.env`).

| Env var               | Flag                  | Default                          | Description                                  |
|------------------------|-----------------------|-----------------------------------|-----------------------------------------------|
| `LISTEN_ADDR`          | `-listen`             | `:8080`                          | Address the HTTP server listens on           |
| `DOCKER_HOST_PATH`     | `-docker-host`        | `unix:///var/run/docker.sock`    | Docker daemon socket/host                    |
| `DB_PATH`              | `-db`                 | `data/docker-manager.db`         | SQLite database file path                    |
| `STACKS_DIR`           | `-stacks-dir`         | `data/stacks`                    | Directory Compose stack files are written under |
| `ADMIN_USERNAME`       | `-admin-username`     | `admin`                          | Login username                               |
| `ADMIN_PASSWORD`       | `-admin-password`     | —                                 | Login password (hashed in memory at startup) |
| `ADMIN_PASSWORD_HASH`  | `-admin-password-hash`| —                                 | Bcrypt hash of the login password (preferred over `ADMIN_PASSWORD`) |

You must set either `ADMIN_PASSWORD` or `ADMIN_PASSWORD_HASH`. For a quick local setup `ADMIN_PASSWORD` is fine (it's hashed in memory at startup and never stored in plaintext); for anything long-lived, precompute a bcrypt hash and set `ADMIN_PASSWORD_HASH` instead so the plaintext isn't sitting in a config file.

Sessions are opaque random tokens stored server-side in SQLite (looked up by ID, not decoded or trusted), so there's no session-signing secret to configure.

## Running locally

```bash
cp .env.example .env   # then edit values
go run ./cmd/server
```

Visit `http://localhost:8080` and log in with the configured admin credentials.

## Running with Docker

The app manages the *host's* Docker daemon, so its own container needs the host socket mounted in. `docker-compose.yml` reads `ADMIN_USERNAME` and `ADMIN_PASSWORD`/`ADMIN_PASSWORD_HASH` from a `.env` file placed next to it (Compose substitutes `${VAR}` references from there automatically — this is the same `.env` used for local `go run`):

```bash
cp .env.example .env   # then edit values
docker compose up -d --build
```

This builds the image, starts the manager on port 8080, and mounts `/var/run/docker.sock` from the host. The app refuses to start if neither `ADMIN_PASSWORD` nor `ADMIN_PASSWORD_HASH` is set.

## Project layout

```
cmd/server/          entrypoint: config load, wiring, graceful shutdown
internal/config/     flag + env + .env config loading
internal/docker/     Docker Engine SDK wrapper (containers, images, volumes, networks, logs, stats)
internal/compose/    shells out to `docker compose` to deploy/tear down saved stacks
internal/store/      SQLite persistence: sessions, saved compose stacks
internal/web/        HTTP layer: router, middleware, handlers, template rendering
web/templates/       html/template pages and htmx partials (embedded via embed.FS)
web/static/          CSS, vendored htmx.min.js, and small plain-JS SSE/fetch clients (embedded via embed.FS)
```

The binary embeds all templates and static assets, so a single compiled executable is fully self-contained.

## Testing

```bash
go test ./... -race -cover
```

The `web` package's handlers are tested against a fake `DockerService` (see `internal/web/server_test.go`), so the suite never needs a live Docker daemon or network access. The `compose` package's tests substitute a fake command runner for the same reason, so CI never invokes a real `docker` binary. CI (`.github/workflows/ci.yml`) runs `go build`, `go vet`, a `gofmt` check, and the full test suite with the race detector on every push and pull request.

## Security notes

- **Docker socket access is root-equivalent.** Anyone who can reach this app can control every container on the host, including ones with host mounts or privileged flags. Treat the admin credentials accordingly and don't expose this beyond a trusted network without a reverse proxy adding its own auth/TLS layer.
- Login is timing-safe: an unknown username still runs a full bcrypt comparison against a dummy hash, so response time can't be used to enumerate valid usernames.
- Session cookies are `HttpOnly`, `SameSite=Lax`, and marked `Secure` automatically whenever the request arrived over TLS (directly or via `X-Forwarded-Proto`).
- Baseline security headers (`X-Content-Type-Options`, `X-Frame-Options`, a same-origin `Content-Security-Policy`) are set on every response.
- Sessions are opaque, randomly generated, server-side-looked-up tokens (not signed/decoded client-side data), with a 24h TTL; a background job purges expired rows from SQLite hourly.
- Log lines and image-pull progress are inserted into the page as text nodes (`textContent`/`createTextNode`), never `innerHTML` — a container that prints `<script>...</script>` to its own stdout, or a registry that returns crafted pull-progress JSON, can't execute anything in your browser.
- Stack names are validated against a strict allowlist (`^[a-z0-9][a-z0-9_-]{0,62}$`) before being used as both a `docker compose` project name and a filesystem directory name, and `docker compose` is invoked with an explicit argument list (never a shell string), which together rule out path traversal and argument/command injection from a stack name.
- **Compose stacks and bind mounts**: because the app shells out to `docker compose` from inside its own container (talking to the *host's* daemon over the mounted socket), any bind-mount paths in a stack's `docker-compose.yml` are resolved by the host daemon, not by this container's filesystem. Stick to named volumes in stacks deployed this way, or mount the same host path into the manager's container that your stack's bind mounts expect.

## License

MIT — see [LICENSE](LICENSE).
