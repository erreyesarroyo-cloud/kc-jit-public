# Phase 6 — Findings

**Date:** 2026-08-15  
**Status:** **PASS** (ledger + chaos revoke drills)  
**Image:** `kcpimlabacr319.azurecr.io/pim:0.5.1`

## Elevation ledger

| Column | Source |
| --- | --- |
| User | request username |
| Time of request | `CreatedAt` |
| Justification / details | required on create (≥ 8 chars) |
| Approver | `ApprovedBy` |
| Time of approval | `ApprovedAt` |
| Notes / guidance | required on approve (`ApprovalNotes`, ≥ 8 chars) |
| Status | pending / approved / released / … |

UI poll: full refresh (status + queue + ledger) every **10s**.

## Chaos revoke drills (automated 2026-08-15)

| Drill | Method | Result |
| --- | --- | --- |
| **A** PIM restart mid-grant | `kubectl delete pod` on PIM while `admin1` active → release | **PASS** — grant survived restart; revoke worked |
| **B** Keycloak unavailable | Set `KC_BASE_URL=http://127.0.0.1:9` on PIM (pod stays up) → release returns **502** → restore URL → release | **PASS** — revoke blocked during outage; membership remained; revoke succeeded after |
| **C** PIM scale 0→1 | Scale deploy to 0 then 1 mid-grant → release | **PASS** — PVC + Keycloak membership survived; revoke worked |

Final check: `admin1` **Currently active = false**; no approved orphans.

### Lab gotchas found during drills

1. **Do not `scale keycloak --replicas=0` on this lab.** Chart uses `start-dev` (ephemeral DB). Scaling to 0 wiped `pim-test`; had to re-run `phase0/run-lab-setup.sh` and refresh PIM client secrets.
2. Prefer **env blackhole** (`KC_BASE_URL=http://127.0.0.1:9`) or a NetworkPolicy for “KC down” without destroying realm data.
3. PIM PVC can keep old `approved` rows after a Keycloak wipe → DB says approved while group membership is gone. Release (or kill-switch) clears the row.

## Runbook notes

### Break-glass
- User `breakglass1` / group `break-glass` holds standing `realm-admin`.
- Use only for emergency Keycloak admin when PIM/approvers are unavailable.
- Sessions are audited (`break_glass_session`).

### Kill-switch — revoke all active
1. Keycloak Admin → realm `pim-test` → group **`admin-active`** → remove all members.
2. In PIM, **Release** any rows still `approved` (or leave them; reconcile/expiry will try to remove again).
3. Confirm with `/api/v1/me` as affected users (`active=false`) and Elevation ledger status.

## Still optional
- Dedicated “revoke all” API button (manual Keycloak + Release is enough for lab DofD #5).
