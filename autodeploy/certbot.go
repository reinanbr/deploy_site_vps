package autodeploy

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

const certbotTimeout = 3 * time.Minute

func CertbotAvailable() error {
	if _, err := exec.LookPath("certbot"); err != nil {
		return fmt.Errorf("'certbot' not found on PATH — install it first, e.g.: " +
			"apt install certbot python3-certbot-dns-cloudflare")
	}
	return nil
}

func runCertbot(args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), certbotTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "certbot", args...)
	out, err := cmd.CombinedOutput()

	if ctx.Err() == context.DeadlineExceeded {
		return strings.TrimSpace(string(out)), fmt.Errorf("certbot timed out after %s", certbotTimeout)
	}
	return strings.TrimSpace(string(out)), err
}

// writeCloudflareCredentials writes a temporary, 0600 credentials file for
// certbot's dns-cloudflare plugin. The caller must invoke the returned
// cleanup function once certbot has finished.
func writeCloudflareCredentials(token string) (string, func(), error) {
	f, err := os.CreateTemp("", "deploy_site-cloudflare-*.ini")
	if err != nil {
		return "", func() {}, err
	}
	cleanup := func() { _ = os.Remove(f.Name()) }

	if _, err := f.WriteString(fmt.Sprintf("dns_cloudflare_api_token = %s\n", token)); err != nil {
		f.Close()
		cleanup()
		return "", func() {}, err
	}
	if err := f.Chmod(0600); err != nil {
		f.Close()
		cleanup()
		return "", func() {}, err
	}
	if err := f.Close(); err != nil {
		cleanup()
		return "", func() {}, err
	}
	return f.Name(), cleanup, nil
}

// ObtainCertificate requests (or extends) a certificate for cfg.Domain using
// the configured challenge method.
func ObtainCertificate(cfg *Config) (string, error) {
	switch cfg.ChallengeMethod {
	case "http":
		if err := EnsureWebroot(); err != nil {
			return "", fmt.Errorf("failed to create webroot: %w", err)
		}
		args := []string{
			"certonly", "--webroot", "-w", DefaultWebroot,
			"-d", cfg.Domain,
			"--non-interactive", "--agree-tos", "--no-eff-email",
			"-m", cfg.CertbotEmail,
		}
		return runCertbot(args...)

	default: // dns-cloudflare
		if cfg.CloudflareToken == "" {
			return "", fmt.Errorf("cloudflare api token not set — export CLOUDFLARE_API_TOKEN or add it to .env")
		}
		credPath, cleanup, err := writeCloudflareCredentials(cfg.CloudflareToken)
		if err != nil {
			return "", fmt.Errorf("failed to write cloudflare credentials: %w", err)
		}
		defer cleanup()

		args := []string{
			"certonly", "--dns-cloudflare",
			"--dns-cloudflare-credentials", credPath,
			"--dns-cloudflare-propagation-seconds", strconv.Itoa(cfg.CloudflarePropagationSeconds),
			"-d", cfg.Domain,
			"--non-interactive", "--agree-tos", "--no-eff-email",
			"-m", cfg.CertbotEmail,
		}
		return runCertbot(args...)
	}
}

// RenewAll runs certbot's own renewal check across every managed certificate
// (a no-op for certs not yet due). Intended to be called on a schedule.
func RenewAll() (string, error) {
	return runCertbot("renew")
}

// CertificateExpiry returns the notAfter timestamp of the live certificate
// for domain, read via openssl.
func CertificateExpiry(domain string) (time.Time, error) {
	path := LiveCertPath(domain)
	if _, err := os.Stat(path); err != nil {
		return time.Time{}, fmt.Errorf("certificate not found: %s", path)
	}
	out, err := runCommandCombined("openssl", "x509", "-enddate", "-noout", "-in", path)
	if err != nil {
		return time.Time{}, fmt.Errorf("openssl failed: %w (%s)", err, out)
	}
	return ParseOpensslEnddate(out)
}

// ParseOpensslEnddate parses the output of `openssl x509 -enddate -noout`,
// e.g. "notAfter=Jan  1 00:00:00 2027 GMT".
func ParseOpensslEnddate(raw string) (time.Time, error) {
	parts := strings.SplitN(raw, "=", 2)
	if len(parts) != 2 {
		return time.Time{}, fmt.Errorf("unexpected openssl output: %q", raw)
	}
	t, err := time.Parse("Jan _2 15:04:05 2006 MST", strings.TrimSpace(parts[1]))
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to parse expiry %q: %w", parts[1], err)
	}
	return t, nil
}
