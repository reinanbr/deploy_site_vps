package tests

import (
	"testing"
	"time"

	"github.com/reinanbr/auto_deploy_go/autodeploy"
)

func TestObtainCertificate_RequiresCloudflareToken(t *testing.T) {
	cfg := &autodeploy.Config{
		Domain:          "app.example.com",
		ChallengeMethod: "dns-cloudflare",
		CloudflareToken: "",
	}
	_, err := autodeploy.ObtainCertificate(cfg)
	if err == nil {
		t.Fatal("expected error when cloudflare token is empty")
	}
}

func TestParseOpensslEnddate(t *testing.T) {
	got, err := autodeploy.ParseOpensslEnddate("notAfter=Jan  1 00:00:00 2027 GMT")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := time.Date(2027, time.January, 1, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestParseOpensslEnddate_InvalidFormat(t *testing.T) {
	if _, err := autodeploy.ParseOpensslEnddate("garbage"); err == nil {
		t.Fatal("expected error for malformed input")
	}
}
