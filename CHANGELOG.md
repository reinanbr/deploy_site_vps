# Changelog

## v0.1.2

- `deploy` now detects an already-running stack (`docker compose ps -q`) and
  runs `docker compose down` before `up -d --build`, instead of building on
  top of containers that may already be up.
- Fixed a regression from v0.1.1: `status` was streaming `docker compose ps`
  output live and then reprinting it, showing it twice. `ComposePs` is back
  to a quiet (non-streamed) call since `status` formats and prints it itself.

## v0.1.1

- `deploy`/`renew` now stream `docker compose` and `certbot` output live to
  stdout as it happens, instead of buffering everything until the command
  exits — long builds and DNS propagation waits no longer look like they've
  hung.
- The log file still gets the full captured output, just without the
  now-redundant duplicate stdout print.

## v0.1.0

Initial release.

- `init`, `dry-run`, `deploy`, `renew`, `status`, `remove`, `service` commands.
- Docker Compose orchestration (`docker compose up -d --build`).
- Nginx vhost generation (bootstrap HTTP + final TLS-terminating config), enable/disable, test, reload.
- Certificate issuance via certbot, with `dns-cloudflare` (default) and `http` challenge methods.
- Cloudflare API token resolution via `CLOUDFLARE_API_TOKEN`/`DEPLOY_SITE_CLOUDFLARE_TOKEN` env vars or `.env`, never stored in config.
- systemd timer integration (`deploy_site service install`) for automatic certificate renewal.
