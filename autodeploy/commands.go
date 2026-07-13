package autodeploy

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const defaultServiceName = "deploy_site-renew"

func CmdInit(cfgPath string) error {
	cfgPath = ResolveConfigPath(cfgPath)
	if _, err := os.Stat(cfgPath); err == nil {
		return fmt.Errorf("config already exists: %s", cfgPath)
	}
	dir := filepath.Dir(cfgPath)
	cf := configFile{
		Domain:                       "",
		UpstreamHost:                 "127.0.0.1",
		UpstreamPort:                 8080,
		ComposeFile:                  detectComposeFile(dir),
		ComposeWorkdir:               "",
		ComposeService:               "",
		CertbotEmail:                 "",
		ChallengeMethod:              "dns-cloudflare",
		CloudflarePropagationSeconds: 30,
		NginxSitesAvailable:          "/etc/nginx/sites-available",
		NginxSitesEnabled:            "/etc/nginx/sites-enabled",
		ClientMaxBodySize:            "10m",
		ForceSSLRedirect:             true,
		ExtraNginxDirectives:         "",
		LogFile:                      "deploy_site.log",
	}
	if err := writeConfig(cfgPath, cf); err != nil {
		return err
	}
	fmt.Printf("Created %s\n", cfgPath)
	fmt.Printf("  compose_file : %s\n", cf.ComposeFile)
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  1. Edit domain, upstream_port and certbot_email in the config")
	fmt.Println("  2. Make sure the domain's DNS record already points to this VPS in Cloudflare")
	fmt.Println("  3. Set your Cloudflare API token (zone:DNS:edit permission):")
	fmt.Println("       echo 'CLOUDFLARE_API_TOKEN=xxxx' >> .env")
	fmt.Println("       echo '.env' >> .gitignore")
	fmt.Println("  4. deploy_site dry-run")
	fmt.Println("  5. deploy_site deploy")
	return nil
}

func CmdDryRun(cfgPath string) error {
	cfgPath = ResolveConfigPath(cfgPath)
	cfg, err := LoadConfig(cfgPath)
	if err != nil {
		return fmt.Errorf("config error: %w", err)
	}
	workdir := composeWorkdir(cfg, cfgPath)

	fmt.Println("=== dry-run: checking configuration ===")
	fmt.Printf("  config   : %s\n", cfgPath)
	fmt.Printf("  domain   : %s\n", orMissing(cfg.Domain))
	fmt.Printf("  upstream : %s:%d\n", cfg.UpstreamHost, cfg.UpstreamPort)
	fmt.Printf("  challenge: %s\n", cfg.ChallengeMethod)

	var failed bool
	check := func(label string, err error) {
		if err != nil {
			fmt.Printf("  %-9s: FAIL — %v\n", label, err)
			failed = true
			return
		}
		fmt.Printf("  %-9s: OK\n", label)
	}

	check("domain", requireNonEmpty(cfg.Domain, "domain is required"))
	check("email", requireNonEmpty(cfg.CertbotEmail, "certbot_email is required"))
	check("docker", DockerAvailable())
	check("compose", ComposeFileExists(workdir, cfg))
	check("nginx", NginxAvailable())
	check("certbot", CertbotAvailable())

	if cfg.ChallengeMethod == "dns-cloudflare" {
		check("cf-token", requireNonEmpty(cfg.CloudflareToken, "CLOUDFLARE_API_TOKEN not set (env or .env)"))
	}

	if cfg.Domain != "" {
		if ips, err := net.LookupHost(cfg.Domain); err != nil {
			fmt.Printf("  dns      : FAIL — %v\n", err)
			failed = true
		} else {
			fmt.Printf("  dns      : resolves to %s\n", strings.Join(ips, ", "))
			fmt.Println("             (with Cloudflare proxy on, this is a Cloudflare edge IP — that's expected)")
		}
	}

	if _, err := os.Stat(cfg.NginxSitesAvailable); err != nil {
		fmt.Printf("  nginx-dir: FAIL — %v\n", err)
		failed = true
	} else {
		fmt.Printf("  nginx-dir: %s exists\n", cfg.NginxSitesAvailable)
	}

	if CertExists(cfg.Domain) {
		fmt.Printf("  cert     : already issued (%s)\n", LiveCertPath(cfg.Domain))
	} else {
		fmt.Printf("  cert     : not issued yet — 'deploy_site deploy' will request one\n")
	}

	if failed {
		return fmt.Errorf("dry-run found problems — fix them before running 'deploy_site deploy'")
	}
	fmt.Println("=== dry-run passed — ready to run 'deploy_site deploy' ===")
	return nil
}

