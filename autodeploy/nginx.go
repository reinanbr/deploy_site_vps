package autodeploy

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// DefaultWebroot is where the http-01 challenge files are served from.
const DefaultWebroot = "/var/www/certbot"

func runCommandCombined(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

// NginxAvailable checks that the nginx CLI is on PATH.
func NginxAvailable() error {
	if _, err := exec.LookPath("nginx"); err != nil {
		return fmt.Errorf("'nginx' not found on PATH")
	}
	return nil
}

func EnsureWebroot() error {
	return os.MkdirAll(DefaultWebroot, 0755)
}

// LiveCertPath returns the path certbot writes the fullchain to for domain.
func LiveCertPath(domain string) string {
	return filepath.Join("/etc/letsencrypt/live", domain, "fullchain.pem")
}

// LiveKeyPath returns the path certbot writes the private key to for domain.
func LiveKeyPath(domain string) string {
	return filepath.Join("/etc/letsencrypt/live", domain, "privkey.pem")
}

func CertExists(domain string) bool {
	_, err := os.Stat(LiveCertPath(domain))
	return err == nil
}

func siteFileName(domain string) string {
	return domain + ".conf"
}

func SiteAvailablePath(cfg *Config) string {
	return filepath.Join(cfg.NginxSitesAvailable, siteFileName(cfg.Domain))
}

func SiteEnabledPath(cfg *Config) string {
	return filepath.Join(cfg.NginxSitesEnabled, siteFileName(cfg.Domain))
}

// RenderBootstrapConfig is the HTTP-only vhost installed before a certificate
// exists. It proxies straight to the upstream service and exposes the
// acme-challenge path for the http-01 method — harmless (and unused) when
// challenge_method is dns-cloudflare.
func RenderBootstrapConfig(cfg *Config) string {
	return fmt.Sprintf(`server {
    listen 80;
    listen [::]:80;
    server_name %s;

    location /.well-known/acme-challenge/ {
        root %s;
    }

    location / {
        proxy_pass http://%s:%d;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
`, cfg.Domain, DefaultWebroot, cfg.UpstreamHost, cfg.UpstreamPort)
}

// RenderSSLConfig is the final vhost installed once a certificate exists:
// HTTP either redirects to HTTPS or keeps proxying (force_ssl_redirect),
// and HTTPS terminates TLS and proxies to the upstream service.
func RenderSSLConfig(cfg *Config) string {
	var httpServer string
	if cfg.ForceSSLRedirect {
		httpServer = fmt.Sprintf(`server {
    listen 80;
    listen [::]:80;
    server_name %s;

    location /.well-known/acme-challenge/ {
        root %s;
    }

    location / {
        return 301 https://$host$request_uri;
    }
}

`, cfg.Domain, DefaultWebroot)
	} else {
		httpServer = RenderBootstrapConfig(cfg) + "\n"
	}

	extra := ""
	if strings.TrimSpace(cfg.ExtraNginxDirectives) != "" {
		extra = "\n    " + strings.TrimSpace(cfg.ExtraNginxDirectives) + "\n"
	}

	sslServer := fmt.Sprintf(`server {
    listen 443 ssl http2;
    listen [::]:443 ssl http2;
    server_name %s;

    ssl_certificate     %s;
    ssl_certificate_key %s;
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers HIGH:!aNULL:!MD5;
    ssl_prefer_server_ciphers off;

    client_max_body_size %s;
%s
    location / {
        proxy_pass http://%s:%d;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
    }
}
`, cfg.Domain, LiveCertPath(cfg.Domain), LiveKeyPath(cfg.Domain), cfg.ClientMaxBodySize, extra, cfg.UpstreamHost, cfg.UpstreamPort)

	return httpServer + sslServer
}

func WriteSiteConfig(cfg *Config, content string) (string, error) {
	path := SiteAvailablePath(cfg)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return "", err
	}
	return path, nil
}

// EnableSite symlinks the site into sites-enabled. On layouts where both
// paths are the same directory (e.g. a plain conf.d/ include) this is a no-op.
func EnableSite(cfg *Config) error {
	available := SiteAvailablePath(cfg)
	enabled := SiteEnabledPath(cfg)
	if available == enabled {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(enabled), 0755); err != nil {
		return err
	}
	_ = os.Remove(enabled)
	return os.Symlink(available, enabled)
}

// DisableSite removes the sites-enabled symlink, leaving sites-available and
// certificates untouched.
func DisableSite(cfg *Config) error {
	available := SiteAvailablePath(cfg)
	enabled := SiteEnabledPath(cfg)
	if available == enabled {
		return nil
	}
	err := os.Remove(enabled)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func SiteEnabled(cfg *Config) bool {
	_, err := os.Lstat(SiteEnabledPath(cfg))
	return err == nil
}

func TestConfig() (string, error) {
	return runCommandCombined("nginx", "-t")
}

func Reload() (string, error) {
	if out, err := runCommandCombined("systemctl", "reload", "nginx"); err == nil {
		return out, nil
	}
	return runCommandCombined("nginx", "-s", "reload")
}
