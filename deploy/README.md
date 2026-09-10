# Deploy

Public install paths, in order:

1. **Docker Compose** at the repo root (`docker compose up --build`) — try the full flow locally.
2. **Helm** in [`helm/`](helm/) — run Keycloak and this service on any Kubernetes cluster.

## Helm

```bash
# Keycloak (dev/lab chart: start-dev, not production HA)
helm upgrade --install keycloak ./deploy/helm/keycloak -n keycloak --create-namespace \
  --set admin.password='YourLabPassword'

# This service — set image.repository and sessionSecret for your environment
helm upgrade --install pim ./deploy/helm/pim -n keycloak-pim --create-namespace \
  --set image.repository=keycloak-jit-access \
  --set image.tag=0.1.0 \
  --set sessionSecret="$(openssl rand -hex 32)" \
  --set publicURL='https://pim.example.com' \
  --set keycloak.publicURL='https://keycloak.example.com'
```

Create the `pim-service-credentials` and `pim-oidc-credentials` secrets before installing the PIM chart. See the root [README](../README.md) and [SECURITY.md](../SECURITY.md).

## Optional: original AKS lab

[`infra/aks/`](infra/aks/) is Terraform from the first lab cluster. It is **not** required to use or share this project.
