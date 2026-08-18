# Keycloak PIM Extension

A Privileged Identity Management layer on top of Keycloak. No one holds standing admin access — instead, users request temporary elevation that is approved, time-boxed, and automatically revoked.

## How It Works

```
Eligible user  ──requests elevation──►  Approval queue
                                             │
                              approved within 1 hour or expires
                                             │
                                             ▼
                                    Role granted (9 hours)
                                             │
                                     timer expires or
                                     user releases early
                                             │
                                             ▼
                                      Role auto-revoked
```

### Groups

| Group | Purpose |
|-------|---------|
| `admin-eligible` | Can request elevation |
| `admin-permanent` | Can approve requests (never auto-revoked) |
| `admin-active` | Currently elevated — membership is temporary |
| `break-glass` | Emergency access, always monitored |

### Key Rules

- **No standing admin access.** Eligible users must request it each time.
- **Time-bound.** Grants last 9 hours, then auto-revoke fires regardless of app state.
- **Approval required.** Only `admin-permanent` members can approve.
- **Request TTL.** Unapproved requests expire after 1 hour.
- **Early release.** Users can drop privileges before the timer runs out.
- **Startup reconciliation.** On restart, the app revokes anything past expiry — no orphaned access.
- **Audit trail.** Every request, approval, and revocation is logged with actor, justification, and timestamp.

### Elevation Ledger

Every elevation records: who requested, when, why (justification), who approved, when they approved, and any approver notes. This is the primary accountability control.

## Project Structure

```
phase0/   – Foundations: Keycloak setup, groups, service account plumbing
phase1/   – Core grant/revoke engine with time-bound scheduler
phase2/   – Audit logging and tamper-resistant event capture
phase3/   – Self-service web UI for requesting and approving
phase4/   – Notifications (webhook/email to approvers)
phase5/   – Step-up security (MFA on activation) — skipped in this lab
phase6/   – Chaos-tested revoke + elevation ledger
charts/   – Helm charts for deployment
scripts/  – Bootstrap and utility scripts
infra/    – Infrastructure definitions
```

## Quick Start

```bash
# Phase 0 bootstrap (sets up Keycloak + groups + service account)
./scripts/bootstrap-phase0.sh
```

## Minimum Viable PIM

Phases 1 + 2 + 3: reliable grant/revoke engine, audit trail, and a web UI for users and approvers. Everything else hardens and extends from there.
