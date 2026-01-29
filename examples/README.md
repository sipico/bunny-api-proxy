# Bunny API Proxy - Production Deployment Examples

This directory contains production-ready deployment configurations for Bunny API Proxy across multiple platforms:

- **Docker Compose** - Simple containerized deployment
- **Docker Compose + Traefik** - Advanced setup with reverse proxy and TLS
- **Systemd** - Bare metal Linux deployment
- **Kubernetes** - Cloud-native deployment with orchestration

Choose the deployment method that best fits your infrastructure.

## Quick Start - Docker Compose

The fastest way to get Bunny API Proxy running:

```bash
# 1. Copy environment template
cp .env.example .env

# 2. Edit configuration (important: change ADMIN_PASSWORD and ENCRYPTION_KEY)
nano .env

# 3. Start services
docker-compose up -d

# 4. Verify it's running
curl http://localhost:8080/health

# 5. Bootstrap with your bunny.net master API key
curl -X POST http://localhost:8080/admin/api/bootstrap \
  -H "AccessKey: your-bunny-net-master-api-key" \
  -H "Content-Type: application/json" \
  -d '{"name": "initial-admin"}'

# View logs
docker-compose logs -f bunny-api-proxy

# Stop services
docker-compose down

# Stop services and remove volumes (resets database)
docker-compose down -v
```

### Quick Start - Required Credentials

Before starting, have your bunny.net master API key ready:

