# deploy_site

[![CI](https://github.com/reinanbr/deploy_site/actions/workflows/ci.yml/badge.svg)](https://github.com/reinanbr/deploy_site/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/reinanbr/deploy_site.svg)](https://pkg.go.dev/github.com/reinanbr/deploy_site)

[![Go Version](https://img.shields.io/github/go-mod/go-version/reinanbr/deploy_site)](go.mod)
[![Release](https://img.shields.io/github/v/release/reinanbr/deploy_site?sort=semver)](https://github.com/reinanbr/deploy_site/releases)
[![Platform](https://img.shields.io/badge/platform-linux-informational?logo=linux&logoColor=white)](README.md)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

Deploys a Docker Compose project on a VPS behind Nginx with a Let's Encrypt certificate — for a domain whose DNS already lives in Cloudflare.

`deploy_site deploy` does, in order: `docker compose up -d --build`, generate and install an Nginx vhost, request a TLS certificate via certbot, and reload Nginx.

Pure Go · zero dependencies · Linux · systemd-ready

---

## How it fits together

```
Cloudflare (DNS, proxy optional) ──▶ VPS:80/443 ──▶ Nginx ──▶ 127.0.0.1:<upstream_port> ──▶ your container
                                         ▲
                                     certbot (TLS cert, dns-cloudflare or http challenge)
```

- **Docker** builds and starts your `docker-compose.yml` service(s).
- **Nginx** terminates TLS and reverse-proxies to the container's published port.
- **certbot** issues and renews the certificate. By default it uses the **`dns-cloudflare`**
  challenge (via the Cloudflare API), which works whether the domain's Cloudflare record is
  proxied (orange cloud) or DNS-only (grey cloud) — HTTP-01 challenges can be finicky with the
  proxy on, DNS-01 sidesteps that entirely.

---

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/reinanbr/deploy_site/main/install.sh | sh
```

Requires on the VPS: `docker` (with the compose plugin), `nginx`, `certbot`, and, for the default
challenge, `certbot`'s Cloudflare DNS plugin:

```bash
sudo apt install -y docker.io docker-compose-plugin nginx certbot python3-certbot-dns-cloudflare
```

Build from source (Go 1.21+):

```bash
go build -o deploy_site .
```

Run tests:

```bash
go test ./...
```

---

## Quick start

```bash
cd /path/to/your/docker-compose/project
deploy_site init        # generates config_deploy_site.json
```

Edit `config_deploy_site.json`: set `domain`, `upstream_port` (the port your container publishes
on `127.0.0.1`), and `certbot_email`.

Set the Cloudflare API token (a scoped token with **Zone → DNS → Edit** on the domain's zone —
not the Global API Key):

```bash
echo 'CLOUDFLARE_API_TOKEN=xxxxxxxxxxxx' >> .env
echo '.env' >> .gitignore
```

Then:

```bash
deploy_site dry-run      # validate tooling, config, DNS, token — no changes made
deploy_site deploy       # compose up, nginx vhost, certbot, reload
```

Your app is now served at `https://<domain>`.

---

## Configuration

`deploy_site init` creates `config_deploy_site.json` next to your compose file.

```json
{
  "domain": "app.example.com",
  "upstream_host": "127.0.0.1",
  "upstream_port": 8080,
  "compose_file": "docker-compose.yml",
  "compose_workdir": "",
  "compose_service": "",
  "certbot_email": "you@example.com",
  "challenge_method": "dns-cloudflare",
  "cloudflare_propagation_seconds": 30,
  "nginx_sites_available": "/etc/nginx/sites-available",
  "nginx_sites_enabled": "/etc/nginx/sites-enabled",
  "client_max_body_size": "10m",
  "force_ssl_redirect": true,
  "extra_nginx_directives": "",
  "log_file": "deploy_site.log"
}
```

| Field | Default | Description |
|---|---|---|
| `domain` | — | Domain to serve *(required)*, must already resolve to this VPS in Cloudflare |
| `upstream_host` | `127.0.0.1` | Where the container publishes its port |
| `upstream_port` | — | Container's published port *(required)* |
| `compose_file` | `docker-compose.yml` | Compose file to build/run |
| `compose_workdir` | config's directory | Working directory for `docker compose` |
| `compose_service` | — | Limit `up`/`ps` to a single service (empty = whole stack) |
| `certbot_email` | — | Email registered with Let's Encrypt *(required)* |
| `challenge_method` | `dns-cloudflare` | `dns-cloudflare` or `http` |
| `cloudflare_propagation_seconds` | `30` | Wait time for the DNS-01 TXT record to propagate |
| `nginx_sites_available` / `nginx_sites_enabled` | Debian/Ubuntu defaults | Adjust for other distros (e.g. `/etc/nginx/conf.d` for both, on RHEL-style layouts) |
| `client_max_body_size` | `10m` | Nginx upload size cap |
| `force_ssl_redirect` | `true` | Redirect HTTP → HTTPS instead of also serving plain HTTP |
| `extra_nginx_directives` | — | Raw directives injected into the HTTPS `server` block |
| `log_file` | `deploy_site.log` | Log file for `deploy`/`renew` |

**`cloudflare_api_token` is not a valid field.** Tokens belong in the environment.

---

## Authentication

The Cloudflare API token is required for the `dns-cloudflare` challenge:

```bash
# environment variable (preferred)
export CLOUDFLARE_API_TOKEN=xxxxxxxxxxxx

# or: .env file next to the config (never commit this)
echo 'CLOUDFLARE_API_TOKEN=xxxxxxxxxxxx' >> .env
echo '.env' >> .gitignore
```

Resolution order: `CLOUDFLARE_API_TOKEN` → `DEPLOY_SITE_CLOUDFLARE_TOKEN` → `.env` next to the config.
Tokens set in `config_deploy_site.json` are rejected at startup.

Create the token at **Cloudflare dashboard → My Profile → API Tokens → Create Token → Edit zone DNS**,
scoped to the specific zone.

---

## Usage

```
deploy_site <command> [config_path]
```

| Command | Description |
|---|---|
| `init` | Scaffold `config_deploy_site.json` for the current project |
| `dry-run` | Validate config, tooling (`docker`/`nginx`/`certbot`), DNS resolution and the Cloudflare token — no changes made |
| `deploy` | `docker compose up -d --build`, install the Nginx vhost, obtain the TLS cert (first run only), reload Nginx |
| `renew` | Run `certbot renew` and reload Nginx — meant to run on a schedule |
| `status` | Show container, Nginx site and certificate status |
| `remove` | Disable the Nginx site (config file, containers and certificate are left untouched) |
| `service <action>` | Manage a systemd timer for automatic renewal (`install`, `uninstall`, `start`, `stop`, `restart`, `status`, `logs [N]`) |
| `--version` | Print version |
| `--help` | Print this reference |

Config path can be passed as the last argument to any command:

```bash
deploy_site deploy /etc/deploy_site/config_deploy_site.json
deploy_site status /etc/deploy_site/config_deploy_site.json
```

### What `deploy` actually does

```
1. docker compose -f <compose_file> up -d --build [service]
2. if no certificate yet:
     install an HTTP-only nginx vhost proxying straight to the container
     nginx -t && reload nginx
     certbot certonly (dns-cloudflare or http, per challenge_method)
3. install the final nginx vhost (TLS termination + reverse proxy)
   nginx -t && reload nginx
```

Deploy is idempotent: re-running it rebuilds the container and re-applies the Nginx config, but
skips certificate issuance if one already exists (use `renew` for that).

### Automatic renewal

Let's Encrypt certificates last 90 days. Install a systemd timer that runs `deploy_site renew`
twice a day (a no-op unless renewal is actually due):

```bash
sudo deploy_site service install /etc/deploy_site/config_deploy_site.json

deploy_site service status
deploy_site service logs 100
sudo deploy_site service stop
sudo deploy_site service uninstall
```

Or wire it into cron yourself:

```cron
0 3,15 * * * /usr/local/bin/deploy_site renew /etc/deploy_site/config_deploy_site.json
```

`service` subcommands are Linux-only and generally require root.

---

## Choosing a challenge method

| | `dns-cloudflare` (default) | `http` |
|---|---|---|
| Works with Cloudflare proxy (orange cloud) on | ✅ | ⚠️ usually works, but breaks if SSL/TLS mode or a Cloudflare rule interferes with `/.well-known/acme-challenge/` |
| Works with Cloudflare DNS-only (grey cloud) | ✅ | ✅ |
| Requires | Cloudflare API token (`Zone:DNS:Edit`) | Port 80 reachable from the internet, no token |
| Also issues wildcard certs | ✅ | ❌ |

If the domain is proxied through Cloudflare, `dns-cloudflare` is the safer default and is what
`deploy_site init` scaffolds. Switch `challenge_method` to `"http"` only for domains kept
DNS-only, or where you'd rather not hand certbot an API token.

---

## Nginx layout notes

- `nginx_sites_available` / `nginx_sites_enabled` default to the Debian/Ubuntu layout
  (`/etc/nginx/sites-available` symlinked into `/etc/nginx/sites-enabled`).
- On distros that use a single `conf.d`/`vhost.d` directory (no separate available/enabled split),
  set both fields to the same path — `deploy_site` skips the symlink step when they're equal.
- `extra_nginx_directives` is injected verbatim into the HTTPS `server` block — useful for
  `proxy_read_timeout`, custom headers, rate limiting, etc.

---

## Files created at runtime

| File | Description |
|---|---|
| `<nginx_sites_available>/<domain>.conf` | The generated vhost |
| `/etc/letsencrypt/live/<domain>/` | Certificate and key (managed by certbot) |
| `deploy_site.log` | Log for `deploy`/`renew` (path set by `log_file`) |

---

## Development

Project layout (Go 1.21+, module `github.com/reinanbr/deploy_site`):

```
deploy_site/
├── main.go              — CLI entry point and command routing
├── autodeploy/           — core package (package autodeploy)
│   ├── config.go        — Config type, LoadConfig, Cloudflare token resolution
│   ├── logger.go        — structured logger with rotation
│   ├── docker.go        — docker compose up/ps helpers
│   ├── nginx.go          — vhost rendering, enable/disable, test, reload
│   ├── certbot.go        — certificate issuance (dns-cloudflare/http), renew, expiry
│   └── commands.go       — CmdInit, CmdDryRun, CmdDeploy, CmdRenew, CmdStatus, CmdRemove, CmdService
└── tests/                — external test suite (package tests)
    ├── config_test.go
    ├── nginx_test.go
    ├── certbot_test.go
    └── commands_test.go
```

Build:

```bash
go build -o deploy_site .
```

Test:

```bash
go test ./...
```

Coverage:

```bash
go test ./... -coverprofile=cover.out && go tool cover -func=cover.out
```

---

## License

MIT
