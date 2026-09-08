# Phase 0 — Foundations & Plumbing Proof

Status: **COMPLETE** — all exit criteria verified by `plumbing_test.sh`.

Goal: prove we can **grant and revoke a role** in the target realm using a
**least-privilege service account**, stand up the Active/Eligible/Break-glass
group model, and validate the test fixtures — before building any PIM logic.

## Decisions (recorded)

| Item | Decision |
|------|----------|
| Target | `https://kc.staging.example.com`, realm **`pim-test`** (ALL calls scoped here) |
| Active persona (standing role) | `superadmin1` — in group `admin-active`, holds `pim-test-role` permanently |
| Eligible personas (PIM-activatable) | `admin1`, `admin2` — in group `admin-eligible`, hold nothing standing |
| Break-glass (permanent full-realm admin) | `breakglass1` — in group `break-glass`, holds `realm-admin` |
| First managed role | `pim-test-role` (safe custom role; avoids touching real admin rights during plumbing) |
| Phase 0 stack | Bash + curl (proof only) |
| Phase 1+ engine stack | Go (durable single binary, reliable scheduler + reconciliation) |
| Phase 1 durable store | SQLite (single file, zero setup) |
| Service-account placement | **Same realm** (`pim-test`). NOTE: same-realm = less separation of duty; acceptable on staging, **revisit before prod** (run from `master`). |
| KC version | Auto-detected by `setup.sh` (verified against 26.3.3) |
| Fixture passwords | `123456789` for all fixtures — staging plumbing only, **never for prod** |

## Files

- `.env.example` — config template (copy to `.env`, fill in, never commit)
- `setup.sh` — idempotent: creates test role, users, groups, and least-privilege PIM client
- `plumbing_test.sh` — the exit-criteria proof (grant/revoke + group model + break-glass)

## Fixtures created by `setup.sh`

### Users

| User | Password | Group | Standing roles (effective) |
|------|----------|-------|----------------------------|
| `breakglass1` | `123456789` | `break-glass` | `realm-management/realm-admin` (full realm admin) |
| `superadmin1` | `123456789` | `admin-active` | `pim-test-role` (inherited from group) |
| `admin1` | `123456789` | `admin-eligible` | *(none — PIM activates on request)* |
| `admin2` | `123456789` | `admin-eligible` | *(none — PIM activates on request)* |

All users are populated with `email`, `firstName`, `lastName`, `emailVerified: true`,
`requiredActions: []`. Keycloak 26+ enforces a declarative user profile — without
these fields, direct-grant login fails with `"Account is not fully set up"`.

### Groups (Active / Eligible / Break-glass model)

| Group | Realm roles | Client roles | Members |
|-------|-------------|--------------|---------|
| `break-glass` | — | `realm-management/realm-admin` | `breakglass1` |
| `admin-active` | `pim-test-role` | — | `superadmin1` |
| `admin-eligible` | — | — | `admin1`, `admin2` |

Groups are the **source of truth** for entitlement (PHASES.md:179). The PIM app
will query group membership to determine who is active, eligible, or break-glass.

## Least-privilege service account

The PIM service-account client (`pim-service`) is granted ONLY these
`realm-management` client roles:

- `view-users` — list users
- `manage-users` — assign/remove role mappings (the core PIM operation)
- `view-realm` — read realm-level roles (`GET /roles/{name}`)
- `query-users`, `query-clients` — support the reads above without broader access

It explicitly CANNOT `manage-realm`, `manage-clients`, or `realm-admin` — it
cannot reconfigure the realm, only assign/remove role mappings.

**Known limitation (design flag for Phase 1):** Keycloak enforces "you can't
grant a role you don't have." `pim-service` can therefore grant `pim-test-role`
but NOT `realm-admin` or other admin composite roles. Phase 1 must decide which
specific admin roles PIM will manage and grant `pim-service` those (and only
those) — do not grant it `realm-admin`.

## How to run

```bash
cd phase0
cp .env.example .env
# edit .env: set KC_ADMIN_USERNAME / KC_ADMIN_PASSWORD (temp admin)
./setup.sh            # idempotent; creates fixtures + PIM client, writes PIM_CLIENT_SECRET to .env
./plumbing_test.sh    # proves exit criteria; prints PASS/FAIL
```

## Exit criteria

- [x] Service account obtains a token (client credentials)
- [x] Service account GRANTS `pim-test-role` to `admin1`
- [x] Role verified present after grant
- [x] Service account REVOKES the role
- [x] Role verified gone after revoke
- [x] Break-glass account can authenticate
- [x] Break-glass account has real admin authority (HTTP 200 on `/admin/realms/pim-test/users`)
- [x] Active user (`superadmin1`) inherits `pim-test-role` via `admin-active` group
- [x] Eligible users (`admin1`, `admin2`) hold NO standing custom roles
- [x] All actions scoped to `pim-test` (guardrail: scripts abort on wrong realm)

Result: `PHASE 0 EXIT CRITERIA: PASS` from `plumbing_test.sh` (10/10 checks).

## Break-glass procedure (staging)

`breakglass1` is a permanent admin that does NOT go through PIM. It is the
escape hatch if the PIM engine misbehaves. For production: strong/hardware MFA,
password in a vault, and alert on every login. On staging it exists to validate
the pattern.
