# Kubernetes Deployment Guide for casbin-gateway

This guide provides instructions for deploying casbin-gateway on Kubernetes.

## Prerequisites

- A running Kubernetes cluster (1.19+)
- `kubectl` configured to access your cluster
- A running Casdoor instance (can be in the same cluster or external)
- Basic understanding of Kubernetes resources

## Architecture

The deployment consists of:
- **casbin-gateway Application**: The main WAF application
- **MySQL Database**: Stores casbin-gateway configuration and data
- **Secrets**: Stores sensitive credentials (Casdoor client ID/secret, MySQL password)
- **ConfigMap**: Contains casbin-gateway configuration template
- **Services**: Exposes casbin-gateway and MySQL within the cluster
- **Ingress** (optional): Exposes casbin-gateway externally

## Quick Start

### 1. Deploy Casdoor (if not already deployed)

casbin-gateway requires Casdoor for authentication. If you don't have Casdoor deployed:

```bash
# Follow Casdoor's Kubernetes deployment guide:
# https://casdoor.org/docs/deployment/k8s
```

### 2. Configure Secrets

Edit `k8s/secret.yaml` and update the sensitive credentials:

```yaml
stringData:
  casdoor-client-id: "YOUR_ACTUAL_CLIENT_ID"
  casdoor-client-secret: "YOUR_ACTUAL_CLIENT_SECRET"
  mysql-password: "YOUR_STRONG_PASSWORD"
```

**Important**: 
- **REQUIRED**: You must replace all placeholder values before deployment
- Get `casdoor-client-id` and `casdoor-client-secret` from your Casdoor application settings
- Use a strong password for `mysql-password` (min 12 characters recommended)
- This password must match the one in `k8s/mysql.yaml`
- The deployment will fail with validation errors if placeholders are not replaced

