# Keycloak (lab Helm chart)

Single-replica Keycloak using `start-dev`. Works on any Kubernetes cluster.

This is for evaluation, not production HA.

## Install

```bash
helm upgrade --install keycloak . -n keycloak --create-namespace \
  --set admin.password='YourLabPassword'
```

## Access

```bash
kubectl get svc -n keycloak
# If service.type=LoadBalancer: http://<EXTERNAL-IP>:8080
```

Admin console: username `admin` (or `admin.username`), password from `--set` / Secret.

## Realm groups this service expects

Create these (see `scripts/setup.sh` or the Compose realm import):

- `admin-eligible`
- `admin-active`
- `admin-permanent`
- `break-glass`

## Not for production

`start-dev` uses an embedded database. Move to `start` + PostgreSQL before any real use.
