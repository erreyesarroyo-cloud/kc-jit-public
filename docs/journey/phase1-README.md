# Phase 1 — PIM API

Go service that implements time-bound Keycloak elevation:

`admin-eligible` → request → `admin-permanent` approves → `admin-active` (9h) → revoke

## API (lab auth via header)

| Method | Path | Header | Notes |
| --- | --- | --- | --- |
| POST | `/api/v1/requests` | `X-Actor-Username: admin1` | body: `{"justification":"..."}` |
| POST | `/api/v1/requests/{id}/approve` | `X-Actor-Username: permanent1` | must be admin-permanent |
| POST | `/api/v1/requests/{id}/release` | actor = subject or permanent | early release |
| GET | `/api/v1/requests` | — | list |
| GET | `/healthz` | — | probe |

## Build image (local)

```bash
docker build -t keycloak-jit-access:0.1.0 .
```

## Deploy (after Phase 0 on cluster)

```bash
# secret from scripts/.env PIM_CLIENT_SECRET (PIM ns, not Keycloak ns)
kubectl create namespace keycloak-pim --dry-run=client -o yaml | kubectl apply -f -
kubectl -n keycloak-pim create secret generic pim-service-credentials \
  --from-literal=client-secret="$PIM_CLIENT_SECRET" \
  --dry-run=client -o yaml | kubectl apply -f -

# load image into AKS node if not using a registry (lab): use ACR or docker save/import
helm upgrade --install pim ../deploy/helm/pim -n keycloak-pim \
  --set image.repository=keycloak-jit-access --set image.tag=0.1.0 \
  --set sessionSecret="$(openssl rand -hex 32)"
```

## Smoke test

```bash
kubectl -n keycloak-pim port-forward svc/pim 8081:8080
curl -s -X POST http://127.0.0.1:8081/api/v1/requests \
  -H 'Content-Type: application/json' \
  -H 'X-Actor-Username: admin1' \
  -d '{"justification":"lab test"}'
```
