# Phase 1 — Findings & validation record

**Date:** 2026-08-02  
**Status:** COMPLETE  
**Environment:** Same lab as Phase 0 (`keycloak` namespace on `kc-pim-aks`)

## What was built
- Go PIM API: `phase1/` (`cmd/pim`, `internal/{api,config,keycloak,store,worker}`)
- Dockerfile + ACR image: `kcpimlabacr082.azurecr.io/pim:0.1.0`
- Helm chart: `charts/pim` (Deployment, Service, PVC in `keycloak` ns)
- Lab auth: `X-Actor-Username` header (OIDC UI deferred to Phase 3)

## Flow proved
```text
admin-eligible → POST /api/v1/requests (justification)
       │
       ├─ no approve within request TTL → status expired
       │
       └─ admin-permanent Approves
              │
              v
        add user to admin-active → status approved (active TTL)
              │
              ├─ early release → removed from admin-active
              └─ TTL / reconcile → removed from admin-active (released)
```

Defaults: **request TTL 1h**, **active TTL 9h**, poll/reconcile interval 30s.

## Proofs
| Test | Result |
| --- | --- |
| Eligible creates request | PASS (`admin1`) |
| Non-permanent cannot approve | PASS (`admin1` → HTTP 403) |
| Permanent approves → `admin-active` | PASS (`permanent1`) |
| Early release | PASS |
| Pending auto-expire | PASS (lab TTL 45s stand-in for 1h) |
| Active auto-revoke | PASS (lab TTL 60s stand-in for 9h) |
| Restart + reconcile still revokes | PASS (delete pod mid-grant) |
| Runs as Deployment + image | PASS |

## Issues found & fixed
1. **Docker Desktop unavailable** — used `az acr build` + ACR `kcpimlabacr082` instead of local docker.  
2. **SQLite CrashLoop** — PVC mounted over `/data` not writable by UID 65532; fixed with pod `fsGroup: 65532`.  
3. **Go compile** — renamed Keycloak client token field/method clash (`cachedToken` / `accessToken()`).

## API cheat sheet
```http
POST /api/v1/requests
X-Actor-Username: admin1
{"justification":"..."}

POST /api/v1/requests/{id}/approve
X-Actor-Username: permanent1

POST /api/v1/requests/{id}/release
X-Actor-Username: admin1
```

```powershell
kubectl -n keycloak port-forward svc/pim 8081:8080
```

## Exit
Phase 1 DofD met. Next: Phase 2 (audit & visibility).  
Note: destroy AKS **and** ACR when tearing down lab to avoid ongoing cost.
