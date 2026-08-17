# Kubernetes Deployment Guide for Casbin Gateway

Casbin Gateway uses SQLite by default. The Kubernetes manifests run one Gateway replica and persist the SQLite database in the `caswaf-data` persistent volume claim, so no database service is required.

## Prerequisites

- A Kubernetes cluster with a default storage class
- `kubectl` configured for the cluster
- A reachable Casdoor instance

## Configure Casdoor

Replace the two placeholders in `secret.yaml` with the client ID and client secret from your Casdoor application. If Casdoor is not available at the default in-cluster address, update `casdoorEndpoint` in `configmap.yaml`.

## Deploy

From the `k8s` directory, run:

```bash
chmod +x deploy.sh
./deploy.sh
```

Alternatively, deploy all default resources with Kustomize:

```bash
kubectl apply -k k8s/
```

The default resources include the application, Casdoor configuration, a 1 Gi SQLite persistent volume claim, and the optional ingress manifest. They do not deploy MySQL.

## Access the application

For local access without an ingress controller:

```bash
kubectl port-forward svc/caswaf 17000:17000 -n caswaf
```

Then open `http://localhost:17000`. To use Ingress, change the host in `ingress.yaml` before deployment.

## Database configuration

The default database settings in `configmap.yaml` are:

```ini
driverName = sqlite
dataSourceName = /data/casbin-gateway.db
dbName =
```

The deployment mounts `caswaf-data` at `/data`. SQLite is intended for the default single-replica deployment.

To use an external MySQL or PostgreSQL database, change these settings in `configmap.yaml` and provide the appropriate connection string. The legacy `mysql.yaml` manifest is available as an optional example but is not part of the default Kustomize deployment.

## Verify and troubleshoot

```bash
kubectl get pods -n caswaf
kubectl get pvc -n caswaf
kubectl logs -f deployment/caswaf -n caswaf
```

If the pod is pending, check that the cluster has a default storage class capable of binding the `caswaf-data` claim. If authentication fails, verify the Casdoor endpoint, client ID, client secret, organization, and application in `configmap.yaml` and `secret.yaml`.

## Updating

Update the image and watch the rollout:

```bash
kubectl set image deployment/caswaf caswaf=casbin/caswaf:NEW_VERSION -n caswaf
kubectl rollout status deployment/caswaf -n caswaf
```

## Uninstall

Delete the namespace to remove all resources. This also deletes the SQLite persistent volume claim; whether the underlying volume is retained depends on the storage class reclaim policy.

```bash
kubectl delete namespace caswaf
```
