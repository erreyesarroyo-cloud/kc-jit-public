# Phase 2 — Findings

**Date:** 2026-08-15  
**Environment:** `kc-pim-aks` — Keycloak in `keycloak`, PIM in `keycloak-pim`  
**Image:** `kcpimlabacr319.azurecr.io/pim:0.2.0`

## Done

| # | Criterion | Result |
| --- | --- | --- |
| 1 | Lifecycle logged (request → approve → grant → release/expire) | PASS |
| 2 | Approvals record actor + justification | PASS (`request_approved`, `grant_active`) |
| 3 | Break-glass flagged | PASS (`break_glass_session` + `BreakGlass=true`; log ALERT) |
| 4 | Audit query API | PASS `GET /api/v1/audit?breakGlass=&requestId=&actor=&limit=` |
| 5 | Event Listener SPI | Stretch — not done |

## Smoke (header auth)

- Create as `admin1` → pending  
- Approve as `admin1` → **403**  
- Approve as `permanent1` → approved + audit events  
- `/api/v1/me` as `breakglass1` → `breakGlass=true` + audit flag  
- Early release as `admin1` → `early_release` audit  

## Notes

- Audit stored in same SQLite PVC as requests (`audit_events` table).  
- Break-glass alert today = audit flag + server log line (no email yet — Phase 4).
