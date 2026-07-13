package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/reinanbr/deploy_site/autodeploy"
)

func testConfig(t *testing.T, domain string) *autodeploy.Config {
	t.Helper()
	dir := t.TempDir()
	available := filepath.Join(dir, "sites-available")
	enabled := filepath.Join(dir, "sites-enabled")
	if err := os.MkdirAll(available, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(enabled, 0755); err != nil {
		t.Fatal(err)
	}
	return &autodeploy.Config{
		Domain:              domain,
		UpstreamHost:        "127.0.0.1",
		UpstreamPort:        8080,
		NginxSitesAvailable: available,
		NginxSitesEnabled:   enabled,
		ClientMaxBodySize:   "10m",
		ForceSSLRedirect:    true,
	}
}

func TestRenderBootstrapConfig(t *testing.T) {
	cfg := testConfig(t, "app.example.com")
	out := autodeploy.RenderBootstrapConfig(cfg)
	if !strings.Contains(out, "server_name app.example.com;") {
		t.Errorf("expected server_name directive, got:\n%s", out)
	}
	if !strings.Contains(out, "proxy_pass http://127.0.0.1:8080;") {
		t.Errorf("expected proxy_pass to upstream, got:\n%s", out)
	}
	if !strings.Contains(out, "/.well-known/acme-challenge/") {
		t.Errorf("expected acme-challenge location, got:\n%s", out)
	}
	if strings.Contains(out, "listen 443") {
		t.Errorf("bootstrap config must not listen on 443, got:\n%s", out)
	}
}

func TestRenderSSLConfig_RedirectsWhenForced(t *testing.T) {
	cfg := testConfig(t, "app.example.com")
	cfg.ForceSSLRedirect = true
	out := autodeploy.RenderSSLConfig(cfg)
	if !strings.Contains(out, "return 301 https://$host$request_uri;") {
		t.Errorf("expected https redirect, got:\n%s", out)
	}
	if !strings.Contains(out, "listen 443 ssl http2;") {
		t.Errorf("expected TLS listen directive, got:\n%s", out)
	}
	if !strings.Contains(out, "ssl_certificate     /etc/letsencrypt/live/app.example.com/fullchain.pem;") {
		t.Errorf("expected cert path, got:\n%s", out)
	}
}

func TestRenderSSLConfig_NoRedirectServesUpstreamOnHTTP(t *testing.T) {
	cfg := testConfig(t, "app.example.com")
	cfg.ForceSSLRedirect = false
	out := autodeploy.RenderSSLConfig(cfg)
	if strings.Contains(out, "return 301") {
		t.Errorf("expected no redirect, got:\n%s", out)
	}
	if !strings.Contains(out, "proxy_pass http://127.0.0.1:8080;") {
		t.Errorf("expected http proxy_pass to remain, got:\n%s", out)
	}
}

func TestRenderSSLConfig_ExtraDirectives(t *testing.T) {
	cfg := testConfig(t, "app.example.com")
	cfg.ExtraNginxDirectives = "add_header X-Test yes;"
	out := autodeploy.RenderSSLConfig(cfg)
	if !strings.Contains(out, "add_header X-Test yes;") {
		t.Errorf("expected extra directive, got:\n%s", out)
	}
}

func TestWriteSiteConfig(t *testing.T) {
	cfg := testConfig(t, "app.example.com")
	path, err := autodeploy.WriteSiteConfig(cfg, "server {}\n")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read written config: %v", err)
	}
	if string(data) != "server {}\n" {
		t.Errorf("got %q, want server {}\\n", string(data))
	}
}

func TestEnableDisableSite(t *testing.T) {
	cfg := testConfig(t, "app.example.com")
	if _, err := autodeploy.WriteSiteConfig(cfg, "server {}\n"); err != nil {
		t.Fatal(err)
	}
	if err := autodeploy.EnableSite(cfg); err != nil {
		t.Fatalf("EnableSite failed: %v", err)
	}
	if !autodeploy.SiteEnabled(cfg) {
		t.Error("expected site to be enabled")
	}
	if err := autodeploy.DisableSite(cfg); err != nil {
		t.Fatalf("DisableSite failed: %v", err)
	}
	if autodeploy.SiteEnabled(cfg) {
		t.Error("expected site to be disabled")
	}
}

func TestEnableSite_ReplacesStaleSymlink(t *testing.T) {
	cfg := testConfig(t, "app.example.com")
	if _, err := autodeploy.WriteSiteConfig(cfg, "server { listen 80; }\n"); err != nil {
		t.Fatal(err)
	}
	if err := autodeploy.EnableSite(cfg); err != nil {
		t.Fatal(err)
	}
	// enabling again (e.g. redeploy) must not fail on the existing symlink
	if err := autodeploy.EnableSite(cfg); err != nil {
		t.Fatalf("second EnableSite failed: %v", err)
	}
}

func TestCertExists(t *testing.T) {
	if autodeploy.CertExists("nonexistent.invalid") {
		t.Error("expected CertExists to be false for a domain with no certificate")
	}
}
