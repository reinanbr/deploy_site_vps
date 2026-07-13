package main

import (
	"fmt"
	"os"

	"github.com/reinanbr/deploy_site/autodeploy"
)

var version = "v0.1.2"

const usage = `Usage: deploy_site <command> [config_path]

Commands:
  init             create config_deploy_site.json for the current project
  dry-run          validate config, tooling and DNS without changing anything
  deploy           docker compose up, generate nginx vhost, obtain TLS cert via certbot
  renew            run certbot renew and reload nginx (meant for cron/systemd timer)
  status           show container, nginx and certificate status
  remove           disable the nginx site (config, containers, cert untouched)
  service ...      systemd timer integration (install/uninstall/start/stop/status/logs)
  --version, -v    print version

Cloudflare token (required for the default dns-cloudflare challenge):
  Set CLOUDFLARE_API_TOKEN in environment or in .env next to the config
  (zone:DNS:edit permission scoped to the domain's zone).
  Never put the token in config_deploy_site.json.
`

func main() {
	args := os.Args[1:]

	if len(args) == 0 {
		fmt.Print(usage)
		os.Exit(1)
	}

	switch args[0] {
	case "--version", "-v":
		fmt.Println("deploy_site", version)
		return

	case "--help", "-h":
		fmt.Print(usage)
		return

	case "init":
		cfg := ""
		if len(args) > 1 {
			cfg = args[1]
		}
		if err := autodeploy.CmdInit(cfg); err != nil {
			fmt.Fprintf(os.Stderr, "init: %v\n", err)
			os.Exit(1)
		}
		return

	case "dry-run":
		cfg := ""
		if len(args) > 1 {
			cfg = args[1]
		}
		if err := autodeploy.CmdDryRun(cfg); err != nil {
			fmt.Fprintf(os.Stderr, "dry-run: %v\n", err)
			os.Exit(1)
		}
		return

	case "deploy":
		cfg := ""
		if len(args) > 1 {
			cfg = args[1]
		}
		if err := autodeploy.CmdDeploy(cfg); err != nil {
			fmt.Fprintf(os.Stderr, "deploy: %v\n", err)
			os.Exit(1)
		}
		return

	case "renew":
		cfg := ""
		if len(args) > 1 {
			cfg = args[1]
		}
		if err := autodeploy.CmdRenew(cfg); err != nil {
			fmt.Fprintf(os.Stderr, "renew: %v\n", err)
			os.Exit(1)
		}
		return

	case "status":
		cfg := ""
		if len(args) > 1 {
			cfg = args[1]
		}
		if err := autodeploy.CmdStatus(cfg); err != nil {
			fmt.Fprintf(os.Stderr, "status: %v\n", err)
			os.Exit(1)
		}
		return

	case "remove":
		cfg := ""
		if len(args) > 1 {
			cfg = args[1]
		}
		if err := autodeploy.CmdRemove(cfg); err != nil {
			fmt.Fprintf(os.Stderr, "remove: %v\n", err)
			os.Exit(1)
		}
		return

	case "service":
		if err := autodeploy.CmdService(args[1:]); err != nil {
			fmt.Fprintf(os.Stderr, "service: %v\n", err)
			os.Exit(1)
		}
		return
	}

	fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", args[0])
	fmt.Print(usage)
	os.Exit(1)
}
