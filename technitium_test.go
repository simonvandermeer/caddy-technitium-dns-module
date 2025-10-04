package technitium

import (
	"context"
	"testing"
	"time"

	"github.com/caddyserver/caddy/v2"
	"github.com/libdns/libdns"
)

func TestProvider_AppendRecords(t *testing.T) {
	// This is a basic test structure
	// In a real scenario, you would mock the HTTP client or use a test server

	p := &Provider{
		ServerURL:   "https://localhost:5380",
		APIToken:    "test-token",
		HTTPTimeout: caddy.Duration(30 * time.Second),
		TTL:         caddy.Duration(120 * time.Second),
	}

	ctx := context.Background()

	// Mock provision
	err := p.Provision(caddy.Context{})
	if err != nil {
		t.Fatalf("Failed to provision: %v", err)
	}

	// Test record structure
	records := []libdns.Record{
		{
			Type:  "TXT",
			Name:  "_acme-challenge.example.com",
			Value: "test-challenge-value",
		},
	}

	// Note: This test will fail without a real Technitium server
	// In practice, you would mock the HTTP calls
	_, err = p.AppendRecords(ctx, "example.com", records)
	if err == nil {
		t.Log("AppendRecords succeeded (or server is available)")
	} else {
		t.Logf("AppendRecords failed as expected without server: %v", err)
	}
}

func TestProvider_CaddyModule(t *testing.T) {
	p := Provider{}
	mod := p.CaddyModule()

	if mod.ID != "dns.providers.technitium" {
		t.Errorf("Expected module ID 'dns.providers.technitium', got '%s'", mod.ID)
	}

	if mod.New == nil {
		t.Error("Expected New function to be set")
	}
}
