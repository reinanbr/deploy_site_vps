# Changelog

## v0.1.0

Initial release.

- `init`, `dry-run`, `deploy`, `renew`, `status`, `remove`, `service` commands.
- Docker Compose orchestration (`docker compose up -d --build`).
- Nginx vhost generation (bootstrap HTTP + final TLS-terminating config), enable/disable, test, reload.
- Certificate issuance via certbot, with `dns-cloudflare` (default) and `http` challenge methods.
- Cloudflare API token resolution via `CLOUDFLARE_API_TOKEN`/`DEPLOY_SITE_CLOUDFLARE_TOKEN` env vars or `.env`, never stored in config.
- systemd timer integration (`deploy_site service install`) for automatic certificate renewal.
