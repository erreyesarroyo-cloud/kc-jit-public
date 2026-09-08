# Phase 5 — Findings (step-up security)

**Date:** 2026-08-15  
**Status:** **SKIPPED / deferred**

## Decision

Phase 5 (MFA / `acr` / `max_age` step-up, dual control, self-approval hardening) is **out of scope for this lab**.

**Rationale:** Work Keycloak sits behind **PKI** (mutual TLS / cert auth at the edge). That already raises the authentication bar for reaching Keycloak; JIT elevation + audit + Slack notify (Phases 1–4) are the PIM controls in use. Extra step-up inside PIM is optional later if threat model changes.

## DofD (not executed)

| # | Criterion | Status |
| --- | --- | --- |
| 1 | Fresh MFA / re-auth (`acr` / `max_age`) | Skipped |
| 2 | Per-role durations / approval rules | Skipped (global TTLs remain from Phase 1) |
| 3 | Optional dual control | Skipped |
| 4 | Self-approval off unless enabled | Skipped (approvers are `admin-permanent` only; eligibles cannot approve) |

## Running image

Lab remains on **`pim:0.3.0`** (Phase 4 Slack webhook). No `0.4.0` Phase 5 deploy.

## Revisit when

- PKI is not present in an environment, or  
- Policy requires MFA at the **moment of approval**, not only at network edge, or  
- Crown-jewel roles need two-person approval.
