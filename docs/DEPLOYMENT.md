# Deployment Guide: Bunny API Proxy

This guide covers deploying Bunny API Proxy in production environments. The proxy serves as an intermediary between clients (like ACME clients) and bunny.net, providing scoped API keys with granular permissions for DNS operations.

## Table of Contents

1. [Prerequisites](#prerequisites)
2. [Quick Start (Docker)](#quick-start-docker)
3. [Docker Compose](#docker-compose)
4. [Initial Setup](#initial-setup)
5. [Configuration Reference](#configuration-reference)
6. [Security Recommendations](#security-recommendations)
7. [Production Deployment Patterns](#production-deployment-patterns)
8. [Backup and Recovery](#backup-and-recovery)
9. [Upgrading](#upgrading)
10. [Monitoring and Health Checks](#monitoring-and-health-checks)

## Prerequisites

### Docker (Recommended)

- Docker 20.10+ or Docker Compose 2.0+
- No additional dependencies (everything is included in the Alpine-based image)

### Go (Source Build)

- Go 1.24 or later
- CGO support with gcc and musl-dev (for SQLite)
- For testing: golangci-lint, govulncheck

### bunny.net Account

- Active bunny.net account with DNS zones configured
- Master API key from your bunny.net account settings
- At least one DNS zone to test with

### Required Credentials

Before deployment, you'll need:

1. **bunny.net Master API Key**: Your master API key from bunny.net account settings
   - Used during bootstrap to create the first admin token
   - Get this from: bunny.net Dashboard > Account Settings > API

## Quick Start (Docker)

The simplest way to deploy Bunny API Proxy is using Docker.

### Run the Container

```bash
docker run -d \
  --name bunny-api-proxy \
  -e LOG_LEVEL="info" \
  -p 8080:8080 \
  -v bunny-proxy-data:/data \
  ghcr.io/sipico/bunny-api-proxy:latest
```

**Key options explained:**

| Flag | Purpose |
|------|---------|
| `-d` | Run in background (detached mode) |
| `--name bunny-api-proxy` | Assign a container name for easier management |
| `-e LOG_LEVEL` | Optional: info (default), debug, warn, or error |
| `-p 8080:8080` | Map container port 8080 to host port 8080 |
| `-v bunny-proxy-data:/data` | Mount Docker volume for persistent database storage |

### Verify Deployment

```bash
# Check container is running
docker ps | grep bunny-api-proxy

# Check health
curl http://localhost:8080/health
# Expected: {"status":"ok"}

# Check readiness (includes database connectivity)
curl http://localhost:8080/ready
# Expected: {"status":"ok"} (after database initialization)

# View logs
docker logs bunny-api-proxy
```

## Docker Compose

For easier multi-container deployments and configuration management, use Docker Compose.

### Basic Example

Create a file named `docker-compose.yml`:

```yaml
version: '3.8'

services:
  bunny-api-proxy:
    image: ghcr.io/sipico/bunny-api-proxy:latest
    container_name: bunny-api-proxy
    restart: unless-stopped

    environment:
      LOG_LEVEL: ${LOG_LEVEL:-info}
      LISTEN_ADDR: ":8080"
      DATABASE_PATH: /data/proxy.db

    ports:
      - "8080:8080"

    volumes:
      - bunny-proxy-data:/data

    healthcheck:
      test: ["CMD", "wget", "--no-verbose", "--tries=1", "--spider", "http://localhost:8080/health"]
      interval: 30s
      timeout: 3s
      retries: 3
      start_period: 5s

volumes:
  bunny-proxy-data:
    driver: local
```

### Environment File

Create a `.env` file in the same directory:

```bash
LOG_LEVEL=info
```

### Deployment Commands

```bash
# Start the service
docker compose up -d

# View logs
docker compose logs -f bunny-api-proxy

# Stop the service
docker compose down

# Restart the service
docker compose restart bunny-api-proxy

# Update to latest version
docker compose pull
docker compose up -d
```

### Production Docker Compose with Reverse Proxy

For production deployments, use a reverse proxy with TLS termination. Below is an example with Traefik:

```yaml
version: '3.8'

services:
  bunny-api-proxy:
    image: ghcr.io/sipico/bunny-api-proxy:latest
    container_name: bunny-api-proxy
    restart: unless-stopped

    environment:
      LOG_LEVEL: ${LOG_LEVEL:-info}
      LISTEN_ADDR: ":8080"
      DATABASE_PATH: /data/proxy.db

    volumes:
      - bunny-proxy-data:/data

    labels:
      - "traefik.enable=true"
      - "traefik.http.routers.bunny-api-proxy.rule=Host(`${PROXY_HOSTNAME}`)"
      - "traefik.http.routers.bunny-api-proxy.entrypoints=websecure"
      - "traefik.http.routers.bunny-api-proxy.tls.certresolver=letsencrypt"
      - "traefik.http.services.bunny-api-proxy.loadbalancer.server.port=8080"

    healthcheck:
      test: ["CMD", "wget", "--no-verbose", "--tries=1", "--spider", "http://localhost:8080/health"]
      interval: 30s
      timeout: 3s
      retries: 3
      start_period: 5s

    networks:
      - traefik-network

  traefik:
    image: traefik:v2.10
    container_name: traefik
    restart: unless-stopped

    environment:
      TRAEFIK_API_INSECURE: "false"
      TRAEFIK_ENTRYPOINTS_WEB_ADDRESS: ":80"
      TRAEFIK_ENTRYPOINTS_WEBSECURE_ADDRESS: ":443"
      TRAEFIK_PROVIDERS_DOCKER: "true"
      TRAEFIK_PROVIDERS_DOCKER_EXPOSEDBYDEFAULT: "false"
      TRAEFIK_CERTIFICATESRESOLVERS_LETSENCRYPT_ACME_HTTPCHALLENGE_ENTRYPOINT: web
      TRAEFIK_CERTIFICATESRESOLVERS_LETSENCRYPT_ACME_EMAIL: ${ACME_EMAIL}
      TRAEFIK_CERTIFICATESRESOLVERS_LETSENCRYPT_ACME_STORAGE: /acme.json

    ports:
      - "80:80"
      - "443:443"

    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
      - traefik-acme:/acme.json

    networks:
      - traefik-network

volumes:
  bunny-proxy-data:
    driver: local
  traefik-acme:
    driver: local

networks:
  traefik-network:
    driver: bridge
```

For this setup, update your `.env` file:

```bash
LOG_LEVEL=info
PROXY_HOSTNAME=api-proxy.example.com
ACME_EMAIL=admin@example.com
```

## Initial Setup

After successful deployment, follow these steps to configure the proxy:

### Step 1: Bootstrap - Create First Admin Token

Use your bunny.net master API key to create the first admin token:

```bash
# Bootstrap with your bunny.net master API key
curl -X POST http://localhost:8080/admin/api/bootstrap \
  -H "AccessKey: your-bunny-net-master-api-key" \
  -H "Content-Type: application/json" \
  -d '{"name": "initial-admin"}'

# Response:
# {
#   "id": 1,
#   "name": "initial-admin",
#   "token": "generated-admin-token"
# }
```

**Important**: Save the returned token securely - it cannot be retrieved later.

### Step 2: Set the Master API Key

Store the master API key hash in the proxy:

```bash
curl -X PUT http://localhost:8080/admin/api/master-key \
  -H "AccessKey: generated-admin-token" \
  -H "Content-Type: application/json" \
  -d '{"api_key": "your-bunny-net-master-api-key"}'
```

### Step 3: Create Your First Scoped Key

```bash
curl -X POST http://localhost:8080/admin/api/keys \
  -H "AccessKey: generated-admin-token" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "acme-dns-validation",
    "zones": [123456],
    "actions": ["list_zones", "list_records", "add_record", "delete_record"],
    "record_types": ["TXT"]
  }'

# Response:
# {
#   "id": 1,
#   "key": "scoped-key-value"
# }
```

**Important**: Save the returned key securely - it cannot be retrieved later.

### Step 4: Test the Scoped Key

```bash
# List zones (should work if key has permission)
curl -H "AccessKey: scoped-key-value" \
  http://localhost:8080/dnszone

# Create a TXT record for ACME validation
curl -X POST \
  -H "AccessKey: scoped-key-value" \
  -H "Content-Type: application/json" \
  -d '{"Type":"TXT","Name":"_acme-challenge","Value":"validation-token","Ttl":300}' \
  http://localhost:8080/dnszone/123456/records

# Delete the test record when done
curl -X DELETE \
  -H "AccessKey: scoped-key-value" \
  http://localhost:8080/dnszone/123456/records/record-id
```

## Configuration Reference

All configuration is done via environment variables. They must be set before the container starts.

### Environment Variables

| Variable | Type | Default | Values | Description |
|----------|------|---------|--------|-------------|
| `LOG_LEVEL` | String | `info` | `debug`, `info`, `warn`, `error` | Logging verbosity. Use `debug` for troubleshooting. Can be changed dynamically via Admin API without restart. |
| `LISTEN_ADDR` | Address | `:8080` | `:8080`, `0.0.0.0:8080` | HTTP server listen address. Must match container port mapping if using Docker. |
| `DATABASE_PATH` | File path | `/data/proxy.db` | `/data/proxy.db` (recommended) | SQLite database file location. Should be on a mounted volume for persistence. |
| `BUNNY_API_URL` | URL | `https://api.bunny.net` | `https://api.bunny.net` | Override bunny.net API endpoint. Mainly for testing against mock servers. |

### Configuration Examples

**Development (verbose logging):**
```bash
LOG_LEVEL=debug
LISTEN_ADDR=:8080
DATABASE_PATH=/data/proxy.db
```

**Production (minimal logging):**
```bash
LOG_LEVEL=warn
LISTEN_ADDR=:8080
DATABASE_PATH=/data/proxy.db
```

**Testing (custom bunny.net endpoint):**
```bash
BUNNY_API_URL=http://mock-bunny:9999
LOG_LEVEL=debug
```

## Security Recommendations

### Essential Security Practices

1. **Protect Your bunny.net Master API Key**
   - Only use during bootstrap to create admin tokens
   - Store securely in a password manager
   - Never commit to version control
   - The proxy stores only a hash - the original key is needed for bunny.net API calls

2. **Protect Admin Tokens**
   - Store admin tokens securely after creation
   - Tokens are shown only once and cannot be retrieved
   - Create separate tokens for different automation purposes
   - Rotate tokens periodically

3. **Network Security**
   - **Never expose port 8080 directly to the internet**
   - Always run behind a reverse proxy with TLS termination
   - Restrict admin UI access to trusted networks only
   - Use firewall rules to limit proxy API access to authorized clients

4. **TLS/HTTPS**
   - Use Let's Encrypt or your organization's certificate authority
   - Renew certificates automatically (Traefik does this automatically)
   - Set HSTS headers in reverse proxy:
     ```
     Strict-Transport-Security: max-age=31536000; includeSubDomains
     ```

5. **API Key Management**
   - Rotate scoped keys regularly (quarterly recommended)
   - Delete unused keys immediately
   - Use the principle of least privilege (limit permissions/zones)
   - Audit key usage via logs (enable info or debug logging)
   - Never share keys via email or chat (use secure secret management)

6. **Monitoring and Logging**
   - Aggregate logs to centralized service (ELK, Splunk, CloudWatch)
   - Alert on authentication failures
   - Monitor for permission denied errors (potential attacks)
   - Keep audit trail of key creation/deletion/modification

7. **Backup and Disaster Recovery**
   - Back up `/data/proxy.db` regularly (daily minimum)
   - Test restore procedures regularly
   - Store backups encrypted and off-site
   - Document recovery procedures

8. **Regular Updates**
   - Monitor for security updates
   - Test updates in staging before production
   - Automate dependency updates via Dependabot
   - Run `govulncheck` regularly for vulnerability scanning

9. **Access Control**
   - Limit admin UI access to specific IP addresses if possible
   - Use a VPN or bastion host for administrative access
   - Consider implementing two-factor authentication at reverse proxy level
   - Audit admin action logs

10. **Database Security**
    - Store database file on encrypted filesystem (recommended)
    - Restrict file permissions: `chmod 600 /data/proxy.db`
    - Never backup credentials alongside unencrypted database files
    - Consider using encrypted volumes (LUKS, BitLocker, etc.)

## Production Deployment Patterns

### Pattern 1: Kubernetes Deployment

For Kubernetes environments, use a Deployment with persistent storage:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: bunny-api-proxy-config
data:
  LOG_LEVEL: "info"
  LISTEN_ADDR: ":8080"
  DATABASE_PATH: "/data/proxy.db"

---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: bunny-api-proxy-data
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 10Gi

---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: bunny-api-proxy
spec:
  replicas: 1  # SQLite doesn't support concurrent writes
  selector:
    matchLabels:
      app: bunny-api-proxy
  template:
    metadata:
      labels:
        app: bunny-api-proxy
    spec:
      containers:
      - name: bunny-api-proxy
        image: ghcr.io/sipico/bunny-api-proxy:latest
        imagePullPolicy: IfNotPresent

        ports:
        - containerPort: 8080
          name: http

        envFrom:
        - configMapRef:
            name: bunny-api-proxy-config

        volumeMounts:
        - name: data
          mountPath: /data

        livenessProbe:
          httpGet:
            path: /health
            port: http
          initialDelaySeconds: 5
          periodSeconds: 10
          timeoutSeconds: 3
          failureThreshold: 3

        readinessProbe:
          httpGet:
            path: /ready
            port: http
          initialDelaySeconds: 10
          periodSeconds: 5
          timeoutSeconds: 3
          failureThreshold: 3

        resources:
          requests:
            memory: "64Mi"
            cpu: "100m"
          limits:
            memory: "256Mi"
            cpu: "500m"

      volumes:
      - name: data
        persistentVolumeClaim:
          claimName: bunny-api-proxy-data

---
apiVersion: v1
kind: Service
metadata:
  name: bunny-api-proxy
spec:
  selector:
    app: bunny-api-proxy
  ports:
  - port: 80
    targetPort: http
    protocol: TCP
  type: ClusterIP
```

**Note**: SQLite supports only one writer at a time, so use `replicas: 1` for Kubernetes deployments. For high availability, consider migrating to PostgreSQL in the future.

### Pattern 2: systemd Service (Bare Metal)

For running on Linux servers without containers:

**Build from source:**
```bash
git clone https://github.com/sipico/bunny-api-proxy.git
cd bunny-api-proxy
go build -o /usr/local/bin/bunny-api-proxy ./cmd/bunny-api-proxy
```

**Create systemd service file** (`/etc/systemd/system/bunny-api-proxy.service`):

```ini
[Unit]
Description=Bunny API Proxy
After=network.target
Documentation=https://github.com/sipico/bunny-api-proxy

[Service]
Type=simple
User=bunny
Group=bunny
WorkingDirectory=/var/lib/bunny-api-proxy

# Environment variables
Environment="LOG_LEVEL=info"
Environment="LISTEN_ADDR=:8080"
Environment="DATABASE_PATH=/var/lib/bunny-api-proxy/proxy.db"

# Security hardening
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=yes
ReadWritePaths=/var/lib/bunny-api-proxy

# Process management
ExecStart=/usr/local/bin/bunny-api-proxy
Restart=on-failure
RestartSec=10

# Logging
StandardOutput=journal
StandardError=journal
SyslogIdentifier=bunny-api-proxy

[Install]
WantedBy=multi-user.target
```

**Setup and run:**
```bash
# Create user and data directory
sudo useradd -r -s /bin/false bunny
sudo mkdir -p /var/lib/bunny-api-proxy
sudo chown -R bunny:bunny /var/lib/bunny-api-proxy
sudo chmod 700 /var/lib/bunny-api-proxy

# Enable and start the service
sudo systemctl daemon-reload
sudo systemctl enable bunny-api-proxy
sudo systemctl start bunny-api-proxy

# Check status
sudo systemctl status bunny-api-proxy

# View logs
sudo journalctl -u bunny-api-proxy -f
```

### Pattern 3: nginx Reverse Proxy

For TLS termination and load balancing:

```nginx
upstream bunny_api_proxy {
    server localhost:8080;
}

server {
    listen 80;
    server_name api-proxy.example.com;
    return 301 https://$server_name$request_uri;
}

server {
    listen 443 ssl http2;
    server_name api-proxy.example.com;

    # SSL configuration
    ssl_certificate /etc/letsencrypt/live/api-proxy.example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/api-proxy.example.com/privkey.pem;
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers HIGH:!aNULL:!MD5;
    ssl_prefer_server_ciphers on;
    ssl_session_cache shared:SSL:10m;
    ssl_session_timeout 10m;

    # Security headers
    add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;
    add_header X-Frame-Options "SAMEORIGIN" always;
    add_header X-Content-Type-Options "nosniff" always;
    add_header X-XSS-Protection "1; mode=block" always;

    # Proxy configuration
    location / {
        proxy_pass http://bunny_api_proxy;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        # Timeouts
        proxy_connect_timeout 15s;
        proxy_send_timeout 15s;
        proxy_read_timeout 15s;
    }

    # Health check endpoint (no proxy needed)
    location /health {
        access_log off;
        proxy_pass http://bunny_api_proxy;
    }
}
```

Enable with Certbot:
```bash
sudo certbot --nginx -d api-proxy.example.com
sudo systemctl reload nginx
```

## Backup and Recovery

### What to Backup

The SQLite database contains:
- Scoped API keys and their permissions
- Configuration (log level, etc.)
- Encrypted master API key
- Admin tokens and sessions

**Location**: `/data/proxy.db` (inside container or mounted volume)

### Backup Procedure

**Option 1: Docker Volume Backup**
```bash
# Create a backup
docker run --rm \
  -v bunny-proxy-data:/data \
  -v $(pwd):/backup \
  alpine tar czf /backup/proxy-backup-$(date +%Y%m%d-%H%M%S).tar.gz -C /data proxy.db

# List backups
ls -lh proxy-backup-*.tar.gz
```

**Option 2: Direct File Backup (Host-mounted volume)**
```bash
# If using host path volume
cp /var/lib/bunny-api-proxy/proxy.db /backups/proxy-$(date +%Y%m%d-%H%M%S).db

# Or with compression
gzip -c /var/lib/bunny-api-proxy/proxy.db > /backups/proxy-$(date +%Y%m%d-%H%M%S).db.gz
```

**Option 3: Automated Daily Backup (cron)**
```bash
# Add to crontab -e
0 2 * * * docker run --rm -v bunny-proxy-data:/data -v /backups:/backup alpine tar czf /backup/proxy-backup-$(date +\%Y\%m\%d).tar.gz -C /data proxy.db && find /backups -name "proxy-backup-*.tar.gz" -mtime +30 -delete
```

### Recovery Procedure

**If database is corrupted or lost:**

1. **Stop the container**
   ```bash
   docker compose down
   # or: docker stop bunny-api-proxy
   ```

2. **Restore the database**
   ```bash
   # Option 1: From Docker volume backup
   docker run --rm \
     -v bunny-proxy-data:/data \
     -v $(pwd):/backup \
     alpine tar xzf /backup/proxy-backup-YYYYMMDD-HHMMSS.tar.gz -C /data

   # Option 2: From direct file backup
   cp /backups/proxy-YYYYMMDD.db /var/lib/bunny-api-proxy/proxy.db
   ```

3. **Verify restoration**
   ```bash
   ls -la /data/proxy.db  # or your data path
   ```

4. **Start the container**
   ```bash
   docker compose up -d
   # or: docker start bunny-api-proxy
   ```

5. **Test access**
   ```bash
   curl http://localhost:8080/ready
   ```

### If Admin Token is Lost

All admin tokens are stored as hashes and cannot be recovered.

**Recovery steps**:

1. If other admin tokens exist, use one to create a new token
2. If no admin tokens exist, the bootstrap endpoint becomes available again:
   ```bash
   curl -X POST http://localhost:8080/admin/api/bootstrap \
     -H "AccessKey: your-bunny-net-master-api-key" \
     -H "Content-Type: application/json" \
     -d '{"name": "recovery-admin"}'
   ```
3. All scoped keys and permissions remain intact

## Upgrading

Upgrades are safe and straightforward. The database schema is auto-initialized on startup.

### Docker Upgrade

```bash
# Pull the latest image
docker pull ghcr.io/sipico/bunny-api-proxy:latest

# Stop the current container
docker stop bunny-api-proxy

# Remove the old container
docker rm bunny-api-proxy

# Start with the new image (same command as initial deployment)
docker run -d \
  --name bunny-api-proxy \
  -e ADMIN_PASSWORD="..." \
  -e ENCRYPTION_KEY="..." \
  -p 8080:8080 \
  -v bunny-proxy-data:/data \
  ghcr.io/sipico/bunny-api-proxy:latest

# Verify
docker logs bunny-api-proxy
curl http://localhost:8080/ready
```

### Docker Compose Upgrade

```bash
# Pull the latest image
docker compose pull

# Restart with new image
docker compose up -d

# Verify
docker compose logs bunny-api-proxy
curl http://localhost:8080/ready
```

### Upgrade Checklist

- [ ] Backup `/data/proxy.db` before upgrading
- [ ] Pull latest image
- [ ] Restart container/service
- [ ] Verify `/ready` endpoint returns OK
- [ ] Test scoped key functionality
- [ ] Check logs for errors: `docker logs bunny-api-proxy`
- [ ] Confirm admin UI is accessible

## Monitoring and Health Checks

### Health Check Endpoints

Two endpoints provide deployment status:

**`GET /health` - Liveness Check**
- Returns immediately if process is alive
- Used to determine if container should be restarted
- No external dependencies checked

```bash
curl http://localhost:8080/health
# Response: {"status":"ok"}
```

**`GET /ready` - Readiness Check**
- Verifies database connectivity and accessibility
- Used to determine if container should receive traffic
- Will return 503 Service Unavailable if database is inaccessible

```bash
curl http://localhost:8080/ready
# Success: {"status":"ok"}
# Failure: {"status":"not_ready","error":"database unavailable"}
```

### Docker Health Check Configuration

Already included in Dockerfile:

```dockerfile
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1
```

### Monitoring with Prometheus (Optional)

While the proxy doesn't currently export Prometheus metrics, you can monitor:

1. **Container health**: Use Docker health status
2. **Endpoint availability**: Poll `/health` and `/ready` endpoints
3. **Logs**: Parse JSON logs for errors and warnings

Example Prometheus scrape config (future enhancement):
```yaml
scrape_configs:
  - job_name: 'bunny-api-proxy'
    static_configs:
      - targets: ['localhost:8080']
    metrics_path: '/metrics'
```

### Key Metrics to Monitor

1. **Availability**: `/ready` endpoint status
2. **Error rate**: Count of 4xx/5xx responses in logs
3. **Authentication failures**: Permission denied errors
4. **Request latency**: Time to respond to requests
5. **Database connectivity**: Any DB errors in logs
6. **Uptime**: Container restart frequency

### Sample Monitoring Setup (ELK Stack)

```yaml
# Filebeat configuration to ship logs to Elasticsearch
filebeat.inputs:
- type: container
  paths:
    - '/var/lib/docker/containers/*/*.log'
  processors:
    - add_docker_metadata: ~

output.elasticsearch:
  hosts: ["elasticsearch:9200"]
  index: "bunny-api-proxy-%{+yyyy.mm.dd}"

setup.kibana:
  host: "kibana:5601"
```

Then create dashboards in Kibana to visualize:
- Request rates and response times
- Authentication failures
- Error trends
- Service availability

### Alerting Rules (Example)

```yaml
# Prometheus alerting rules
groups:
  - name: bunny_api_proxy
    rules:
      - alert: BunnyAPIProxyDown
        expr: up{job="bunny-api-proxy"} == 0
        for: 2m
        annotations:
          summary: "Bunny API Proxy is down"

      - alert: BunnyAPIProxyNotReady
        expr: bunny_api_proxy_ready{} == 0
        for: 1m
        annotations:
          summary: "Bunny API Proxy is not ready (database issue)"

      - alert: HighErrorRate
        expr: rate(http_requests_total{status=~"5.."}[5m]) > 0.05
        for: 5m
        annotations:
          summary: "High error rate detected"
```

### Logging Best Practices

**Log Levels and When to Use**

| Level | Use Case | Performance Impact |
|-------|----------|-------------------|
| `error` | Production - minimal logging | Lowest |
| `warn` | Production - security focus | Very low |
| `info` | Default - balance | Low |
| `debug` | Troubleshooting only | Medium (verbose) |

**Change Log Level at Runtime** (no restart needed):

```bash
curl -X POST http://localhost:8080/admin/api/loglevel \
  -H "AccessKey: admin-token" \
  -H "Content-Type: application/json" \
  -d '{"level":"debug"}'
```

**Log JSON Structure**

Logs are output as JSON for structured analysis:

```json
{
  "time": "2024-01-25T10:30:45.123Z",
  "level": "INFO",
  "msg": "Request processed",
  "request_id": "abc123",
  "method": "GET",
  "path": "/dnszone",
  "status": 200,
  "duration_ms": 45
}
```

Parse with tools like `jq`:
```bash
docker logs bunny-api-proxy | jq 'select(.level=="ERROR")'
```

## Troubleshooting

### Container Won't Start

**Check logs:**
```bash
docker logs bunny-api-proxy
```

**Common issues:**

| Error | Solution |
|-------|----------|
| `ADMIN_PASSWORD is required` | Add `-e ADMIN_PASSWORD=...` to docker run |
| `ENCRYPTION_KEY is required` | Add `-e ENCRYPTION_KEY=...` (32 chars) |
| `ENCRYPTION_KEY must be exactly 32 characters` | Verify key length is exactly 32 |
| `address already in use` | Change port or stop existing container |
| `permission denied` | Run with `--user 1000:1000` or ensure volume permissions |

### Database Errors

**Check database connectivity:**
```bash
curl http://localhost:8080/ready
```

**If database is corrupted:**
1. Back up `/data/proxy.db`
2. Delete the file (database will be recreated)
3. Restart the container
4. Reconfigure scoped keys

### API Not Accessible

**Check if service is running:**
```bash
docker ps | grep bunny-api-proxy
curl http://localhost:8080/health
```

**If behind reverse proxy:**
- Verify reverse proxy is running
- Check reverse proxy configuration
- View reverse proxy logs
- Ensure correct hostname in requests

---

## Additional Resources

- [ARCHITECTURE.md](../ARCHITECTURE.md) - Technical architecture and design decisions
- [bunny.net API Docs](https://docs.bunny.net/reference/bunnynet-api-overview) - bunny.net API reference
- [GitHub Repository](https://github.com/sipico/bunny-api-proxy) - Source code and issues
- [Docker Documentation](https://docs.docker.com/) - Docker concepts and best practices

## Support

For issues or questions:
1. Check the [GitHub Issues](https://github.com/sipico/bunny-api-proxy/issues)
2. Review logs: `docker logs bunny-api-proxy` or `docker compose logs -f`
3. Test endpoints: `curl http://localhost:8080/health`
4. Open a new issue with reproduction steps

---

**Last Updated**: 2026-01-25
**Version**: 0.1.0
