# keycloak-jit-access

**Just-in-time, time-bound, approved admin access for Keycloak.**

`keycloak-jit-access` is a small companion service that brings an Azure-PIM-style
workflow to [Keycloak](https://www.keycloak.org/): nobody holds standing admin
rights. Instead, eligible users **request** elevation, an approver **grants** it,
the grant is **time-boxed**, and it is **automatically revoked** when the timer
expires — with a tamper-evident audit ledger of every request, approval, and
revocation.

> **What this is (and isn't).** This is **not** a Keycloak server extension (SPI/JAR
> running inside Keycloak). It is a standalone service that drives Keycloak through
> its [Admin REST API](https://www.keycloak.org/docs-api/latest/rest-api/index.html)
> using a confidential client / service account. It works with an unmodified,
> stock Keycloak.

---

## Why

Standing admin access is the single biggest source of privilege-escalation risk in
an IdP. Keycloak has rich RBAC but no built-in *just-in-time* activation flow.
`keycloak-jit-access` adds that flow on top of groups you already control:

- **No standing admin access** — eligible users must request it every time.
- **Time-bound** — grants expire and auto-revoke regardless of app state.
- **Approval required** — only permanent admins can approve.
- **Reconciled on startup** — restarts revoke anything already past expiry; no orphaned access.
- **Fully audited** — who, when, why, approved-by, and notes for every elevation.

## How it works

```
Eligible user ──request──►  Approval queue
                                  │  approved within REQUEST_TTL (default 1h) or expires
                                  ▼
                          Role/group granted
                                  │  active for ACTIVE_TTL (default 9h), or released early
                                  ▼
                          Auto-revoked + logged
```

### Architecture

```
┌──────────────┐     OIDC login          ┌────────────────────────┐
│   Browser    │ ───────────────────────►│  keycloak-jit-access   │
│ (user/appr.) │ ◄─── self-service UI ────│  (this service, Go)    │
└──────────────┘                          │  - request/approve API │
                                          │  - expiry scheduler     │
                                          │  - SQLite audit ledger  │
                                          └───────────┬────────────┘
                                                      │ Admin REST API
                                                      │ (service account)
                                                      ▼
                                              ┌────────────────┐
                                              │    Keycloak    │
                                              │  groups: add / │
                                              │  remove member │
                                              └────────────────┘
```

### Keycloak groups it manages

| Group             | Purpose                                                |
|-------------------|--------------------------------------------------------|
| `admin-eligible`  | May request elevation                                  |
| `admin-permanent` | May approve requests (never auto-revoked)              |
| `admin-active`    | Currently elevated — membership is temporary           |
| `break-glass`     | Emergency access, always monitored                     |

Group names are configurable (see [Configuration](#configuration)).

## Compatibility

- **Keycloak:** 24+ (tested against the current stable release; uses only stable Admin REST endpoints).
- **Go:** 1.26+ (to build from source).
- **Storage:** SQLite (embedded, no external DB required).

## Quick start (Docker Compose)

The compose stack starts a throwaway Keycloak, imports a demo realm with the four
groups + a service-account client, and runs the service — so you can see the full
flow in a couple of minutes.

```bash
git clone https://github.com/erreyesarroyo-cloud/keycloak-jit-access.git
cd keycloak-jit-access
docker compose up --build
```

Then open:

- **JIT Access UI:** <http://localhost:8081>
- **Keycloak admin console:** <http://localhost:8080> (admin / admin)

Demo users are created by the realm import (see `examples/realm-export.json`).
Log in as an eligible user, request elevation, then log in as an approver to grant it.

| User    | Password   | Role                                   |
|---------|------------|----------------------------------------|
| `alice` | `password` | `admin-eligible` — can request access  |
| `bob`   | `password` | `admin-permanent` — can approve        |

The quickstart runs with OIDC login enabled (`ALLOW_HEADER_AUTH=false`), so use
the web UI to log in — that's the realistic flow.

Tear down with `docker compose down -v`.

## Configuration

All configuration is via environment variables:

| Variable                | Default                          | Description                                        |
|-------------------------|----------------------------------|----------------------------------------------------|
| `LISTEN_ADDR`           | `:8080`                          | Address the service listens on                     |
| `PUBLIC_URL`            | `http://127.0.0.1:8081`          | Browser-facing URL of this service                 |
| `DB_PATH`               | `/data/pim.db`                   | SQLite database path                               |
| `KC_BASE_URL`           | `http://keycloak:8080`           | Server-side Keycloak base URL                      |
| `KC_PUBLIC_URL`         | (empty)                          | Browser-facing Keycloak URL (if different)         |
| `KC_REALM`              | `pim-test`                       | Realm to manage                                    |
| `PIM_CLIENT_ID`         | `pim-service`                    | Confidential client / service account              |
| `PIM_CLIENT_SECRET`     | (required)                       | Service-account secret                             |
| `OIDC_ENABLED`          | `true`                           | Enable OIDC login for the UI                       |
| `OIDC_CLIENT_ID`        | `pim-web`                        | Public/confidential client for user login          |
| `OIDC_CLIENT_SECRET`    | (empty)                          | Secret for the web client                          |
| `ALLOW_HEADER_AUTH`     | `true`                           | Trust `X-Actor-Username` header (**set `false` in production**; use OIDC) |
| `SESSION_SECRET`        | `lab-only-change-me`             | **Change in production**                           |
| `NOTIFY_WEBHOOK_URL`    | (empty)                          | Optional webhook for approver notifications        |
| `GROUP_ADMIN_ELIGIBLE`  | `admin-eligible`                 | Eligible group name                                |
| `GROUP_ADMIN_ACTIVE`    | `admin-active`                   | Active (elevated) group name                       |
| `GROUP_ADMIN_PERMANENT` | `admin-permanent`                | Permanent-admin / approver group name              |
| `GROUP_BREAKGLASS`      | `break-glass`                    | Break-glass group name                             |
| `REQUEST_TTL`           | `1h`                             | Time an unapproved request stays valid             |
| `ACTIVE_TTL`            | `9h`                             | How long a grant lasts before auto-revoke          |
| `POLL_INTERVAL`         | `30s`                            | Expiry-scheduler tick interval                     |

See [`examples/keycloak-lab.env.example`](examples/keycloak-lab.env.example) for a full sample.

## Deployment

- **Helm:** charts for Keycloak and the service live in [`deploy/helm/`](deploy/helm/).
- **Infra (reference):** example AKS Terraform lives in [`deploy/infra/`](deploy/infra/).

## Security

`keycloak-jit-access` holds an Admin-API service account, so read
[SECURITY.md](SECURITY.md) before deploying — it documents the least-privilege
service-account roles, the threat model, and the (deliberate, flagged) self-approval
option.

## Documentation

- [Architecture & design decisions](docs/journey/PHASES.md)
- [Build journey / findings](docs/journey/) — the phased lab notes this project grew from.

## Contributing

Contributions welcome — see [CONTRIBUTING.md](CONTRIBUTING.md) and
[CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md).

## License

[Apache License 2.0](LICENSE).