1. Log in to your bunny.net account
2. Go to Account Settings > API
3. Copy your API key (you'll need this for bootstrap)

## Docker Compose with Traefik

For production with automatic HTTPS/TLS certificates:

```bash
# 1. Prepare environment
cp .env.example .env
nano .env

# 2. Create Traefik certificates directory
mkdir -p traefik/certs

# 3. Edit Traefik configuration
# Replace:
# - TRAEFIK_DOMAIN=api.example.com with your domain
# - AWS/CloudFlare DNS provider settings
# - admin@example.com with your email
nano examples/docker-compose.traefik.yml

# 4. Start with Traefik
docker-compose -f examples/docker-compose.traefik.yml up -d

# 5. Check status
docker-compose -f examples/docker-compose.traefik.yml ps
docker-compose -f examples/docker-compose.traefik.yml logs -f traefik

# 6. Access via HTTPS
curl https://api.example.com/health

# 7. View Traefik dashboard (if enabled)
# Browser: http://localhost:8080/dashboard/
```

### Traefik Setup Explained

**Reverse Proxy**: Traefik sits between clients and the proxy, handling:
- HTTP requests on port 80
- HTTPS requests on port 443
- Automatic TLS certificate generation via Let's Encrypt

**DNS Challenge**: Required for certificate issuance:
1. Traefik verifies domain ownership via DNS
2. Supports multiple DNS providers (Route53, CloudFlare, Azure, etc.)
3. Configure your DNS provider credentials in docker-compose.traefik.yml

**ACME Configuration**: Let's Encrypt integration:
- Automatic certificate renewal (90 days)
- Certificates stored in `traefik/certs/acme.json`
- Email for renewal notifications

### Traefik Production Checklist

- [ ] Change `TRAEFIK_DOMAIN` to your domain
- [ ] Select and configure DNS provider (Route53, CloudFlare, etc.)
- [ ] Change email to your address
- [ ] Set up DNS records (A record pointing to server)
- [ ] Configure AWS/CloudFlare API credentials
- [ ] Test certificate generation
- [ ] Enable firewall rules for ports 80/443
- [ ] Set up monitoring/alerts for certificate expiration

## Systemd Setup (Bare Metal)

For running directly on Linux without containers:

### Prerequisites

- Linux system (Ubuntu 20.04+, CentOS 8+, etc.)
- `curl` utility for health checks
- 500MB free disk space
- Port 8080 available (or configure HTTP_PORT)

### Installation Steps

**1. Create service user:**

```bash
sudo useradd --system --home /var/lib/bunny-api-proxy \
  --shell /usr/sbin/nologin bunny-proxy
```

**2. Create data directory:**

```bash
sudo mkdir -p /var/lib/bunny-api-proxy
sudo chown bunny-proxy:bunny-proxy /var/lib/bunny-api-proxy
sudo chmod 700 /var/lib/bunny-api-proxy
```

**3. Create configuration directory:**

```bash
sudo mkdir -p /etc/bunny-api-proxy
sudo chown root:bunny-proxy /etc/bunny-api-proxy
sudo chmod 750 /etc/bunny-api-proxy
```

**4. Copy environment file:**

```bash
sudo cp ../.env.example /etc/bunny-api-proxy/.env
sudo chown root:bunny-proxy /etc/bunny-api-proxy/.env
sudo chmod 640 /etc/bunny-api-proxy/.env
```

**5. Edit configuration:**

```bash
sudo nano /etc/bunny-api-proxy/.env
# Update: DATA_PATH=/var/lib/bunny-api-proxy/proxy.db
# Change: ADMIN_PASSWORD and ENCRYPTION_KEY
```

**6. Download binary:**

```bash
# Option A: From releases
sudo wget https://github.com/sipico/bunny-api-proxy/releases/download/v2026.01.2/bunny-api-proxy-linux-amd64 \
  -O /usr/local/bin/bunny-api-proxy
sudo chmod 755 /usr/local/bin/bunny-api-proxy

# Option B: Build from source
cd /home/user/bunny-api-proxy
go build -o bunny-api-proxy ./cmd/bunny-api-proxy
sudo cp bunny-api-proxy /usr/local/bin/
sudo chmod 755 /usr/local/bin/bunny-api-proxy
```

**7. Copy systemd service file:**

```bash
sudo cp systemd/bunny-api-proxy.service /etc/systemd/system/
sudo systemctl daemon-reload
```

**8. Enable and start:**

```bash
sudo systemctl enable bunny-api-proxy
sudo systemctl start bunny-api-proxy
```

**9. Verify:**

```bash
# Check status
sudo systemctl status bunny-api-proxy

# View logs
sudo journalctl -u bunny-api-proxy -f

# Test health endpoint
curl http://localhost:8080/health

# Check database
ls -lh /var/lib/bunny-api-proxy/proxy.db
```

### Systemd Management

```bash
# Start service
sudo systemctl start bunny-api-proxy

# Stop service
sudo systemctl stop bunny-api-proxy

# Restart (reload config)
sudo systemctl restart bunny-api-proxy

# View status
sudo systemctl status bunny-api-proxy

# View logs (realtime)
sudo journalctl -u bunny-api-proxy -f

# View last 50 lines
sudo journalctl -u bunny-api-proxy -n 50

# View logs from past 2 hours
sudo journalctl -u bunny-api-proxy --since "2 hours ago"

# Enable auto-start on reboot
sudo systemctl enable bunny-api-proxy

# Disable auto-start
sudo systemctl disable bunny-api-proxy
```

### Systemd with Reverse Proxy

Add Nginx/Apache in front for HTTPS/load balancing:

```bash
# Install Nginx
sudo apt-get install -y nginx

# Enable Nginx
sudo systemctl enable nginx
sudo systemctl start nginx

# Configure Nginx reverse proxy
# /etc/nginx/sites-available/bunny-api-proxy
upstream bunny_proxy {
    server localhost:8080;
}

server {
    listen 80;
    server_name api.example.com;

    location / {
        proxy_pass http://bunny_proxy;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }
}

# Enable site
sudo ln -s /etc/nginx/sites-available/bunny-api-proxy /etc/nginx/sites-enabled/
sudo nginx -s reload

# Set up HTTPS with Certbot
sudo apt-get install -y certbot python3-certbot-nginx
sudo certbot --nginx -d api.example.com
```

## Kubernetes Deployment

For cloud-native deployments with orchestration, self-healing, and scaling.

### Prerequisites

- Kubernetes cluster 1.20+ (EKS, GKE, AKS, or self-managed)
- `kubectl` configured to access cluster
- Ingress controller (nginx, Traefik, etc.)
- cert-manager installed (for automatic TLS)
- Persistent volume provisioner (built-in for most clouds)

### Installation Steps

**1. Create namespace (optional):**

```bash
kubectl create namespace bunny-api-proxy
# Then update namespace in all YAML files
```

**2. Create Secret with credentials:**

```bash
# Create ConfigMap for non-secret configuration
kubectl create configmap bunny-api-proxy-config \
  --from-literal=LOG_LEVEL=info \
  --from-literal=LISTEN_ADDR=:8080 \
  --from-literal=DATABASE_PATH=/data/proxy.db

# Verify
kubectl get configmap bunny-api-proxy-config
```

**3. Create PersistentVolumeClaim:**

```bash
kubectl apply -f examples/kubernetes/pvc.yaml
kubectl get pvc bunny-api-proxy-pvc
```

**4. Deploy application:**

```bash
kubectl apply -f examples/kubernetes/deployment.yaml
kubectl apply -f examples/kubernetes/service.yaml

# Wait for deployment
kubectl rollout status deployment/bunny-api-proxy

# Check pods
kubectl get pods -l app=bunny-api-proxy
```

**5. Set up HTTPS (with cert-manager):**

```bash
# Install cert-manager (if not already installed)
helm repo add jetstack https://charts.jetstack.io
helm repo update
helm install cert-manager jetstack/cert-manager \
  --namespace cert-manager --create-namespace \
  --set installCRDs=true

# Create ClusterIssuer for Let's Encrypt
kubectl apply -f - <<EOF
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: letsencrypt-prod
spec:
  acme:
    server: https://acme-v02.api.letsencrypt.org/directory
    email: admin@example.com
    privateKeySecretRef:
      name: letsencrypt-prod-key
    solvers:
    - http01:
        ingress:
          class: nginx
EOF

# Create Ingress
kubectl apply -f examples/kubernetes/ingress.yaml

# Check certificate creation
kubectl get certificate bunny-api-proxy-cert
```

**6. Verify deployment:**

```bash
# Check pod status
kubectl get pod -l app=bunny-api-proxy

# View logs
kubectl logs -l app=bunny-api-proxy -f

# Port-forward for testing
kubectl port-forward service/bunny-api-proxy 8080:8080
curl http://localhost:8080/health

# Check Ingress
kubectl get ingress
kubectl describe ingress bunny-api-proxy-ingress

# Check TLS certificate
kubectl get certificate bunny-api-proxy-cert
```

### Kubernetes Management

```bash
# View deployments
kubectl get deployments
kubectl describe deployment bunny-api-proxy

# View pods
kubectl get pods
kubectl describe pod <pod-name>

# View logs
kubectl logs <pod-name>
kubectl logs -f <pod-name>  # Follow logs
kubectl logs <pod-name> -p  # Previous pod logs

# Port forwarding (local testing)
kubectl port-forward service/bunny-api-proxy 8080:8080
curl http://localhost:8080/health

# Execute command in pod
kubectl exec -it <pod-name> -- /bin/sh
kubectl exec <pod-name> -- curl http://localhost:8080/health

# Update image (when deploying new version)
kubectl set image deployment/bunny-api-proxy \
  bunny-api-proxy=sipico/bunny-api-proxy:v2026.01.2 \
  --record

# Rollback to previous version
kubectl rollout undo deployment/bunny-api-proxy

# Scale replicas (not recommended with SQLite)
kubectl scale deployment bunny-api-proxy --replicas=3

# Delete deployment
kubectl delete deployment bunny-api-proxy
```

### Kubernetes on Different Clouds

**AWS EKS:**
```bash
# Create cluster (requires AWS CLI)
eksctl create cluster --name bunny-proxy --region us-east-1 --nodes 2

# Configure kubectl
aws eks update-kubeconfig --name bunny-proxy --region us-east-1

# Use AWS EBS for storage
# storageClassName: gp2 (or gp3 for newer clusters)
```

**Google GKE:**
```bash
# Create cluster
gcloud container clusters create bunny-proxy --zone us-central1-a --num-nodes 2

# Configure kubectl
gcloud container clusters get-credentials bunny-proxy --zone us-central1-a

# Use Google Persistent Disks
# storageClassName: standard (or premium-rwo)
```

**Azure AKS:**
```bash
# Create cluster
az aks create --resource-group my-rg --name bunny-proxy --node-count 2

# Configure kubectl
az aks get-credentials --resource-group my-rg --name bunny-proxy

# Use Azure Managed Disks
# storageClassName: default (or managed-premium)
```

## Security Recommendations

### Secrets Management

1. **Protect your bunny.net master API key**
   - Only use during bootstrap
   - Store in password manager
   - Never commit to Git

2. **Protect admin tokens**
   - Store securely after creation (shown only once)
   - Create separate tokens for different services
   - Rotate tokens periodically

3. **External secret managers** (production)
   - HashiCorp Vault
   - AWS Secrets Manager
   - Google Secret Manager
   - Azure Key Vault

### Network Security

1. **Firewall rules**
   - Only expose needed ports (80, 443)
   - Restrict admin access by IP
   - Use security groups/network policies

2. **TLS/HTTPS everywhere**
   - Use reverse proxy (Traefik, Nginx) for TLS
   - Let's Encrypt for free certificates
   - Kubernetes: cert-manager for automation

3. **Rate limiting**
   - Nginx: `limit_req` directive
   - Traefik: middleware
   - Kubernetes: NetworkPolicy resources

### Access Control

1. **Authentication**
   - Protect admin tokens securely
   - API key rotation
   - Use separate tokens per service

2. **Authorization**
   - Scoped API keys (only needed permissions)
   - RBAC for Kubernetes
   - Network policies for pod-to-pod access

3. **Audit logging**
   - Enable access logs
   - Monitor for suspicious patterns
   - Archive logs for compliance

### System Hardening

1. **Container security**
   - Run as non-root user
   - Drop unnecessary capabilities
   - Read-only root filesystem

2. **Operating system**
   - Keep Linux kernel updated
   - Regular security patches
   - Minimal software installation

3. **Monitoring & alerting**
   - Health checks for auto-recovery
   - Resource limits to prevent DoS
   - Prometheus metrics for monitoring

## Backup & Restore

### Backup Procedures

**Docker Compose:**
```bash
# Backup database file
docker cp bunny-api-proxy:/data/proxy.db ./backup/proxy.db.$(date +%Y%m%d)

# Backup entire volume
docker run --rm -v bunny_data:/data -v $(pwd)/backup:/backup \
  busybox tar czf /backup/proxy.db.tar.gz /data/proxy.db

# Backup to external storage (S3 example)
aws s3 cp backup/proxy.db.tar.gz s3://my-bucket/backups/
```

**Systemd:**
```bash
# Backup database
sudo cp /var/lib/bunny-api-proxy/proxy.db \
  /backup/proxy.db.$(date +%Y%m%d)

# Backup with rsync
rsync -avz /var/lib/bunny-api-proxy/ /backup/bunny-proxy/

# Backup to remote server
rsync -avz /var/lib/bunny-api-proxy/ user@backup-server:/backups/bunny-proxy/
```

**Kubernetes:**
```bash
# Create volumesnapshot (if supported)
kubectl apply -f - <<EOF
apiVersion: snapshot.storage.k8s.io/v1
kind: VolumeSnapshot
metadata:
  name: bunny-api-proxy-snapshot-$(date +%Y%m%d)
spec:
  source:
    persistentVolumeClaimName: bunny-api-proxy-pvc
EOF

# Backup via pod
kubectl exec -it <pod-name> -- tar czf /data/backup.tar.gz /data/proxy.db

# Copy out of pod
kubectl cp <pod-name>:/data/backup.tar.gz ./backup.tar.gz
```

### Restore Procedures

**Docker Compose:**
```bash
# Stop services
docker-compose down

# Restore database
docker cp ./backup/proxy.db.20240125 bunny-api-proxy:/data/proxy.db

# Start services
docker-compose up -d
```

**Systemd:**
```bash
# Stop service
sudo systemctl stop bunny-api-proxy

# Restore database
sudo cp /backup/proxy.db.20240125 /var/lib/bunny-api-proxy/proxy.db
sudo chown bunny-proxy:bunny-proxy /var/lib/bunny-api-proxy/proxy.db

# Start service
sudo systemctl start bunny-api-proxy
```

**Kubernetes:**
```bash
# Create new PVC from snapshot
kubectl apply -f - <<EOF
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: bunny-api-proxy-pvc-restored
spec:
  dataSource:
    name: bunny-api-proxy-snapshot-20240125
    kind: VolumeSnapshot
    apiGroup: snapshot.storage.k8s.io
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 1Gi
EOF

# Update deployment to use new PVC
kubectl patch deployment bunny-api-proxy -p \
  '{"spec":{"template":{"spec":{"volumes":[{"name":"data","persistentVolumeClaim":{"claimName":"bunny-api-proxy-pvc-restored"}}]}}}}'
```

## Troubleshooting

### Common Issues

**Port already in use:**
```bash
# Find process using port 8080
lsof -i :8080
netstat -tlnp | grep 8080

# Kill process
kill -9 <pid>
```

**Database locked:**
```bash
# SQLite can have write conflicts
# Solution: Ensure only one instance is running
# Docker: restart with `docker-compose restart`
# Systemd: restart with `systemctl restart bunny-api-proxy`
# Kubernetes: delete pod to force restart
```

**Connection refused:**
```bash
# Check if service is running
docker-compose ps
systemctl status bunny-api-proxy
kubectl get pods

# Check logs
docker-compose logs bunny-api-proxy
journalctl -u bunny-api-proxy
kubectl logs <pod-name>

# Test connectivity
curl http://localhost:8080/health
```

**Certificate issues:**
```bash
# Check certificate status
kubectl get certificate bunny-api-proxy-cert
kubectl describe certificate bunny-api-proxy-cert

# Check cert-manager logs
kubectl logs -n cert-manager -l app=cert-manager -f

# Force certificate renewal
kubectl delete certificate bunny-api-proxy-cert
# cert-manager will automatically recreate it
```

**High memory usage:**
```bash
# Check resource usage
docker stats
kubectl top pod <pod-name>
systemctl status bunny-api-proxy

# Increase limits in deployment/docker-compose
# Add memory limit in config
# Investigate for memory leaks in application

# Restart to clear memory
docker-compose restart
systemctl restart bunny-api-proxy
kubectl delete pod <pod-name>
```

### Debug Commands

```bash
# Docker Compose
docker-compose ps
docker-compose logs bunny-api-proxy
docker-compose exec bunny-api-proxy /bin/sh
docker inspect <container-id>

# Systemd
systemctl status bunny-api-proxy
journalctl -u bunny-api-proxy -n 100
ss -tlnp | grep 8080
ls -lh /var/lib/bunny-api-proxy/

# Kubernetes
kubectl get pods
kubectl describe pod <pod-name>
kubectl logs <pod-name>
kubectl exec <pod-name> -- ps aux
kubectl top nodes
kubectl top pods
```

## Configuration Reference

### Environment Variables

See `../.env.example` for complete configuration options:

- `LOG_LEVEL` - Logging level: debug, info, warn, error (default: info)
- `LISTEN_ADDR` - Address to listen on (default: :8080)
- `DATABASE_PATH` - SQLite database file path (default: /data/proxy.db)
- `BUNNY_API_URL` - bunny.net API URL (default: https://api.bunny.net)

### Version Information

All examples reference version `2026.01.2`:
- Docker image: `sipico/bunny-api-proxy:latest` or `sipico/bunny-api-proxy:v2026.01.2`
- Binary releases: https://github.com/sipico/bunny-api-proxy/releases

## Next Steps

1. **Choose deployment method** based on your infrastructure
2. **Generate secure credentials** (ADMIN_PASSWORD, ENCRYPTION_KEY)
3. **Configure networking** (firewall, reverse proxy, TLS)
4. **Set up monitoring** (health checks, logs, alerting)
5. **Plan backup strategy** (automated backups, testing restores)
6. **Document your setup** (runbooks, recovery procedures)

For questions or issues, see the main [README.md](../README.md) and [ARCHITECTURE.md](../ARCHITECTURE.md).
