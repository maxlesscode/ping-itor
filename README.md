# ping-itor

A lightweight web uptime monitor. Add URLs from the dashboard, get a live status feed without reloading, and receive email alerts when a site goes down or recovers.

## Features

- Checks every monitored URL every 60 seconds with a 10-second per-request timeout
- HTTP 2xx–3xx responses are counted as up; anything else (4xx, 5xx, network error) is down
- Down alert fires after **2 consecutive failures**; recovery alert fires on the next successful check
- Status updates stream to all open browser tabs via SSE — no polling, no page reload
- Add and delete monitors directly from the dashboard (HTMX partial updates)
- SQLite persistence via `modernc.org/sqlite` — pure Go, no CGO, no system libraries required
- Email alerts over SMTP with STARTTLS (works with Resend, Postmark, Gmail SMTP, or any standard SMTP relay)
- Single static binary; Docker Compose deployment included

## Getting started

### Prerequisites

- Go 1.26+
- [`templ`](https://templ.guide/) CLI — required to regenerate templates before building

```bash
go install github.com/a-h/templ/cmd/templ@latest
```

### Development

```bash
# 1. Clone the repository
git clone https://github.com/maxlesscode/ping-itor.git
cd ping-itor

# 2. Generate templates (must be run after any template change)
templ generate

# 3. Copy and edit environment variables
cp .env.example .env
# Edit .env — see Configuration reference below

# 4. Run
go run ./cmd/ping-itor
```

The server listens on `http://localhost:8080` by default.

Email alerts are **optional**. If `SMTP_TO` is not set, the application starts normally and skips all alert delivery. If `SMTP_TO` is set, `SMTP_HOST`, `SMTP_USER`, `SMTP_PASS`, and `SMTP_FROM` are all required — the process exits on startup if any are missing.

### Production (Docker Compose)

```bash
# 1. Create a .env file with your secrets (never committed)
cat > .env <<'EOF'
RESEND_API_KEY=re_xxxxxxxxxxxx
SMTP_FROM=noreply@yourdomain.com
SMTP_TO=you@email.com
EOF

# 2. Build and start
docker compose up -d --build

# 3. Tail logs
docker compose logs -f
```

The SQLite database is stored in a named Docker volume (`ping-itor-data`) mounted at `/data/ping-itor.db` inside the container. Data survives container restarts and image rebuilds.

To inject a version string into the binary at build time:

```bash
VERSION=v1.2.3 docker compose up -d --build
```

## Configuration reference

All configuration is via environment variables. The process validates values on startup and exits immediately if required variables are missing.

| Variable        | Default             | Required                      | Description                                                             |
|-----------------|---------------------|-------------------------------|-------------------------------------------------------------------------|
| `HTTP_ADDR`     | `:8080`             | No                            | TCP address the HTTP server listens on                                  |
| `DATABASE_PATH` | `./ping-itor.db`    | No                            | Path to the SQLite database file                                        |
| `SMTP_TO`       | —                   | No                            | Recipient address for alerts. Omit to disable all email notifications   |
| `SMTP_HOST`     | —                   | If `SMTP_TO` is set           | SMTP server hostname (e.g. `smtp.resend.com`)                           |
| `SMTP_PORT`     | `587`               | No                            | SMTP server port (STARTTLS)                                             |
| `SMTP_USER`     | —                   | If `SMTP_TO` is set           | SMTP username                                                           |
| `SMTP_PASS`     | —                   | If `SMTP_TO` is set           | SMTP password or API key                                                |
| `SMTP_FROM`     | —                   | If `SMTP_TO` is set           | Sender address used in the `From` header                                |

### Using Resend as the SMTP relay

The included `docker-compose.yml` is pre-configured for [Resend](https://resend.com):

| Compose variable | Value            |
|------------------|------------------|
| `SMTP_HOST`      | `smtp.resend.com` |
| `SMTP_PORT`      | `587`             |
| `SMTP_USER`      | `resend`          |
| `SMTP_PASS`      | your Resend API key (`RESEND_API_KEY` from `.env`) |

Any SMTP relay that supports STARTTLS on port 587 works as a drop-in replacement — update `SMTP_HOST`, `SMTP_USER`, and `SMTP_PASS` accordingly.

## Stack

| Layer       | Technology                                                |
|-------------|-----------------------------------------------------------|
| Language    | Go 1.26                                                   |
| Router      | [chi v5](https://github.com/go-chi/chi)                   |
| Templates   | [templ v0.3](https://templ.guide/)                        |
| Frontend    | [HTMX 2.0.4](https://htmx.org) + htmx-ext-sse 2.2.2      |
| Styling     | Tailwind CSS (CDN)                                        |
| Database    | SQLite via [modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite) (pure Go) |
| Alerts      | Standard library `net/smtp` with STARTTLS                 |

## License

No license file is present in this repository. All rights reserved by default until a license is added.
