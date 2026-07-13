package tests

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/reinanbr/deploy_site/autodeploy"
)

func configPathIn(dir string) string {
	return filepath.Join(dir, "config_deploy_site.json")
}

// ─── CmdInit ─────────────────────────────────

func TestCmdInit_CreatesConfig(t *testing.T) {
	dir := t.TempDir()
	cfgPath := configPathIn(dir)

	if err := autodeploy.CmdInit(cfgPath); err != nil {
		t.Fatalf("CmdInit failed: %v", err)
	}
	if _, err := os.Stat(cfgPath); err != nil {
		t.Fatal("config file should exist after CmdInit")
	}
}

func TestCmdInit_ConfigHasExpectedFields(t *testing.T) {
	dir := t.TempDir()
	cfgPath := configPathIn(dir)

	if err := autodeploy.CmdInit(cfgPath); err != nil {
		t.Fatalf("CmdInit failed: %v", err)
	}

	data, _ := os.ReadFile(cfgPath)
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	for _, field := range []string{
		"domain", "upstream_host", "upstream_port", "compose_file",
		"certbot_email", "challenge_method", "nginx_sites_available",
		"nginx_sites_enabled", "force_ssl_redirect",
	} {
		if _, ok := raw[field]; !ok {
			t.Errorf("expected field %q in generated config", field)
		}
	}
	if raw["challenge_method"] != "dns-cloudflare" {
		t.Errorf("challenge_method = %v, want dns-cloudflare", raw["challenge_method"])
	}
	if _, hasToken := raw["cloudflare_api_token"]; hasToken {
		t.Error("generated config must not contain cloudflare_api_token")
	}
}

func TestCmdInit_DetectsExistingComposeFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "compose.yaml"), []byte("services: {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	cfgPath := configPathIn(dir)
	if err := autodeploy.CmdInit(cfgPath); err != nil {
		t.Fatalf("CmdInit failed: %v", err)
	}
	data, _ := os.ReadFile(cfgPath)
	var raw map[string]any
	json.Unmarshal(data, &raw)
	if raw["compose_file"] != "compose.yaml" {
		t.Errorf("compose_file = %v, want compose.yaml", raw["compose_file"])
	}
}

func TestCmdInit_FailsIfConfigExists(t *testing.T) {
	dir := t.TempDir()
	cfgPath := configPathIn(dir)
	if err := autodeploy.CmdInit(cfgPath); err != nil {
		t.Fatalf("first CmdInit failed: %v", err)
	}
	if err := autodeploy.CmdInit(cfgPath); err == nil {
		t.Fatal("expected error when config already exists")
	}
}

// ─── CmdStatus ───────────────────────────────

func TestCmdStatus_DoesNotPanicWithoutCertOrSite(t *testing.T) {
	dir := t.TempDir()
	cfgPath := configPathIn(dir)
	if err := autodeploy.CmdInit(cfgPath); err != nil {
		t.Fatalf("CmdInit failed: %v", err)
	}
	// point nginx dirs at temp dirs so os.Stat/Lstat calls stay local
	data, _ := os.ReadFile(cfgPath)
	var raw map[string]any
	json.Unmarshal(data, &raw)
	raw["domain"] = "status-test.invalid"
	raw["nginx_sites_available"] = filepath.Join(dir, "sites-available")
	raw["nginx_sites_enabled"] = filepath.Join(dir, "sites-enabled")
	out, _ := json.Marshal(raw)
	os.WriteFile(cfgPath, out, 0644)

	if err := autodeploy.CmdStatus(cfgPath); err != nil {
		t.Fatalf("CmdStatus failed: %v", err)
	}
}

// ─── CmdRemove ───────────────────────────────

func TestCmdRemove_NoOpWhenNotEnabled(t *testing.T) {
	dir := t.TempDir()
	cfgPath := configPathIn(dir)
	if err := autodeploy.CmdInit(cfgPath); err != nil {
		t.Fatalf("CmdInit failed: %v", err)
	}
	data, _ := os.ReadFile(cfgPath)
	var raw map[string]any
	json.Unmarshal(data, &raw)
	raw["domain"] = "remove-test.invalid"
	raw["nginx_sites_available"] = filepath.Join(dir, "sites-available")
	raw["nginx_sites_enabled"] = filepath.Join(dir, "sites-enabled")
	out, _ := json.Marshal(raw)
	os.WriteFile(cfgPath, out, 0644)

	cfg, err := autodeploy.LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	// DisableSite must be a safe no-op when the site was never enabled.
	if err := autodeploy.DisableSite(cfg); err != nil {
		t.Fatalf("DisableSite on unenabled site failed: %v", err)
	}
}
