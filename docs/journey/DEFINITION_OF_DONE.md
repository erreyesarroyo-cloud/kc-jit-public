# Definition of Done (DofD) — Keycloak PIM Extension

Aligned with the locked lab design:

- `admin-eligible` requests  
- `admin-permanent` approves only  
- On approve → user moves to `admin-active` for **9 hours**  
- Request TTL **1 hour** (timeout if no decision)  
- Early release supported  
- PIM app (not Keycloak Admin console) owns the workflow  
- Access cluster: Keycloak + PIM (any Kubernetes, or Compose for local); app cluster later for demos  
- Namespaces: Keycloak in `keycloak`; PIM in **`keycloak-pim`** (separate)  

---

## Phase 0 — Foundations & plumbing

**Goal:** Cluster + Keycloak + Admin API grant/revoke proven.

| # | Done when |
| --- | --- |
| 1 | A Compose stack or any Kubernetes cluster can host the services |
| 2 | Keycloak is installed (Compose or Helm) and reachable |
| 3 | Realm `pim-test` exists with groups: `admin-eligible`, `admin-active`, `admin-permanent`, `break-glass` |
| 4 | Least-privilege `pim-service` client can grant and revoke via Admin API |
| 5 | Fixture users exist; break-glass can authenticate with admin authority |
| 6 | Automated plumbing test exits **PASS** |
| 7 | Realm bootstrap documented (`scripts/setup.sh` / Compose import). Optional AKS lab: `deploy/infra/aks/bootstrap.sh` |

**Out of scope:** PIM workflow app, timers, UI, notifications.

---

## Phase 1 — Core grant/revoke engine

**Goal:** Time-bound elevation works through the PIM API.

| # | Done when |
| --- | --- |
| 1 | Eligible user can create a request with justification |
| 2 | Unapproved requests expire after **1 hour** |
| 3 | Only `admin-permanent` can approve; others are rejected |
| 4 | Approve adds the user to `admin-active` in Keycloak |
| 5 | After **9 hours** (or early release) user is removed from `admin-active` |
| 6 | App restart still revokes expired grants (startup reconcile) |
| 7 | PIM runs as a Deployment (lab: `keycloak-pim` namespace) with a built image |

**Out of scope:** Polished UI, Rocket.Chat/Gmail, Harbor, Cilium, audit SPI.

**One-liner:** Eligible → permanent approves → active 9h → reliably revoked — via the app.

---

## Phase 2 — Audit & visibility

**Goal:** Every activation is traceable; audit survives Keycloak console tampering as much as practical.

| # | Done when |
| --- | --- |
| 1 | Full lifecycle logged: request → approve → grant → expire/release |
| 2 | Approvals by `admin-permanent` are recorded with actor + justification |
| 3 | Break-glass usage is flagged/alerted |
| 4 | Basic query/view of audit history exists (API or simple page) |
| 5 | (Stretch) Keycloak Event Listener SPI writes to an append-only external store |

**Out of scope:** Full SIEM integration, long-term compliance reporting UI.

---

## Phase 3 — Self-service activation UX

**Goal:** Users and approvers use the PIM web app end-to-end (no Admin console for day-to-day).

| # | Done when |
| --- | --- |
| 1 | OIDC login to PIM app via Keycloak |
| 2 | Eligible user can request, see pending/active, and early-release from UI |
| 3 | Permanent admin can see queue and approve from UI |
| 4 | Countdown for request TTL and active grant is visible |
| 5 | Non-eligible / non-permanent users see appropriate empty/denied states |

**Out of scope:** Mobile app, complex theming, multi-realm UI.

---

## Phase 4 — Notifications & approval UX polish

**Goal:** Approvers are notified without watching the queue constantly.

**Locked order:** (1) in-app approval queue in Phase 3, (2) email (Gmail SMTP lab OK), (3) Rocket.Chat optional later. Webhooks (Teams/Slack) allowed as an alternate to email.

| # | Done when |
| --- | --- |
| 1 | Phase 3 in-app queue is the primary approve path (no external notifier required for MVP) |
| 2 | Pending request can notify `admin-permanent` via email (or Teams/Slack webhook) |
| 3 | Approve action works from notify link or deep-link into app (authenticated) |
| 4 | Requester is notified on approve / expire / revoke |
| 5 | Noise controls exist (no spam on every reconcile tick) |

