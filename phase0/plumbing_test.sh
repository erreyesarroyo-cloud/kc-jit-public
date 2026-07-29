#!/usr/bin/env bash
#
# Phase 0 exit-criteria proof.
# Using the LEAST-PRIVILEGE PIM service account (client credentials):
#   1. obtain a token
#   2. GRANT the test role to admin1
#   3. verify it is present
#   4. REVOKE the test role
#   5. verify it is gone
# Also verifies the break-glass account can authenticate.
# Prints PASS/FAIL.
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ENV_FILE="${SCRIPT_DIR}/.env"
[[ -f "$ENV_FILE" ]] || { echo "ERROR: .env not found. Run setup.sh first." >&2; exit 1; }
# shellcheck disable=SC1090
set -a; source "$ENV_FILE"; set +a

[[ "${KC_REALM}" != "master" && -n "${KC_REALM}" ]] || { echo "ERROR: bad KC_REALM." >&2; exit 1; }
[[ -n "${PIM_CLIENT_SECRET}" ]] || { echo "ERROR: PIM_CLIENT_SECRET empty. Run setup.sh." >&2; exit 1; }

need() { command -v "$1" >/dev/null 2>&1 || { echo "ERROR: '$1' required." >&2; exit 1; }; }
need curl; need jq

API="${KC_BASE_URL}/admin/realms/${KC_REALM}"
FAIL=0
ok()   { echo "  PASS: $1"; }
bad()  { echo "  FAIL: $1"; FAIL=1; }

echo ">> Target: ${KC_BASE_URL} realm=${KC_REALM}"

# ---- 1. PIM service-account token (client credentials) ----------------------
echo ">> [1] Obtaining PIM service-account token..."
PIM_TOKEN=$(curl -sf \
  -d "client_id=${PIM_CLIENT_ID}" \
  -d "client_secret=${PIM_CLIENT_SECRET}" \
  -d "grant_type=client_credentials" \
  "${KC_BASE_URL}/realms/${KC_REALM}/protocol/openid-connect/token" \
  | jq -r .access_token)
if [[ -n "$PIM_TOKEN" && "$PIM_TOKEN" != "null" ]]; then ok "got service-account token"; else bad "no token"; exit 1; fi
PH=(-H "Authorization: Bearer ${PIM_TOKEN}" -H "Content-Type: application/json")

# ---- lookups ----------------------------------------------------------------
ADMIN_UID=$(curl -sf "${PH[@]}" "${API}/users?username=${ADMIN_USERNAME}&exact=true" | jq -r '.[0].id // empty')
[[ -n "$ADMIN_UID" ]] && ok "found ${ADMIN_USERNAME} (${ADMIN_UID})" || { bad "cannot find ${ADMIN_USERNAME}"; exit 1; }
ROLE_JSON=$(curl -sf "${PH[@]}" "${API}/roles/${TEST_ROLE}")
ROLE_ID=$(echo "$ROLE_JSON" | jq -r '.id // empty')
[[ -n "$ROLE_ID" ]] && ok "found role ${TEST_ROLE}" || { bad "cannot find role ${TEST_ROLE}"; exit 1; }

has_role() {
  curl -sf "${PH[@]}" "${API}/users/${ADMIN_UID}/role-mappings/realm" \
    | jq -e --arg r "$TEST_ROLE" 'any(.[]; .name == $r)' >/dev/null 2>&1
}

# ---- 2. GRANT ---------------------------------------------------------------
echo ">> [2] Granting '${TEST_ROLE}' to ${ADMIN_USERNAME}..."
curl -sf "${PH[@]}" -X POST "${API}/users/${ADMIN_UID}/role-mappings/realm" -d "[${ROLE_JSON}]" >/dev/null

# ---- 3. verify present ------------------------------------------------------
if has_role; then ok "role present after grant"; else bad "role NOT present after grant"; fi

# ---- 4. REVOKE --------------------------------------------------------------
echo ">> [4] Revoking '${TEST_ROLE}' from ${ADMIN_USERNAME}..."
curl -sf "${PH[@]}" -X DELETE "${API}/users/${ADMIN_UID}/role-mappings/realm" -d "[${ROLE_JSON}]" >/dev/null

