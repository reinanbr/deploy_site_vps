package autodeploy

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	Domain                       string `json:"domain"`
	UpstreamHost                 string `json:"upstream_host"`
	UpstreamPort                 int    `json:"upstream_port"`
	ComposeFile                  string `json:"compose_file"`
	ComposeWorkdir               string `json:"compose_workdir"`
	ComposeService               string `json:"compose_service"`
	CertbotEmail                 string `json:"certbot_email"`
	ChallengeMethod              string `json:"challenge_method"`
	CloudflarePropagationSeconds int    `json:"cloudflare_propagation_seconds"`
	NginxSitesAvailable          string `json:"nginx_sites_available"`
	NginxSitesEnabled            string `json:"nginx_sites_enabled"`
	ClientMaxBodySize            string `json:"client_max_body_size"`
	ForceSSLRedirect             bool   `json:"force_ssl_redirect"`
	ExtraNginxDirectives         string `json:"extra_nginx_directives"`
	LogFile                      string `json:"log_file"`

	// CloudflareToken is intentionally excluded from JSON serialization.
	// Set via CLOUDFLARE_API_TOKEN (or AUTODEPLOY_CLOUDFLARE_TOKEN) env var,
	// or a .env file in compose_workdir.
	CloudflareToken string `json:"-"`
}

// configFile is the on-disk representation — mirrors Config but omits the token.
type configFile struct {
	Domain                       string `json:"domain"`
	UpstreamHost                 string `json:"upstream_host"`
	UpstreamPort                 int    `json:"upstream_port"`
	ComposeFile                  string `json:"compose_file"`
	ComposeWorkdir               string `json:"compose_workdir"`
	ComposeService               string `json:"compose_service"`
	CertbotEmail                 string `json:"certbot_email"`
	ChallengeMethod              string `json:"challenge_method"`
	CloudflarePropagationSeconds int    `json:"cloudflare_propagation_seconds"`
	NginxSitesAvailable          string `json:"nginx_sites_available"`
	NginxSitesEnabled            string `json:"nginx_sites_enabled"`
	ClientMaxBodySize            string `json:"client_max_body_size"`
	ForceSSLRedirect             bool   `json:"force_ssl_redirect"`
	ExtraNginxDirectives         string `json:"extra_nginx_directives"`
	LogFile                      string `json:"log_file"`
}

func LoadDotEnvToken(baseDir string) string {
	f, err := os.Open(filepath.Join(baseDir, ".env"))
	if err != nil {
		return ""
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.Trim(strings.TrimSpace(parts[1]), `"'`)
		if key == "CLOUDFLARE_API_TOKEN" || key == "AUTODEPLOY_CLOUDFLARE_TOKEN" {
			return val
		}
	}
	if scanner.Err() != nil {
		return ""
	}
	return ""
}

func tokenFromEnv() string {
	if v := os.Getenv("CLOUDFLARE_API_TOKEN"); v != "" {
		return v
	}
	if v := os.Getenv("AUTODEPLOY_CLOUDFLARE_TOKEN"); v != "" {
		return v
	}
	return ""
}