**Security Best Practice**: Never commit actual secrets to version control. Consider using:
- [Sealed Secrets](https://github.com/bitnami-labs/sealed-secrets)
- [External Secrets Operator](https://external-secrets.io/)
- Cloud provider secret management (AWS Secrets Manager, Azure Key Vault, GCP Secret Manager)

### 3. Configure MySQL Password

Edit `k8s/mysql.yaml` and update the MySQL root password:

```bash
# Generate base64 encoded password (use the same password as in secret.yaml)
echo -n "YOUR_SECURE_PASSWORD" | base64
```

Then update the `mysql-root-password` in the Secret resource with the base64 value.

### 4. Configure Casdoor Endpoint (Optional)

If your Casdoor is not at `http://casdoor.casdoor-system.svc.cluster.local:8000`, edit `k8s/configmap.yaml`:

```yaml
casdoorEndpoint: http://your-casdoor-service:port
```

### 5. Deploy to Kubernetes

**Option A: Using the deployment script (Recommended)**

The easiest way to deploy casbin-gateway:

```bash
cd k8s
chmod +x deploy.sh
./deploy.sh
```

The script will:
- Validate your configuration
- Deploy MySQL and wait for it to be ready
- Deploy secrets and configuration
- Deploy casbin-gateway application
- Optionally deploy Ingress
- Show deployment status

**Option B: Using individual files**

```bash
# Create namespace and deploy MySQL
kubectl apply -f k8s/mysql.yaml

# Wait for MySQL to be ready
kubectl wait --for=condition=ready pod -l app=caswaf-mysql -n caswaf --timeout=300s

# Deploy Secrets
kubectl apply -f k8s/secret.yaml

# Deploy ConfigMap
kubectl apply -f k8s/configmap.yaml

# Deploy casbin-gateway
kubectl apply -f k8s/deployment.yaml

# (Optional) Deploy Ingress
kubectl apply -f k8s/ingress.yaml
```

**Option C: Using Kustomize**
```bash
kubectl apply -k k8s/
```

### 6. Verify Deployment

```bash
# Check if pods are running
kubectl get pods -n caswaf

# Check logs
kubectl logs -f deployment/caswaf -n caswaf

# Check services
kubectl get svc -n caswaf
```

### 7. Access casbin-gateway

If using Ingress:
```bash
# Update your DNS or /etc/hosts to point to your ingress controller IP
# Then access: http://caswaf.example.com
```

If using port-forward for testing:
```bash
kubectl port-forward svc/caswaf 17000:17000 -n caswaf
# Access: http://localhost:17000
```

## Configuration Details

### Secrets (`secret.yaml`)

Stores sensitive credentials:
- `casdoor-client-id`: Casdoor application client ID
- `casdoor-client-secret`: Casdoor application client secret  
- `mysql-password`: MySQL root password (must match mysql.yaml)

**Security Note**: Never commit actual secrets to version control. Use sealed-secrets, external secret operators, or other secret management solutions in production.

### ConfigMap (`configmap.yaml`)

Key configuration parameters:

| Parameter | Description | Default |
|-----------|-------------|---------|
| `httpport` | casbin-gateway HTTP port | `17000` |
| `runmode` | Run mode (dev/prod) | `prod` |
| `driverName` | Database driver | `mysql` |
| `dataSourceName` | MySQL connection string | Uses secrets substitution |
| `dbName` | Database name | `caswaf` |
| `casdoorEndpoint` | Casdoor API endpoint | Required |
| `casdoorInsecureSkipVerify` | Skip TLS verification for Casdoor | `true` |
| `clientId` | Casdoor application client ID | Uses secrets substitution |
| `clientSecret` | Casdoor application client secret | Uses secrets substitution |
| `casdoorOrganization` | Casdoor organization name | `built-in` |
| `casdoorApplication` | Casdoor application name | Required |

### MySQL Deployment (`mysql.yaml`)

- Uses MySQL 8.0.36 (latest stable)
- Persistent storage with PVC (10Gi)
- Includes health checks (liveness and readiness probes)
- Root password stored in Kubernetes Secret with obvious placeholder

### casbin-gateway Deployment (`deployment.yaml`)

Features:
- Init containers:
  - Wait for MySQL readiness
  - Substitute secrets into configuration file
- TCP-based liveness and readiness probes (no authentication required)
- Resource limits and requests
- Configuration mounted from ConfigMap with secret substitution

## Troubleshooting

### Common Issues

#### 1. "wait-for-it: timeout occurred after waiting 15 seconds for db:3306"

**Note**: This issue is fixed in the current deployment by using an init container instead of wait-for-it.

**Cause**: MySQL is not ready or not accessible

**Solution**:
```bash
# Check MySQL pod status
kubectl get pods -n caswaf -l app=caswaf-mysql

# Check MySQL logs
kubectl logs -n caswaf -l app=caswaf-mysql

# Verify MySQL service
kubectl get svc -n caswaf caswaf-mysql
```

#### 2. "casdoorsdk.GetCerts() error: Unauthorized operation"

**Cause**: Incorrect Casdoor configuration or credentials

**Solution**:
1. Verify Casdoor is accessible:
   ```bash
   kubectl run -it --rm debug --image=curlimages/curl --restart=Never -n caswaf -- \
     curl -v http://casdoor.casdoor-system.svc.cluster.local:8000
   ```

2. Verify `clientId` and `clientSecret` in ConfigMap match your Casdoor application

3. Ensure the Casdoor application is configured correctly:
   - Organization name matches `casdoorOrganization`
   - Application name matches `casdoorApplication`
   - Client ID and secret are correct

4. Check Casdoor logs for authentication errors

#### 3. Database Connection Issues

**Solution**:
```bash
# Test MySQL connection from casbin-gateway pod
kubectl exec -it deployment/caswaf -n caswaf -- sh
# Then inside the pod:
# nc -zv caswaf-mysql 3306
```

#### 4. Init Container Stuck

If the init container is stuck waiting for MySQL:
```bash
# Check init container logs
kubectl logs -n caswaf <pod-name> -c wait-for-mysql

# Force restart
kubectl rollout restart deployment/caswaf -n caswaf
```

### Viewing Logs

```bash
# casbin-gateway logs
kubectl logs -f deployment/caswaf -n caswaf

# MySQL logs
kubectl logs -f deployment/caswaf-mysql -n caswaf

# All logs in namespace
kubectl logs -f -n caswaf --all-containers=true
```

## Production Recommendations

1. **Use External MySQL**: For production, consider using a managed MySQL service (AWS RDS, Google Cloud SQL, etc.)

2. **Configure TLS**: 
   - Set `casdoorInsecureSkipVerify = false`
   - Use proper TLS certificates for Casdoor

3. **Resource Limits**: Adjust resource limits based on your traffic:
   ```yaml
   resources:
     requests:
       memory: "512Mi"
       cpu: "500m"
     limits:
       memory: "2Gi"
       cpu: "2000m"
   ```

4. **High Availability**:
   - Increase replicas for casbin-gateway
   - Use MySQL replication or managed service
   - Configure proper health checks

5. **Monitoring**: Set up monitoring and alerting:
   - Prometheus metrics
   - Application logs
   - Resource usage

6. **Backup**: Regular backups of MySQL data

7. **Security**:
   - Use Kubernetes Secrets for sensitive data
   - Enable RBAC
   - Network policies to restrict traffic
   - Regular security updates

## Updating casbin-gateway

```bash
# Update the image version in deployment.yaml, then:
kubectl set image deployment/caswaf caswaf=casbin/caswaf:NEW_VERSION -n caswaf

# Or apply updated deployment
kubectl apply -f k8s/deployment.yaml

# Check rollout status
kubectl rollout status deployment/caswaf -n caswaf
```

## Uninstall

```bash
# Delete all resources
kubectl delete -f k8s/ingress.yaml
kubectl delete -f k8s/deployment.yaml
kubectl delete -f k8s/configmap.yaml
kubectl delete -f k8s/mysql.yaml

# Or delete the entire namespace
kubectl delete namespace caswaf
```

## Advanced Configuration

### Using External MySQL

Edit `configmap.yaml`:
```yaml
dataSourceName: root:password@tcp(external-mysql.example.com:3306)/
```

Then skip deploying `mysql.yaml`.

### Using Redis for Sessions

Edit `configmap.yaml`:
```yaml
redisEndpoint: redis-host:6379
```

### Custom Domain

Edit `ingress.yaml`:
```yaml
spec:
  rules:
  - host: waf.yourdomain.com
```

## Support

For issues and questions:
- GitHub Issues: https://github.com/casbin/caswaf/issues
- Documentation: https://caswaf.org
- Casdoor Documentation: https://casdoor.org

## License

Apache-2.0
