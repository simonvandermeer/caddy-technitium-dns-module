# Caddy DNS Provider Module for Technitium

> [!WARNING]  
> This module was made using an LLM and I have **not** done a lot of testing. It works for my homelab but no guarantees it will work for your situation. Please report (or better yet, open PRs for) any issues.

This package contains a DNS provider module for Caddy that integrates with Technitium DNS Server to solve ACME DNS-01 challenges automatically.

## Features

- Automatic DNS-01 ACME challenge validation
- Support for wildcard certificates
- Configurable HTTP timeout and TTL settings
- Uses Technitium's HTTP API
- Environment variable configuration support

## Installation

### Method 1: Build with xcaddy (Recommended)

First, install xcaddy:
```bash
go install github.com/caddyserver/xcaddy/cmd/xcaddy@latest
```

Then build Caddy with the Technitium DNS plugin:
```bash
xcaddy build --with github.com/simonvandermeer/caddy-technitium-dns-module
```

### Method 2: Docker Build

Create a Dockerfile:
```dockerfile
FROM caddy:builder AS builder

RUN xcaddy build \
    --with github.com/caddy-dns/technitium

FROM caddy:latest
COPY --from=builder /usr/bin/caddy /usr/bin/caddy
```

## Configuration

### Prerequisites

1. **Technitium DNS Server**: Set up and configure Technitium DNS Server as the authoritative DNS server for your domain
2. **API Token**: Generate an API token from the Technitium web console:
   - Login to web console
   - Click user menu (top right)
   - Click "Create API Token"
   - Enter password and token name
   - Save the generated token

### Caddyfile Configuration

#### Global Configuration (all sites)
```caddyfile
{
    acme_dns technitium {
        server_url https://your-dns-server:5380
        api_token {env.TECHNITIUM_API_TOKEN}
        http_timeout 30s
        ttl 120s
    }
}

example.com {
    respond "Hello World!"
}
```

#### Per-site Configuration
```caddyfile
example.com {
    tls {
        dns technitium {
            server_url https://your-dns-server:5380
            api_token {env.TECHNITIUM_API_TOKEN}
            http_timeout 30s
            ttl 120s
        }
    }
    respond "Hello World!"
}
```

#### Wildcard Certificate Example
```caddyfile
*.example.com, example.com {
    tls {
        dns technitium {
            server_url https://your-dns-server:5380
            api_token {env.TECHNITIUM_API_TOKEN}
        }
    }
    respond "Wildcard cert working!"
}
```

### JSON Configuration

```json
{
  "apps": {
    "http": {
      "servers": {
        "srv0": {
          "listen": [":443"],
          "routes": [
            {
              "match": [{"host": ["example.com"]}],
              "handle": [
                {
                  "handler": "static_response",
                  "body": "Hello World!"
                }
              ]
            }
          ]
        }
      }
    },
    "tls": {
      "automation": {
        "policies": [
          {
            "subjects": ["example.com"],
            "issuers": [
              {
                "module": "acme",
                "challenges": {
                  "dns": {
                    "provider": {
                      "name": "technitium",
                      "server_url": "https://your-dns-server:5380",
                      "api_token": "{env.TECHNITIUM_API_TOKEN}",
                      "http_timeout": "30s",
                      "ttl": "120s"
                    }
                  }
                }
              }
            ]
          }
        ]
      }
    }
  }
}
```

### Environment Variables

```bash
export TECHNITIUM_API_TOKEN="your_api_token_here"
```

## Configuration Options

| Option         | Type     | Default  | Description                                                                   |
| -------------- | -------- | -------- | ----------------------------------------------------------------------------- |
| `server_url`   | string   | Required | Base URL of your Technitium DNS server (e.g., `https://dns.example.com:5380`) |
| `api_token`    | string   | Required | API token for authentication                                                  |
| `http_timeout` | duration | `30s`    | HTTP timeout for API requests                                                 |
| `ttl`          | duration | `120s`   | TTL for TXT records used in challenges                                        |

## How It Works

1. When Caddy needs to obtain/renew a certificate, it triggers the DNS-01 challenge
2. The plugin creates a TXT record at `_acme-challenge.yourdomain.com` using Technitium's API
3. Let's Encrypt validates the challenge by querying the DNS record
4. After validation, the plugin automatically deletes the challenge record
5. Caddy completes the certificate issuance process

## Security Considerations

- **API Token Security**: Store your API token securely using environment variables
- **Network Security**: Use HTTPS for the Technitium server URL when possible
- **Firewall**: Ensure your Technitium server is accessible from where Caddy runs
- **DNS Authority**: Technitium must be authoritative for your domain (NS records must point to your server)

## Troubleshooting

### Common Issues

1. **"API returned error"**: Check your API token and server URL
2. **"Connection refused"**: Verify Technitium server is running and accessible
3. **"Domain not found"**: Ensure Technitium is authoritative for your domain
4. **Certificate not obtained**: Check Caddy logs for detailed error messages

### Debug Steps

1. Test API connectivity:
   ```bash
   curl "https://your-dns-server:5380/api/zones/records/add?token=YOUR_TOKEN&domain=_acme-challenge.test.example.com&type=TXT&ttl=60&text=test123"
   ```

2. Verify DNS authority:
   ```bash
   dig NS example.com
   ```

3. Check Caddy logs:
   ```bash
   caddy run --config Caddyfile --adapter caddyfile
   ```

## Requirements

- Caddy v2.7.0 or later
- Technitium DNS Server (any recent version with HTTP API)
- Go 1.21 or later (for building)
- Your domain's NS records must point to your Technitium server

## API Reference

This plugin uses the following Technitium DNS Server API endpoints:

- `GET /api/zones/records/add` - Add TXT record
- `GET /api/zones/records/delete` - Delete TXT record

## Contributing

Contributions are welcome! Please feel free to submit issues and pull requests.

## License

This project follows the same license as Caddy (Apache 2.0).
