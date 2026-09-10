# Phase 0 — Findings & validation record

**Date:** 2026-08-02  
**Status:** COMPLETE  
**Environment:** Azure AKS Free tier (`kc-pim-aks` / `kc-pim-rg`), Keycloak 26.3.3 (Helm `start-dev`)

## What was built
- Terraform: `deploy/infra/aks` (AKS Free, 1× Standard_B2s)
- Helm chart: `deploy/helm/keycloak`
- Scripts: `scripts/setup.sh`, `scripts/plumbing_test.sh`, `scripts/run-lab-setup.sh`
- Optional AKS lab bootstrap: `deploy/infra/aks/bootstrap.sh`

## Locked group model (fixtures)
| Group | Purpose | Standing role |
| --- | --- | --- |
| `admin-eligible` | Can request elevation | none |
| `admin-active` | JIT elevation target (empty at rest) | `realm-admin` (+ lab `pim-test-role`) |
| `admin-permanent` | Approvers only | `realm-admin` |
| `break-glass` | Emergency admin | `realm-admin` |

| User | Group | Password (lab only) |
| --- | --- | --- |
| `admin1`, `admin2`, `superadmin1` | `admin-eligible` | `123456789` |
| `permanent1` | `admin-permanent` | `123456789` |
| `breakglass1` | `break-glass` | `123456789` |

Also: realm `pim-test`, client `pim-service` (least-privilege: view/manage/query users, view-realm).

## Proofs (plumbing_test PASS)
1. `pim-service` obtains client-credentials token  
2. Grant `pim-test-role` to `admin1` → present  
3. Revoke → gone  
4. Break-glass authenticates and can call Admin API (HTTP 200)  
5. `permanent1` in `admin-permanent`; eligibles not in `admin-active` at rest  
6. Service account can add/remove user on `admin-active`  

## Ops notes
- Daily lab pattern: `deploy/infra/aks/bootstrap.sh` (or TF + Helm + `run-lab-setup.sh`) → work → `terraform destroy`  
- Keycloak health probes must use management port **9000** (KC 25+)  
- Admin console lab password default: `admin` / `ChangeMe-LabOnly!`  
- Do not commit `.env` (contains `PIM_CLIENT_SECRET`)

## Exit
Phase 0 DofD met. Ready for Phase 1 engine against this fixture model.
