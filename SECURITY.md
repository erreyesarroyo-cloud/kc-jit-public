# Security Policy

`keycloak-jit-access` is a privileged component: it holds an Admin-API service
account for your Keycloak realm and can add/remove users from admin groups. Treat
it with the same care as any other privileged-access-management (PAM) system.

## Reporting a vulnerability

Please **do not** open a public issue for security problems. Report them privately
via GitHub Security Advisories ("Report a vulnerability" on the repository's
**Security** tab). We aim to acknowledge reports within 5 business days.

## Least-privilege service account

The service authenticates to Keycloak as a confidential client
(`PIM_CLIENT_ID` / `PIM_CLIENT_SECRET`). Grant it **only** the realm-management
roles it actually uses:

- `view-users` — resolve usernames to user IDs and read group membership.
- `manage-users` — add/remove users from the managed groups.
- `query-groups` / `view-realm` — resolve group names to group IDs.

Do **not** grant `realm-admin` or `manage-realm`. The account never needs to create
clients, edit realm settings, or manage other clients. Scope it to the single realm
named by `KC_REALM`.

## Trust boundaries

- **Admin credentials at rest:** `PIM_CLIENT_SECRET` (and `OIDC_CLIENT_SECRET`) must
  be injected as secrets, never baked into images or committed. See `.gitignore` —
  `.env` files are excluded by design.
- **Session secret:** `SESSION_SECRET` defaults to a lab placeholder and **must** be
  overridden with a strong random value in any real deployment.
- **Transport:** put Keycloak and this service behind TLS. `KC_BASE_URL` may be a
  cluster-internal plaintext address; `KC_PUBLIC_URL` (browser-facing) should be HTTPS.
- **Audit database:** the SQLite ledger (`DB_PATH`) is the primary accountability
  control. Store it on durable, access-controlled storage and back it up.

## Threat model highlights

| Risk | Mitigation |
|------|------------|
| Standing admin access | Eliminated — all admin group membership is time-boxed and auto-revoked. |
| Orphaned access after crash | Startup reconciliation revokes anything past expiry. |
| Unapproved elevation | Only `admin-permanent` members can approve; approver notes are mandatory. |
| Self-approval by a permanent admin | **Allowed by design but flagged and audited.** If you need strict segregation of duties, ensure requesters are not also in `admin-permanent`. |
| Break-glass abuse | Break-glass sessions/actions emit `ALERT` log lines and are marked in the audit ledger. |
| Header-based auth in production | `ALLOW_HEADER_AUTH` defaults to `true` for lab use. **Set it to `false`** and rely on OIDC in production. |

## Hardening checklist for production

- [ ] Scope the service account to `view-users`, `manage-users`, `query-groups` only.
- [ ] Set a strong `SESSION_SECRET`.
- [ ] Set `ALLOW_HEADER_AUTH=false`; require OIDC login.
- [ ] Serve everything over TLS; set `KC_PUBLIC_URL` to the HTTPS URL.
- [ ] Ensure requesters and approvers are disjoint if you require segregation of duties.
- [ ] Ship audit-ledger and `ALERT` log lines to your SIEM.
- [ ] Persist and back up `DB_PATH`.
