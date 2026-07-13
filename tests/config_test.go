package tests

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/reinanbr/auto_deploy_go/autodeploy"
)

// ─── ResolveConfigPath ───────────────────────

func TestResolveConfigPath_EmptyUsesDefault(t *testing.T) {
	got := autodeploy.ResolveConfigPath("")
	if !strings.HasSuffix(got, "config_auto_deploy.json") {
		t.Errorf("got %q, want suffix config_auto_deploy.json", got)
	}
}

func TestResolveConfigPath_ReturnsAbsolute(t *testing.T) {
	got := autodeploy.ResolveConfigPath("relative.json")
	if !filepath.IsAbs(got) {
		t.Errorf("expected absolute path, got %q", got)
	}
}

// ─── NormalizeChallengeMethod ────────────────

func TestNormalizeChallengeMethod(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", "dns-cloudflare"},
		{"dns-cloudflare", "dns-cloudflare"},
		{"DNS-CLOUDFLARE", "dns-cloudflare"},
		{"http", "http"},
		{" HTTP ", "http"},
		{"invalid", "dns-cloudflare"},
	}
	for _, c := range cases {
		got := autodeploy.NormalizeChallengeMethod(c.in)
		if got != c.want {
			t.Errorf("NormalizeChallengeMethod(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// ─── LoadDotEnvToken ─────────────────────────

func TestLoadDotEnvToken_CLOUDFLARE_API_TOKEN(t *testing.T) {
	dir := t.TempDir()
	writeEnvFile(t, dir, "CLOUDFLARE_API_TOKEN=mytoken\n")
	got := autodeploy.LoadDotEnvToken(dir)
	if got != "mytoken" {
		t.Errorf("got %q, want %q", got, "mytoken")
	}
}

func TestLoadDotEnvToken_AUTODEPLOY_CLOUDFLARE_TOKEN(t *testing.T) {
	dir := t.TempDir()
	writeEnvFile(t, dir, "AUTODEPLOY_CLOUDFLARE_TOKEN=alt\n")
	got := autodeploy.LoadDotEnvToken(dir)
	if got != "alt" {
		t.Errorf("got %q, want %q", got, "alt")
	}
}

func TestLoadDotEnvToken_QuotedValue(t *testing.T) {
	dir := t.TempDir()
	writeEnvFile(t, dir, `CLOUDFLARE_API_TOKEN="quoted"`+"\n")
	got := autodeploy.LoadDotEnvToken(dir)
	if got != "quoted" {
		t.Errorf("got %q, want %q", got, "quoted")
	}
}

func TestLoadDotEnvToken_CommentsIgnored(t *testing.T) {
	dir := t.TempDir()
	writeEnvFile(t, dir, "# CLOUDFLARE_API_TOKEN=nope\nAUTODEPLOY_CLOUDFLARE_TOKEN=real\n")
	got := autodeploy.LoadDotEnvToken(dir)
	if got != "real" {
		t.Errorf("got %q, want %q", got, "real")
	}
}

func TestLoadDotEnvToken_MissingFile(t *testing.T) {
	if got := autodeploy.LoadDotEnvToken(t.TempDir()); got != "" {
		t.Errorf("expected empty for missing .env, got %q", got)
	}
}

// ─── LoadConfig ──────────────────────────────

func TestLoadConfig_Defaults(t *testing.T) {
	dir := t.TempDir()
	path := writeConfig(t, dir, map[string]any{"domain": "example.com"})
	cfg, err := autodeploy.LoadConfig(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.UpstreamHost != "127.0.0.1" {
		t.Errorf("UpstreamHost = %q, want 127.0.0.1", cfg.UpstreamHost)
	}
	if cfg.ComposeFile != "docker-compose.yml" {
		t.Errorf("ComposeFile = %q, want docker-compose.yml", cfg.ComposeFile)
	}
	if cfg.ChallengeMethod != "dns-cloudflare" {
		t.Errorf("ChallengeMethod = %q, want dns-cloudflare", cfg.ChallengeMethod)
	}
	if cfg.CloudflarePropagationSeconds != 30 {
		t.Errorf("CloudflarePropagationSeconds = %d, want 30", cfg.CloudflarePropagationSeconds)
	}
	if cfg.ClientMaxBodySize != "10m" {
		t.Errorf("ClientMaxBodySize = %q, want 10m", cfg.ClientMaxBodySize)
	}
	if cfg.LogFile != "auto_deploy.log" {
		t.Errorf("LogFile = %q, want auto_deploy.log", cfg.LogFile)
	}
}

func TestLoadConfig_CustomValues(t *testing.T) {
	dir := t.TempDir()
	path := writeConfig(t, dir, map[string]any{
		"domain":          "app.example.com",
		"upstream_port":   9090,
		"challenge_method": "http",
	})
	cfg, err := autodeploy.LoadConfig(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Domain != "app.example.com" {
		t.Errorf("Domain = %q, want app.example.com", cfg.Domain)
	}
	if cfg.UpstreamPort != 9090 {
		t.Errorf("UpstreamPort = %d, want 9090", cfg.UpstreamPort)
	}
	if cfg.ChallengeMethod != "http" {
		t.Errorf("ChallengeMethod = %q, want http", cfg.ChallengeMethod)
	}
}

func TestLoadConfig_RejectsCloudflareToken(t *testing.T) {
	dir := t.TempDir()
	path := writeConfig(t, dir, map[string]any{
		"domain":               "example.com",
		"cloudflare_api_token": "secret",
	})
	_, err := autodeploy.LoadConfig(path)
	if err == nil || !strings.Contains(err.Error(), "cloudflare_api_token") {
		t.Fatalf("expected cloudflare_api_token error, got %v", err)
	}
}

func TestLoadConfig_TokenFromDotEnv(t *testing.T) {
	dir := t.TempDir()
	path := writeConfig(t, dir, map[string]any{"domain": "example.com"})
	writeEnvFile(t, dir, "CLOUDFLARE_API_TOKEN=envfiletoken\n")
	cfg, err := autodeploy.LoadConfig(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.CloudflareToken != "envfiletoken" {
		t.Errorf("CloudflareToken = %q, want envfiletoken", cfg.CloudflareToken)
	}
}

func TestLoadConfig_TokenFromEnvVar(t *testing.T) {
	dir := t.TempDir()
	path := writeConfig(t, dir, map[string]any{"domain": "example.com"})
	t.Setenv("CLOUDFLARE_API_TOKEN", "envvartoken")
	cfg, err := autodeploy.LoadConfig(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.CloudflareToken != "envvartoken" {
		t.Errorf("CloudflareToken = %q, want envvartoken", cfg.CloudflareToken)
	}
}

func TestLoadConfig_TokenNotInJSON(t *testing.T) {
	dir := t.TempDir()
	path := writeConfig(t, dir, map[string]any{"domain": "example.com"})
	data, _ := os.ReadFile(path)
	if strings.Contains(string(data), "cloudflare_api_token") || strings.Contains(string(data), "CloudflareToken") {
		t.Error("config JSON must not contain token field")
	}
}

func TestLoadConfig_MissingFile(t *testing.T) {
	_, err := autodeploy.LoadConfig("/nonexistent/path/config.json")
	if err == nil {
		t.Fatal("expected error for missing config file")
	}
}

// ─── helpers ─────────────────────────────────

func writeConfig(t *testing.T, dir string, content map[string]any) string {
	t.Helper()
	data, err := json.Marshal(content)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "config_auto_deploy.json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func writeEnvFile(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
}