# ---- 5. verify gone ---------------------------------------------------------
if has_role; then bad "role STILL present after revoke"; else ok "role gone after revoke"; fi

# ---- break-glass login check ------------------------------------------------
echo ">> [6] Verifying break-glass account can authenticate..."
BG_TOKEN=$(curl -sf \
  -d "client_id=admin-cli" \
  -d "username=${BREAKGLASS_USERNAME}" \
  -d "password=${BREAKGLASS_PASSWORD}" \
  -d "grant_type=password" \
  "${KC_BASE_URL}/realms/${KC_REALM}/protocol/openid-connect/token" \
  | jq -r .access_token 2>/dev/null || true)
if [[ -n "$BG_TOKEN" && "$BG_TOKEN" != "null" ]]; then ok "break-glass can authenticate"; else bad "break-glass auth failed (may need admin-cli direct grants enabled)"; fi

# ---- [7] break-glass has real admin power -----------------------------------
# Not just token — must actually be able to call /admin/... endpoints.
echo ">> [7] Verifying break-glass has admin authority (hits /admin/realms/${KC_REALM}/users)..."
if [[ -n "$BG_TOKEN" && "$BG_TOKEN" != "null" ]]; then
  BG_STATUS=$(curl -s -o /dev/null -w '%{http_code}' \
    -H "Authorization: Bearer ${BG_TOKEN}" \
    "${API}/users?max=1")
  if [[ "$BG_STATUS" == "200" ]]; then
    ok "break-glass can call admin API (HTTP 200)"
  else
    bad "break-glass admin call returned HTTP ${BG_STATUS} (expected 200)"
  fi
else
  bad "break-glass admin authority skipped (no token)"
fi

# ---- [8] Active/Eligible group model — effective role mappings --------------
# superadmin1 (in admin-active) must have TEST_ROLE via group inheritance.
# admin1 & admin2 (in admin-eligible) must have NO standing custom roles.
# This proves the Active/Eligible model is wired the way PHASES.md expects.
echo ">> [8] Verifying Active/Eligible group model..."

# helper: composite of effective realm role names for a user
effective_realm_role_names() {
  local uid="$1"
  # /role-mappings returns { realmMappings, clientMappings }
  # /role-mappings/realm/composite returns effective realm roles incl. group inheritance
  curl -sf "${PH[@]}" "${API}/users/${uid}/role-mappings/realm/composite" \
    | jq -r '[.[].name] // [] | .[]' 2>/dev/null | sort -u
}

# superadmin1 -> must include TEST_ROLE (inherited from admin-active group)
LEAD_UID=$(curl -sf "${PH[@]}" "${API}/users?username=${LEAD_USERNAME}&exact=true" | jq -r '.[0].id // empty')
if [[ -n "$LEAD_UID" ]]; then
  if effective_realm_role_names "$LEAD_UID" | grep -qx "${TEST_ROLE}"; then
    ok "${LEAD_USERNAME} has '${TEST_ROLE}' via admin-active group"
  else
    bad "${LEAD_USERNAME} MISSING '${TEST_ROLE}' (expected via admin-active group)"
  fi
else
  bad "cannot look up ${LEAD_USERNAME}"
fi

# admin1 & admin2 -> must NOT have TEST_ROLE standing (they're only eligible)
for u in "${ADMIN_USERNAME}" "${ADMIN2_USERNAME}"; do
  UID_=$(curl -sf "${PH[@]}" "${API}/users?username=${u}&exact=true" | jq -r '.[0].id // empty')
  if [[ -z "$UID_" ]]; then bad "cannot look up ${u}"; continue; fi
  if effective_realm_role_names "$UID_" | grep -qx "${TEST_ROLE}"; then
    bad "${u} unexpectedly has standing '${TEST_ROLE}' (should be eligible-only)"
  else
    ok "${u} has no standing '${TEST_ROLE}' (eligible-only, as designed)"
  fi
done

echo ""
if [[ "$FAIL" -eq 0 ]]; then
  echo "======================================"
  echo " PHASE 0 EXIT CRITERIA: PASS"
  echo "======================================"
  exit 0
else
  echo "======================================"
  echo " PHASE 0 EXIT CRITERIA: FAIL"
  echo "======================================"
  exit 1
fi