func CmdDeploy(cfgPath string) error {
	cfgPath = ResolveConfigPath(cfgPath)
	cfg, err := LoadConfig(cfgPath)
	if err != nil {
		return fmt.Errorf("config error: %w", err)
	}
	if err := requireNonEmpty(cfg.Domain, "domain is required in config"); err != nil {
		return err
	}
	if err := requireNonEmpty(cfg.CertbotEmail, "certbot_email is required in config"); err != nil {
		return err
	}

	l, err := newLogger(logPath(cfg, cfgPath))
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	defer l.close()

	workdir := composeWorkdir(cfg, cfgPath)

	l.info(fmt.Sprintf("deploying %s (upstream %s:%d, challenge %s)", cfg.Domain, cfg.UpstreamHost, cfg.UpstreamPort, cfg.ChallengeMethod))

	if err := ComposeFileExists(workdir, cfg); err != nil {
		l.errLog(err.Error())
		return err
	}

	l.info("running docker compose up -d --build")
	out, err := ComposeUp(workdir, cfg)
	if out != "" {
		l.raw(out)
	}
	if err != nil {
		l.errLog(fmt.Sprintf("docker compose up failed: %v", err))
		return fmt.Errorf("docker compose up failed: %w", err)
	}
	l.ok("containers are up")

	if err := EnsureWebroot(); err != nil {
		l.warn(fmt.Sprintf("could not create webroot: %v", err))
	}

	if !CertExists(cfg.Domain) {
		l.info("no certificate yet — installing bootstrap nginx site")
		if err := installSite(cfg, RenderBootstrapConfig(cfg), l); err != nil {
			return err
		}

		l.info(fmt.Sprintf("requesting certificate via %s", cfg.ChallengeMethod))
		out, err := ObtainCertificate(cfg)
		if out != "" {
			l.raw(out)
		}
		if err != nil {
			l.errLog(fmt.Sprintf("certbot failed: %v", err))
			return fmt.Errorf("certbot failed: %w", err)
		}
		l.ok("certificate issued")
	} else {
		l.info("certificate already exists — skipping issuance (use 'deploy_site renew' to renew)")
	}

	l.info("installing final nginx site (TLS termination)")
	if err := installSite(cfg, RenderSSLConfig(cfg), l); err != nil {
		return err
	}

	l.ok(fmt.Sprintf("deploy complete — https://%s", cfg.Domain))
	return nil
}

// installSite writes the vhost, (re)enables it, tests, and reloads nginx.
func installSite(cfg *Config, content string, l *logger) error {
	path, err := WriteSiteConfig(cfg, content)
	if err != nil {
		l.errLog(fmt.Sprintf("failed to write nginx config: %v", err))
		return fmt.Errorf("failed to write nginx config: %w", err)
	}
	l.info(fmt.Sprintf("wrote %s", path))

	if err := EnableSite(cfg); err != nil {
		l.errLog(fmt.Sprintf("failed to enable site: %v", err))
		return fmt.Errorf("failed to enable site: %w", err)
	}

	if out, err := TestConfig(); err != nil {
		l.errLog(fmt.Sprintf("nginx -t failed: %v\n%s", err, out))
		return fmt.Errorf("nginx config test failed: %w\n%s", err, out)
	}

	if out, err := Reload(); err != nil {
		l.errLog(fmt.Sprintf("nginx reload failed: %v\n%s", err, out))
		return fmt.Errorf("nginx reload failed: %w", err)
	}
	l.ok("nginx reloaded")
	return nil
}

