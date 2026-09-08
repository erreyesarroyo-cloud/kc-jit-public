# Keycloak (lab Helm chart)

Single-replica Keycloak using `start-dev` for the PIM **access** cluster on free-tier AKS.

## Install

```powershell
# after AKS kubeconfig is configured
helm upgrade --install keycloak . -n keycloak --create-namespace `
  --set admin.password="YourLabPassword"
```

## Access

```powershell
kubectl get svc -n keycloak
# EXTERNAL-IP from the LoadBalancer → http://<EXTERNAL-IP>:8080
```

Admin console: username `admin` (or `admin.username`), password from `--set` / Secret.

## Next (Phase 0)

Create groups on this Keycloak:

- `admin-eligible`
- `admin-active` (standing master role)
- `admin-approved` (JIT master role via group)
- `break-glass` (standing master role)

## Not for production

`start-dev` uses an embedded database. Move to `start` + PostgreSQL before any real use.