func ResolveConfigPath(p string) string {
	if p == "" {
		p = "config_auto_deploy.json"
	}
	if abs, err := filepath.Abs(p); err == nil {
		return abs
	}
	return p
}

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}
	if _, hasToken := raw["cloudflare_api_token"]; hasToken {
		return nil, fmt.Errorf(
			"'cloudflare_api_token' must not be set in config_auto_deploy.json — " +
				"use CLOUDFLARE_API_TOKEN in a .env file or environment variable instead",
		)
	}

	var cf configFile
	if err := json.Unmarshal(data, &cf); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	cfg := &Config{
		Domain:                       strings.TrimSpace(cf.Domain),
		UpstreamHost:                 cf.UpstreamHost,
		UpstreamPort:                 cf.UpstreamPort,
		ComposeFile:                  cf.ComposeFile,
		ComposeWorkdir:               cf.ComposeWorkdir,
		ComposeService:               cf.ComposeService,
		CertbotEmail:                 strings.TrimSpace(cf.CertbotEmail),
		ChallengeMethod:              NormalizeChallengeMethod(cf.ChallengeMethod),
		CloudflarePropagationSeconds: cf.CloudflarePropagationSeconds,
		NginxSitesAvailable:          cf.NginxSitesAvailable,
		NginxSitesEnabled:            cf.NginxSitesEnabled,
		ClientMaxBodySize:            cf.ClientMaxBodySize,
		ForceSSLRedirect:             cf.ForceSSLRedirect,
		ExtraNginxDirectives:         cf.ExtraNginxDirectives,
		LogFile:                      cf.LogFile,
	}

	// defaults
	if cfg.UpstreamHost == "" {
		cfg.UpstreamHost = "127.0.0.1"
	}
	if cfg.ComposeFile == "" {
		cfg.ComposeFile = "docker-compose.yml"
	}
	if cfg.CloudflarePropagationSeconds <= 0 {
		cfg.CloudflarePropagationSeconds = 30
	}
	if cfg.NginxSitesAvailable == "" {
		cfg.NginxSitesAvailable = "/etc/nginx/sites-available"
	}
	if cfg.NginxSitesEnabled == "" {
		cfg.NginxSitesEnabled = "/etc/nginx/sites-enabled"
	}
	if cfg.ClientMaxBodySize == "" {
		cfg.ClientMaxBodySize = "10m"
	}
	if cfg.LogFile == "" {
		cfg.LogFile = "auto_deploy.log"
	}

	// token resolution: env var → .env file in compose workdir → empty
	token := tokenFromEnv()
	if token == "" {
		baseDir := cfg.ComposeWorkdir
		if baseDir == "" {
			baseDir = filepath.Dir(path)
		}
		token = LoadDotEnvToken(baseDir)
	}
	cfg.CloudflareToken = token

	return cfg, nil
}

func NormalizeChallengeMethod(v string) string {
	v = strings.ToLower(strings.TrimSpace(v))
	switch v {
	case "", "dns-cloudflare":
		return "dns-cloudflare"
	case "http":
		return "http"
	default:
		return "dns-cloudflare"
	}
}

func configToFile(cfg *Config) configFile {
	return configFile{
		Domain:                       cfg.Domain,
		UpstreamHost:                 cfg.UpstreamHost,
		UpstreamPort:                 cfg.UpstreamPort,
		ComposeFile:                  cfg.ComposeFile,
		ComposeWorkdir:               cfg.ComposeWorkdir,
		ComposeService:               cfg.ComposeService,
		CertbotEmail:                 cfg.CertbotEmail,
		ChallengeMethod:              cfg.ChallengeMethod,
		CloudflarePropagationSeconds: cfg.CloudflarePropagationSeconds,
		NginxSitesAvailable:          cfg.NginxSitesAvailable,
		NginxSitesEnabled:            cfg.NginxSitesEnabled,
		ClientMaxBodySize:            cfg.ClientMaxBodySize,
		ForceSSLRedirect:             cfg.ForceSSLRedirect,
		ExtraNginxDirectives:         cfg.ExtraNginxDirectives,
		LogFile:                      cfg.LogFile,
	}
}

func writeConfig(path string, cfg configFile) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// detectComposeFile looks for the common compose file names in dir and
// returns the first match, or the conventional default if none exist yet.
func detectComposeFile(dir string) string {
	candidates := []string{"docker-compose.yml", "docker-compose.yaml", "compose.yml", "compose.yaml"}
	for _, c := range candidates {
		if _, err := os.Stat(filepath.Join(dir, c)); err == nil {
			return c
		}
	}
	return "docker-compose.yml"
}

func logPath(cfg *Config, cfgPath string) string {
	if filepath.IsAbs(cfg.LogFile) {
		return cfg.LogFile
	}
	return filepath.Join(filepath.Dir(cfgPath), cfg.LogFile)
}

func composeWorkdir(cfg *Config, cfgPath string) string {
	if cfg.ComposeWorkdir != "" {
		return cfg.ComposeWorkdir
	}
	return filepath.Dir(cfgPath)
}
