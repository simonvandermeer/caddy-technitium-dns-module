package technitium

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/libdns/libdns"
	"go.uber.org/zap"
)

func init() {
	caddy.RegisterModule(Provider{})
}

// Provider wraps the provider as a Caddy module
type Provider struct {
	// The base URL of the Technitium DNS server
	ServerURL string `json:"server_url,omitempty"`

	// The API token for authentication
	APIToken string `json:"api_token,omitempty"`

	// HTTP timeout for API requests (default: 30s)
	HTTPTimeout caddy.Duration `json:"http_timeout,omitempty"`

	// TTL for TXT records (default: 120s)
	TTL caddy.Duration `json:"ttl,omitempty"`

	logger *zap.Logger
	client *http.Client
}

// CaddyModule returns the Caddy module information
func (Provider) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "dns.providers.technitium",
		New: func() caddy.Module { return &Provider{} },
	}
}

// Provision sets up the provider
func (p *Provider) Provision(ctx caddy.Context) error {
	p.logger = ctx.Logger()

	// Set defaults
	if p.HTTPTimeout == 0 {
		p.HTTPTimeout = caddy.Duration(30 * time.Second)
	}
	if p.TTL == 0 {
		p.TTL = caddy.Duration(120 * time.Second)
	}

	// Create HTTP client
	p.client = &http.Client{
		Timeout: time.Duration(p.HTTPTimeout),
	}

	return nil
}

// UnmarshalCaddyfile implements caddyfile.Unmarshaler
func (p *Provider) UnmarshalCaddyfile(d *caddyfile.Dispenser) error {
	for d.Next() {
		if d.NextArg() {
			return d.ArgErr()
		}
		for nesting := d.Nesting(); d.NextBlock(nesting); {
			switch d.Val() {
			case "server_url":
				if !d.NextArg() {
					return d.ArgErr()
				}
				p.ServerURL = d.Val()
			case "api_token":
				if !d.NextArg() {
					return d.ArgErr()
				}
				p.APIToken = d.Val()
			case "http_timeout":
				if !d.NextArg() {
					return d.ArgErr()
				}
				dur, err := time.ParseDuration(d.Val())
				if err != nil {
					return d.Err(err.Error())
				}
				p.HTTPTimeout = caddy.Duration(dur)
			case "ttl":
				if !d.NextArg() {
					return d.ArgErr()
				}
				dur, err := time.ParseDuration(d.Val())
				if err != nil {
					return d.Err(err.Error())
				}
				p.TTL = caddy.Duration(dur)
			default:
				return d.Errf("unrecognized subdirective '%s'", d.Val())
			}
		}
	}

	if p.ServerURL == "" {
		return d.Err("server_url is required")
	}
	if p.APIToken == "" {
		return d.Err("api_token is required")
	}

	return nil
}

// AppendRecords adds records to the zone
func (p *Provider) AppendRecords(ctx context.Context, zone string, records []libdns.Record) ([]libdns.Record, error) {
	var appendedRecords []libdns.Record

	for _, record := range records {
		var recordData = record.RR()
		if recordData.Type != "TXT" {
			continue // Only handle TXT records for ACME challenges
		}

		// Clean up the record name and zone
		name := strings.TrimSuffix(recordData.Name, ".")
		if !strings.HasSuffix(name, zone) {
			name = name + "." + strings.TrimSuffix(zone, ".")
		}

		err := p.addRecord(name, recordData.Data, int(time.Duration(p.TTL).Seconds()))
		if err != nil {
			return nil, fmt.Errorf("failed to add TXT record for %s: %v", name, err)
		}

		appendedRecords = append(appendedRecords, record)
		p.logger.Info("Added TXT record", zap.String("name", name), zap.String("value", recordData.Data))
	}

	return appendedRecords, nil
}

// DeleteRecords removes records from the zone
func (p *Provider) DeleteRecords(ctx context.Context, zone string, records []libdns.Record) ([]libdns.Record, error) {
	var deletedRecords []libdns.Record

	for _, record := range records {
		var recordData = record.RR()
		if recordData.Type != "TXT" {
			continue // Only handle TXT records for ACME challenges
		}

		// Clean up the record name and zone
		name := strings.TrimSuffix(recordData.Name, ".")
		if !strings.HasSuffix(name, zone) {
			name = name + "." + strings.TrimSuffix(zone, ".")
		}

		err := p.deleteRecord(name, recordData.Data)
		if err != nil {
			return nil, fmt.Errorf("failed to delete TXT record for %s: %v", name, err)
		}

		deletedRecords = append(deletedRecords, record)
		p.logger.Info("Deleted TXT record", zap.String("name", name), zap.String("value", recordData.Data))
	}

	return deletedRecords, nil
}

// GetRecords retrieves records from the zone
func (p *Provider) GetRecords(ctx context.Context, zone string) ([]libdns.Record, error) {
	// This method is not required for ACME DNS challenges
	// but can be implemented if needed
	return nil, fmt.Errorf("GetRecords not implemented")
}

// SetRecords replaces records in the zone
func (p *Provider) SetRecords(ctx context.Context, zone string, records []libdns.Record) ([]libdns.Record, error) {
	// This method is not required for ACME DNS challenges
	// but can be implemented if needed
	return nil, fmt.Errorf("SetRecords not implemented")
}

// addRecord adds a TXT record via Technitium API
func (p *Provider) addRecord(domain, text string, ttl int) error {
	apiURL := fmt.Sprintf("%s/api/zones/records/add", strings.TrimSuffix(p.ServerURL, "/"))

	params := url.Values{}
	params.Set("token", p.APIToken)
	params.Set("domain", domain)
	params.Set("type", "TXT")
	params.Set("ttl", fmt.Sprintf("%d", ttl))
	params.Set("text", text)

	req, err := http.NewRequest("GET", apiURL+"?"+params.Encode(), nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("failed to parse response: %v", err)
	}

	if status, ok := result["status"].(string); !ok || status != "ok" {
		return fmt.Errorf("API returned error: %s", string(body))
	}

	return nil
}

// deleteRecord deletes a TXT record via Technitium API
func (p *Provider) deleteRecord(domain, text string) error {
	apiURL := fmt.Sprintf("%s/api/zones/records/delete", strings.TrimSuffix(p.ServerURL, "/"))

	params := url.Values{}
	params.Set("token", p.APIToken)
	params.Set("domain", domain)
	params.Set("type", "TXT")
	params.Set("text", text)

	req, err := http.NewRequest("GET", apiURL+"?"+params.Encode(), nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("failed to parse response: %v", err)
	}

	if status, ok := result["status"].(string); !ok || status != "ok" {
		return fmt.Errorf("API returned error: %s", string(body))
	}

	return nil
}

// Interface guards
var (
	_ caddyfile.Unmarshaler = (*Provider)(nil)
	_ caddy.Provisioner     = (*Provider)(nil)
	_ libdns.RecordAppender = (*Provider)(nil)
	_ libdns.RecordDeleter  = (*Provider)(nil)
)