**Out of scope for now:** Rocket.Chat, full on-call calendars, PagerDuty, multi-tier escalation (unless added later).

---

## Phase 5 — Step-up security

**Goal:** Harden the moment of elevation.

**Lab status:** **SKIPPED** — work Keycloak is behind PKI; revisit only if policy needs MFA/`acr` at approve time or dual control. See [phase5-FINDINGS.md](phase5-FINDINGS.md).

| # | Done when |
| --- | --- |
| 1 | Sensitive activation can require fresh MFA / re-auth (`acr` / `max_age`) |
| 2 | Per-role (or per-group) config for durations and approval rules |
| 3 | Optional dual control for crown-jewel roles (e.g. two `admin-permanent` approvals) |
| 4 | Self-approval remains **off** unless explicitly enabled and flagged |

---

## Phase 6 — Chaos revoke + elevation ledger

**Goal:** Prove the revoke path survives failure, and give operators a clear elevation ledger (replaces a separate “dashboard”).

**In one sentence:** Chaos-test revoke, and keep an elevation ledger: user, request time, justification, approver, approval time, and approver notes/guidance.

| # | Done when |
| --- | --- |
| 1 | **Chaos / failure-tested revoke:** Keycloak unavailable, PIM restart, and DB restore still leave no orphaned `admin-active` |
| 2 | **Elevation ledger UI:** User · Time of request · Justification · Approver · Time of approval · Notes/guidance · Status |
| 3 | **Justification required** on every request (min length enforced) |
| 4 | **Approver notes required** on every approve (written feedback / guidance) |
| 5 | Short runbook notes for break-glass and “revoke all active” kill-switch (can be markdown) |

**Out of scope for Phase 6:** multi-cloud image promotion (→ Phase 8), scheduled access-review campaigns (→ Phase 7), separate sign-in/out dashboard (ledger covers elevation accountability).

**Why it exists:** Phases 1–4 prove the happy path. Phase 6 proves “still safe when things break” and “ops can see who asked, who approved, and why.”

---

## Phase 7 (Optional) — Advanced parity

**Goal:** Closer to full Azure PIM (workflow extras, not cloud replication).

| # | Done when |
| --- | --- |
| 1 | Scheduled access reviews / recertification |
| 2 | Richer escalation / on-call-aware routing |
| 3 | Anomaly alerts on unusual activation patterns |
| 4 | Multi-realm support with per-realm config |
| 5 | App-cluster demo (e.g. Nautobot) consuming elevated groups/roles |

---

## Phase 8 (Last) — Multi-cloud image promotion

**Goal:** Treat a known-good PIM image as portable across clouds (Azure lab now → AWS later).

**In one sentence:** Promote a stable, versioned PIM image from the lab registry (ACR) into another cloud’s registry (e.g. AWS ECR) and run the same app there.

| # | Done when |
| --- | --- |
| 1 | Lab image promotion path documented (build → tag → ACR; only promote known-good tags) |
| 2 | Replicate/promote that image into a second cloud registry (AWS later) |
| 3 | Same chart/config pattern deploys PIM successfully in the second environment (smoke: healthz + one elevate/revoke) |

**Out of scope until later:** full AWS Keycloak rebuild (can reuse existing IdP or stand up later); Harbor optional.

---

## Quick map

| Phase | One-line DofD |
| --- | --- |
| **0** | Cluster + Keycloak + plumbing grant/revoke PASS |
| **1** | API elevation to `admin-active` for 9h, revoke reliable |
| **2** | Every grant is auditable |
| **3** | Users/approvers do it in the web UI |
| **4** | Approvers get Slack/Teams webhook (email / Rocket.Chat optional later) |
| **5** | MFA / dual-control on elevation (**skipped** in this lab — PKI edge) |
| **6** | Chaos-tested revoke + elevation ledger (user / times / justification / approver / notes) |
| **7** | Optional Azure PIM extras (access reviews, escalation, multi-realm) |
| **8** | Last: promote stable image to another cloud (Azure ACR → AWS later) |

**Minimum Viable PIM for real use:** Phases **1 + 2 + 3** (engine + audit + in-app UI queue).  
**Notify (4)** = Slack/Teams webhook lab path done.  
**5** skipped here (PKI).  
**6** = chaos revoke + elevation ledger before treating the lab as ops-serious.  
**8** = multi-cloud image promotion (last).
