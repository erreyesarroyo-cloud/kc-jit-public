# Phase 3 — Findings (thin UI)

**Date:** 2026-08-15  
**Status:** **PASS** (DofD met)  
**Image:** `kcpimlabacr319.azurecr.io/pim:0.2.3`  
**Namespace:** `keycloak-pim`

## DofD checklist

| # | Criterion | Status |
| --- | --- | --- |
| 1 | OIDC login via Keycloak (`pim-web` client) | **PASS** — PKCE S256; Sign in → Keycloak → callback |
| 2 | Eligible request / pending / early-release UI | **PASS** — `admin1` create + release in UI |
| 3 | Permanent approve queue in UI | **PASS** — `permanent1` Approve in UI |
| 4 | Countdown for request TTL / active grant | **PASS** — pending ~1h; active ~9h |
| 5 | Non-eligible / non-permanent empty/denied | **PASS** — `/api/v1/me` gates form/Approve; API enforces |

## Lab validation (2026-08-15)

End-to-end browser + Keycloak membership check:

| Step | Result |
| --- | --- |
| `admin1` creates request (`test2` / `af577b64…`) | pending |
| `permanent1` approves | `request_approved` + `grant_active` |
| Keycloak `admin1` groups after approve | `admin-active`, `admin-eligible` |
| `admin-active` members | `admin1` |
| PIM `/me` for `admin1` | `active: true` |
| Earlier run: early release | removed from `admin-active` (groups back to eligible only) |

Audit sample for approved grant:

- `request_created` (admin1)
- `request_approved` / `grant_active` (permanent1 → admin1)
- (optional) `early_release` removes `admin-active`

## How to open UI

```powershell
kubectl -n keycloak-pim port-forward svc/pim 8081:8080
# http://127.0.0.1:8081
# OIDC: Sign in — fixtures use password 123456789
#   admin1 / admin2 = eligible requester
#   permanent1 = approver
# Lab fallback: Actor + Continue (X-Actor-Username) if needed
```

## Fixes landed this phase

| Image | Fix |
| --- | --- |
| `0.2.2` | OIDC PKCE (`code_challenge` / `code_verifier`) — Keycloak required S256 |
| `0.2.3` | Sign out clears lab Actor `localStorage` (was appearing still signed in) |

## Notes

- `pim-web` from Phase 0 `setup.sh`; redirect URIs include `http://127.0.0.1:8081/auth/callback`.
- Browser OIDC uses `KC_PUBLIC_URL` (Keycloak LB); server Admin API uses in-cluster DNS.
- TTL: request must be approved within **1h**; grant lasts **9h** (or early release).
- MVP = Phases **1 + 2 + 3** complete. Next: Phase 4 (notifications).
