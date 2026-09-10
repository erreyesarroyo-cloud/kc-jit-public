# Keycloak PIM Extension — Project Phases

A Privileged Identity Management (PIM) capability for Keycloak admin accounts,
inspired by Azure PIM: just-in-time, time-bound, self-service (and approved)
activation of admin roles, with tamper-proof audit.

## Guiding Principle

> Ship the **enforcement + revoke** skeleton before the **workflow polish**.
> A PIM that grants reliably and revokes reliably — even with a crude UI — is
> more valuable than a pretty approval UI that leaks standing access.

## Locked Design Decisions

| Rule | Decision |
|------|----------|
| Lead self-approval | Allowed (accepted, calculated risk) — but flagged + alerted |
| Request validity (time to approve) | 1 hour — request expires if not approved |
| Role activation duration | 9 hours once activated |
| Approval fallback | Leads -> DevOps -> Site lead (escalation) |
| Primary control (given self-approval) | Tamper-proof audit + alerting |

### Two Timers (distinct)

```
Request created --(must be approved within 1h)--> Approved
                                                     |
                                                     v
                                              Role granted
                                                     |
                                            (active for 9h)
                                                     v
                                              Auto-revoked
```

---

## Phase 0 — Foundations & Decisions
*Goal: no surprises later.*

- Finalize scope: which realm(s), which admin roles are PIM-managed
  (`realm-admin`, `manage-users`, `master` admin, etc.).
- Define the least-privilege service account for the PIM app
  (only role-assignment scope; run from `master`/dedicated realm).
- Choose stack: PIM app language, durable store (Postgres etc.),
  notification channel (Slack/Teams/email).
- Stand up break-glass account (permanent admin, monitored) BEFORE removing
  standing access from anyone.
- **Exit criteria:** service account can grant/revoke a test role via Admin
  REST API; break-glass verified.

## Phase 1 — Core Grant/Revoke Engine (safety-critical core)
*Goal: reliably activate and, above all, reliably de-activate.*

- Eligible-vs-active model: strip standing admin roles, define eligible list.
- Activate endpoint: grant role via Admin REST API, persist `expires_at`
  (durable).
- Auto-revoke scheduler at 9h.
- Startup reconciliation: on boot, revoke anything past expiry
  (heals missed revokes).
- Failure alerting on revoke errors + retry.
- Short admin token lifespan configured so revocation takes effect fast.
- **Exit criteria:** grant expires reliably across app restarts and Keycloak
  downtime; no orphaned standing access. *Do not rush this phase.*

## Phase 2 — Audit & Visibility
*Goal: everything is recorded tamper-proof (primary control, given self-approval).*

- Java Event Listener SPI -> external append-only audit store.
- PIM app logs full lifecycle: request -> approve -> grant -> expire/release.
- Flag + alert on self-approval and on any site-lead (break-glass) approval.
- Basic audit view / query.
- **Exit criteria:** every activation is traceable; self-approvals visibly
  alerted; audit survives an activated admin (cannot be erased from console).

## Phase 3 — Self-Service Activation UX
*Goal: users can request without ops involvement.*

- Web UI (or CLI): list eligible roles, request with justification + duration.
- Request TTL (1h to get approved) + role duration (9h).
- Early release ("deactivate now") button.
- "My active grants" view with countdown.
- **Exit criteria:** a dev can self-activate (auto-approve path) end-to-end,
  see it expire, and release early.

## Phase 4 — Approval Workflow
*Goal: the tiered approval + escalation model.*

- Approver tiers backed by Keycloak groups (leads -> devops -> site-lead).
- State machine: pending -> approved/denied -> active -> expired.
- Self-approval allowed (per decision) but flagged.
- Time-based escalation (e.g., 15m -> next tier).
- Notifications to current tier's approvers.
- Auto-deny if request TTL (1h) elapses with no approval.
- **Exit criteria:** full request -> escalate -> approve -> grant flow works;
  escalation fires correctly; peer/self approval behaves as configured.

## Phase 5 — Step-Up Security
*Goal: harden the activation moment.*

- MFA / re-auth on activation via Keycloak auth flow (`acr` / `max_age`).
- Optional dual control for the highest roles (e.g., `master` admin = 2 approvers).
- Per-role config (durations, approvals_required, self-approval on/off).
- **Exit criteria:** activating a sensitive role forces fresh MFA; crown-jewel
  roles require configured approvals.

## Phase 6 — Hardening & Operationalize
*Goal: production-ready.*

- Load/failure testing of revoke path (Keycloak down, DB down, restart storms).
- Monitoring/dashboards: active grants, pending requests, revoke failures,
  self-approval rate.
- Runbooks: break-glass procedure, "revoke everything" kill-switch, PIM app
  outage handling.
- Access review / reporting (who activated what, how often) — Azure PIM
  "access review" analog.
- Security review / threat model sign-off.
- **Exit criteria:** on-call can operate it; failure modes documented; sign-off
  to remove standing admin access for real.

## Phase 7 (Optional) — Advanced Parity
*Goal: closer to full Azure PIM.*

- Scheduled access reviews / recertification.
- On-call/calendar-aware escalation (availability-based instead of time-based).
- Analytics, anomaly alerts (unusual activation patterns).
- Multi-realm support with per-realm config.

---

## Sequencing Rationale

| Phase | De-risks |
|-------|----------|
| 1 (revoke core) | The #1 failure mode: standing access that never gets removed |
| 2 (audit) | The only real control since self-approval is allowed |
| 3-4 (UX/approvals) | Adoption & workflow — safe to iterate once core is solid |
| 5 (step-up/dual) | Hardening the sensitive edges |
| 6 (ops) | Trust to actually cut over |

**Minimum Viable PIM** = Phases 1 + 2 + 3 (reliable grant/revoke + audit +
self-service). Phases 4-6 make it robust and auditable enough to be the real
access-control gate.

---

## Reference: Config Shape

```yaml
roles:
  realm-admin:
    request_ttl: 1h        # approval must happen within this
    active_duration: 9h    # role held once activated
    approvals_required: 1
    allow_self_approval: true
    on_self_approval: notify   # flag + alert, don't block
    early_release: true        # user can deactivate before 9h
    approver_tiers:
      - group: pim-approvers-leads
        escalate_after: 15m
      - group: pim-approvers-devops
        escalate_after: 15m
      - group: pim-approvers-site-lead   # break-glass approver
  master-admin:
    request_ttl: 1h
    active_duration: 9h
    approvals_required: 2                 # dual control for crown jewels
    allow_self_approval: false
    approver_tiers:
      - group: pim-approvers-leads
      - group: pim-approvers-site-lead
```

## Reference: Architecture Split

| Concern | Where |
|---------|-------|
| Approval tiers, peer rules, escalation timers | External PIM app (its own DB) |
| Who is a lead / devops / site-lead | Keycloak groups (source of truth) |
| Granting the role once approved | Admin REST API |
| Time-bound auto-revoke | Scheduler + startup reconciliation (PIM app) |
| Tamper-proof audit | Event Listener SPI (Java) -> external append-only store |
| Step-up MFA on activation | Keycloak auth flow (`acr` / `max_age`) |