func CmdRenew(cfgPath string) error {
	cfgPath = ResolveConfigPath(cfgPath)
	cfg, err := LoadConfig(cfgPath)
	if err != nil {
		return fmt.Errorf("config error: %w", err)
	}

	l, err := newLogger(logPath(cfg, cfgPath))
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	defer l.close()

	l.info("running certbot renew")
	out, err := RenewAll()
	if out != "" {
		l.raw(out)
	}
	if err != nil {
		l.errLog(fmt.Sprintf("certbot renew failed: %v", err))
		return fmt.Errorf("certbot renew failed: %w", err)
	}

	if out, err := Reload(); err != nil {
		l.warn(fmt.Sprintf("nginx reload after renew failed: %v\n%s", err, out))
	} else {
		l.ok("nginx reloaded")
	}

	l.ok("renew check complete")
	return nil
}

func CmdStatus(cfgPath string) error {
	cfgPath = ResolveConfigPath(cfgPath)
	cfg, err := LoadConfig(cfgPath)
	if err != nil {
		return err
	}
	workdir := composeWorkdir(cfg, cfgPath)

	fmt.Printf("Config    : %s\n", cfgPath)
	fmt.Printf("Domain    : %s\n", orMissing(cfg.Domain))
	fmt.Printf("Upstream  : %s:%d\n", cfg.UpstreamHost, cfg.UpstreamPort)
	fmt.Printf("Challenge : %s\n", cfg.ChallengeMethod)

	fmt.Println("Nginx     :")
	if SiteEnabled(cfg) {
		fmt.Printf("  site    : enabled (%s)\n", SiteEnabledPath(cfg))
	} else {
		fmt.Printf("  site    : not enabled\n")
	}

	fmt.Println("Certificate:")
	if CertExists(cfg.Domain) {
		expiry, err := CertificateExpiry(cfg.Domain)
		if err != nil {
			fmt.Printf("  expiry  : failed to read (%v)\n", err)
		} else {
			days := int(time.Until(expiry).Hours() / 24)
			fmt.Printf("  expiry  : %s (%d days left)\n", expiry.Format(time.RFC3339), days)
		}
	} else {
		fmt.Printf("  status  : not issued\n")
	}

	fmt.Println("Docker    :")
	if out, err := ComposePs(workdir, cfg); err != nil {
		fmt.Printf("  ps      : failed — %v\n", err)
	} else {
		for _, line := range strings.Split(out, "\n") {
			fmt.Printf("  %s\n", line)
		}
	}

	return nil
}

func CmdRemove(cfgPath string) error {
	cfgPath = ResolveConfigPath(cfgPath)
	cfg, err := LoadConfig(cfgPath)
	if err != nil {
		return err
	}
	if err := DisableSite(cfg); err != nil {
		return fmt.Errorf("failed to disable site: %w", err)
	}
	if out, err := Reload(); err != nil {
		return fmt.Errorf("nginx reload failed: %w\n%s", err, out)
	}
	fmt.Printf("disabled nginx site for %s (config, containers and certificate left untouched)\n", cfg.Domain)
	return nil
}

func requireNonEmpty(v, msg string) error {
	if strings.TrimSpace(v) == "" {
		return fmt.Errorf("%s", msg)
	}
	return nil
}

func orMissing(v string) string {
	if strings.TrimSpace(v) == "" {
		return "(not set)"
	}
	return v
}

func runCommand(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

func ServiceUnitContent(execPath, cfgPath string) string {
	return fmt.Sprintf(`[Unit]
Description=deploy_site certificate renewal

[Service]
Type=oneshot
ExecStart=%q renew %q
`, execPath, cfgPath)
}

func TimerUnitContent() string {
	return `[Unit]
Description=Run deploy_site certificate renewal twice a day

[Timer]
OnCalendar=*-*-* 03,15:00:00
RandomizedDelaySec=1800
Persistent=true

[Install]
WantedBy=timers.target
`
}

// CmdService manages a systemd timer that periodically runs 'deploy_site renew'.
func CmdService(args []string) error {
	if runtime.GOOS != "linux" {
		return fmt.Errorf("service command is only available on Linux")
	}
	if len(args) == 0 {
		return fmt.Errorf("usage: deploy_site service <install|uninstall|start|stop|restart|status|logs> [config|lines]")
	}

	action := args[0]
	servicePath := filepath.Join("/etc/systemd/system", defaultServiceName+".service")
	timerPath := filepath.Join("/etc/systemd/system", defaultServiceName+".timer")

	switch action {
	case "install":
		cfgPath := ""
		if len(args) > 1 {
			cfgPath = args[1]
		}
		cfgPath = ResolveConfigPath(cfgPath)
		if _, err := os.Stat(cfgPath); err != nil {
			return fmt.Errorf("config file not found: %s", cfgPath)
		}

		execPath, err := os.Executable()
		if err != nil {
			return fmt.Errorf("cannot determine executable path: %w", err)
		}
		execPath, _ = filepath.Abs(execPath)

		if err := os.WriteFile(servicePath, []byte(ServiceUnitContent(execPath, cfgPath)), 0644); err != nil {
			return fmt.Errorf("failed to write %s (try with sudo): %w", servicePath, err)
		}
		if err := os.WriteFile(timerPath, []byte(TimerUnitContent()), 0644); err != nil {
			return fmt.Errorf("failed to write %s (try with sudo): %w", timerPath, err)
		}

		if out, err := runCommand("systemctl", "daemon-reload"); err != nil {
			return fmt.Errorf("systemctl daemon-reload failed: %v\n%s", err, out)
		}
		if out, err := runCommand("systemctl", "enable", "--now", defaultServiceName+".timer"); err != nil {
			return fmt.Errorf("systemctl enable --now failed: %v\n%s", err, out)
		}

		fmt.Printf("installed and started systemd timer: %s.timer\n", defaultServiceName)
		return nil

	case "uninstall":
		if out, err := runCommand("systemctl", "disable", "--now", defaultServiceName+".timer"); err != nil {
			fmt.Fprintf(os.Stderr, "warning: disable failed: %v\n%s\n", err, out)
		}
		for _, p := range []string{servicePath, timerPath} {
			if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("failed removing %s: %w", p, err)
			}
		}
		if out, err := runCommand("systemctl", "daemon-reload"); err != nil {
			return fmt.Errorf("systemctl daemon-reload failed: %v\n%s", err, out)
		}
		fmt.Printf("uninstalled systemd timer: %s.timer\n", defaultServiceName)
		return nil

	case "start", "stop", "restart", "status":
		out, err := runCommand("systemctl", action, defaultServiceName+".timer")
		if out != "" {
			fmt.Println(out)
		}
		if action == "status" {
			return nil
		}
		if err != nil {
			return fmt.Errorf("systemctl %s failed: %v", action, err)
		}
		fmt.Printf("timer %s: %s\n", defaultServiceName, action)
		return nil

	case "logs":
		lines := "50"
		if len(args) > 1 {
			if _, err := strconv.Atoi(args[1]); err != nil {
				return fmt.Errorf("logs expects a numeric line count")
			}
			lines = args[1]
		}
		out, err := runCommand("journalctl", "-u", defaultServiceName+".service", "-n", lines, "--no-pager")
		if err != nil {
			return fmt.Errorf("journalctl failed: %v\n%s", err, out)
		}
		fmt.Println(out)
		return nil

	default:
		return fmt.Errorf("unknown service action: %s", action)
	}
}
