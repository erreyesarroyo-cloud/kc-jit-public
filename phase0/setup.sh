#!/usr/bin/env bash
#
# Phase 0 setup — idempotent.
# Creates in the TARGET REALM ONLY:
#   - test role (TEST_ROLE)
#   - users: superadmin1 (lead), admin1 (approved admin), breakglass1 (permanent)
#   - PIM service-account client (least privilege: view-users + manage-users)
#
# Guardrail: aborts if KC_REALM is empty or looks like "master".
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ENV_FILE="${SCRIPT_DIR}/.env"

if [[ ! -f "$ENV_FILE" ]]; then
  echo "ERROR: ${ENV_FILE} not found. Copy .env.example to .env and fill it in." >&2
  exit 1
fi
# shellcheck disable=SC1090
set -a; source "$ENV_FILE"; set +a

# ---- Guardrails --------------------------------------------------------------
if [[ -z "${KC_REALM:-}" ]]; then
  echo "ERROR: KC_REALM is empty. Refusing to run." >&2; exit 1
fi
if [[ "${KC_REALM}" == "master" ]]; then
  echo "ERROR: KC_REALM is 'master'. Refusing to run against master realm." >&2; exit 1
fi
echo ">> Target: ${KC_BASE_URL}  realm=${KC_REALM}"

need() { command -v "$1" >/dev/null 2>&1 || { echo "ERROR: '$1' is required." >&2; exit 1; }; }
need curl; need jq

# ---- Admin token -------------------------------------------------------------
echo ">> Obtaining admin token..."
ADMIN_TOKEN=$(curl -sf \
  -d "client_id=${KC_ADMIN_CLIENT_ID}" \
  -d "username=${KC_ADMIN_USERNAME}" \
  -d "password=${KC_ADMIN_PASSWORD}" \
  -d "grant_type=password" \
  "${KC_BASE_URL}/realms/${KC_ADMIN_REALM}/protocol/openid-connect/token" \
  | jq -r .access_token)

if [[ -z "$ADMIN_TOKEN" || "$ADMIN_TOKEN" == "null" ]]; then
  echo "ERROR: failed to obtain admin token. Check credentials." >&2; exit 1
fi
AH=(-H "Authorization: Bearer ${ADMIN_TOKEN}" -H "Content-Type: application/json")
API="${KC_BASE_URL}/admin/realms/${KC_REALM}"

# ---- Verify target realm exists ---------------------------------------------
echo ">> Verifying realm '${KC_REALM}' exists..."
REALM_NAME=$(curl -sf "${AH[@]}" "${API}" | jq -r .realm || true)
if [[ "$REALM_NAME" != "$KC_REALM" ]]; then
  echo "ERROR: realm '${KC_REALM}' not found or not accessible." >&2; exit 1
fi

# ---- Detect Keycloak version -------------------------------------------------
KC_VERSION=$(curl -sf "${KC_BASE_URL}/admin/serverinfo" -H "Authorization: Bearer ${ADMIN_TOKEN}" \
  | jq -r '.systemInfo.version' 2>/dev/null || echo "unknown")
echo ">> Keycloak version: ${KC_VERSION}"

# ---- Helper: create realm role if missing -----------------------------------
ensure_role() {
  local role="$1"
  if curl -sf "${AH[@]}" "${API}/roles/${role}" >/dev/null 2>&1; then
    echo "   role '${role}' exists"
  else
    curl -sf "${AH[@]}" -X POST "${API}/roles" -d "{\"name\":\"${role}\"}"
    echo "   role '${role}' created"
  fi
}

# ---- Helper: create user if missing, set password ---------------------------
# NOTE: Keycloak 26+ enforces a declarative user profile. If email/firstName/
# lastName are missing, direct-grant login fails with
# "Account is not fully set up". We populate placeholder values so fixtures
# can authenticate. Override real values via the admin console if needed.
ensure_user() {
  local username="$1" password="$2"
  local uid
  local body="{
    \"username\":\"${username}\",
    \"enabled\":true,
    \"emailVerified\":true,
    \"email\":\"${username}@example.local\",
    \"firstName\":\"${username}\",
    \"lastName\":\"fixture\",
    \"requiredActions\":[]
  }"
  uid=$(curl -sf "${AH[@]}" "${API}/users?username=${username}&exact=true" | jq -r '.[0].id // empty')
  if [[ -z "$uid" ]]; then
    curl -sf "${AH[@]}" -X POST "${API}/users" -d "${body}"
    uid=$(curl -sf "${AH[@]}" "${API}/users?username=${username}&exact=true" | jq -r '.[0].id')
    echo "   user '${username}' created (${uid})"
  else
    # Idempotent: make sure required profile fields are populated even if the
    # user was created before this script was fixed.
    curl -sf "${AH[@]}" -X PUT "${API}/users/${uid}" -d "${body}"
    echo "   user '${username}' exists (${uid}) — profile fields ensured"
  fi
  curl -sf "${AH[@]}" -X PUT "${API}/users/${uid}/reset-password" \
    -d "{\"type\":\"password\",\"value\":\"${password}\",\"temporary\":false}"
  echo "   user '${username}' password set"
}

echo ">> Ensuring test role..."
ensure_role "${TEST_ROLE}"

echo ">> Ensuring users..."
ensure_user "${LEAD_USERNAME}"       "${LEAD_PASSWORD}"
ensure_user "${ADMIN_USERNAME}"      "${ADMIN_PASSWORD}"
ensure_user "${ADMIN2_USERNAME}"     "${ADMIN2_PASSWORD}"
ensure_user "${BREAKGLASS_USERNAME}" "${BREAKGLASS_PASSWORD}"

# ---- PIM service-account client (least privilege) ----------------------------
echo ">> Ensuring PIM service-account client '${PIM_CLIENT_ID}'..."
CID=$(curl -sf "${AH[@]}" "${API}/clients?clientId=${PIM_CLIENT_ID}" | jq -r '.[0].id // empty')
if [[ -z "$CID" ]]; then
  curl -sf "${AH[@]}" -X POST "${API}/clients" -d "{
    \"clientId\":\"${PIM_CLIENT_ID}\",
    \"enabled\":true,
    \"publicClient\":false,
    \"serviceAccountsEnabled\":true,
    \"standardFlowEnabled\":false,
    \"directAccessGrantsEnabled\":false
  }"
  CID=$(curl -sf "${AH[@]}" "${API}/clients?clientId=${PIM_CLIENT_ID}" | jq -r '.[0].id')
  echo "   client created (${CID})"
else
  echo "   client exists (${CID})"
fi

# Fetch/generate client secret
SECRET=$(curl -sf "${AH[@]}" "${API}/clients/${CID}/client-secret" | jq -r '.value // empty')
if [[ -z "$SECRET" || "$SECRET" == "null" ]]; then
  SECRET=$(curl -sf "${AH[@]}" -X POST "${API}/clients/${CID}/client-secret" | jq -r '.value')
fi
echo "   client secret retrieved"

# ---- Grant least-privilege realm-management roles to the service account -----
# Service account user for the client:
SA_UID=$(curl -sf "${AH[@]}" "${API}/clients/${CID}/service-account-user" | jq -r '.id')
RM_CID=$(curl -sf "${AH[@]}" "${API}/clients?clientId=realm-management" | jq -r '.[0].id')

grant_client_role() {
  local role="$1"
  local role_json
  role_json=$(curl -sf "${AH[@]}" "${API}/clients/${RM_CID}/roles/${role}")
  # Idempotent: only assign if not already present.
  if curl -sf "${AH[@]}" \
      "${API}/users/${SA_UID}/role-mappings/clients/${RM_CID}" \
      | jq -e --arg r "$role" 'any(.[]; .name == $r)' >/dev/null 2>&1; then
    echo "   role '${role}' already on service account"
    return 0
  fi
  curl -sf "${AH[@]}" -X POST \
    "${API}/users/${SA_UID}/role-mappings/clients/${RM_CID}" \
    -d "[${role_json}]"
  echo "   granted '${role}' to service account"
}
# Least-privilege set:
#   view-users, manage-users  -> list users, assign/remove role mappings
#   view-realm                -> read realm-level roles (GET /roles/{name})
#   query-users, query-clients-> support the queries above without full realm access
# Explicitly NOT granted: manage-realm, manage-clients, realm-admin.
echo ">> Granting least-privilege roles to service account..."
grant_client_role "view-users"
grant_client_role "manage-users"
grant_client_role "view-realm"
grant_client_role "query-users"
grant_client_role "query-clients"

# ---- Groups (Active / Eligible / Break-glass model) --------------------------
# Structure (see PHASES.md — Locked Design Decisions):
#   break-glass     -> permanent full-realm admin (realm-management/realm-admin)
#   admin-active    -> holds TEST_ROLE permanently (Azure PIM "Active" concept)
#   admin-eligible  -> no roles; marker for users PIM can activate TEST_ROLE for
# Membership drives entitlement — the PIM app reads these groups as source of truth.

ensure_group() {
  local name="$1"
  local gid
  gid=$(curl -sf "${AH[@]}" "${API}/groups?search=${name}&exact=true" | jq -r '.[0].id // empty')
  if [[ -z "$gid" ]]; then
    curl -sf "${AH[@]}" -X POST "${API}/groups" -d "{\"name\":\"${name}\"}" >&2
    gid=$(curl -sf "${AH[@]}" "${API}/groups?search=${name}&exact=true" | jq -r '.[0].id')
    echo "   group '${name}' created (${gid})" >&2
  else
    echo "   group '${name}' exists (${gid})" >&2
  fi
  # Only the gid goes to stdout (captured by the caller).
  printf '%s' "$gid"
}

# Grant a realm-management client role to a group (idempotent).
grant_group_client_role() {
  local gid="$1" role="$2"
  local role_json
  role_json=$(curl -sf "${AH[@]}" "${API}/clients/${RM_CID}/roles/${role}")
  if curl -sf "${AH[@]}" \
      "${API}/groups/${gid}/role-mappings/clients/${RM_CID}" \
      | jq -e --arg r "$role" 'any(.[]; .name == $r)' >/dev/null 2>&1; then
    echo "   group already has client-role '${role}'"
    return 0
  fi
  curl -sf "${AH[@]}" -X POST \
    "${API}/groups/${gid}/role-mappings/clients/${RM_CID}" \
    -d "[${role_json}]"
  echo "   granted client-role '${role}' to group"
}

# Grant a realm role to a group (idempotent).
grant_group_realm_role() {
  local gid="$1" role="$2"
  local role_json
  role_json=$(curl -sf "${AH[@]}" "${API}/roles/${role}")
  if curl -sf "${AH[@]}" "${API}/groups/${gid}/role-mappings/realm" \
      | jq -e --arg r "$role" 'any(.[]; .name == $r)' >/dev/null 2>&1; then
    echo "   group already has realm-role '${role}'"
    return 0
  fi
  curl -sf "${AH[@]}" -X POST "${API}/groups/${gid}/role-mappings/realm" -d "[${role_json}]"
  echo "   granted realm-role '${role}' to group"
}

# Add user to group (idempotent — PUT is naturally idempotent here).
add_user_to_group() {
  local username="$1" gid="$2"
  local uid
  uid=$(curl -sf "${AH[@]}" "${API}/users?username=${username}&exact=true" | jq -r '.[0].id')
  curl -sf "${AH[@]}" -X PUT "${API}/users/${uid}/groups/${gid}"
  echo "   user '${username}' in group"
}

echo ">> Ensuring group '${GROUP_BREAKGLASS}' (permanent realm-admin)..."
BG_GID=$(ensure_group "${GROUP_BREAKGLASS}")
grant_group_client_role "${BG_GID}" "realm-admin"
add_user_to_group "${BREAKGLASS_USERNAME}" "${BG_GID}"

echo ">> Ensuring group '${GROUP_ADMIN_ACTIVE}' (standing ${TEST_ROLE})..."
AA_GID=$(ensure_group "${GROUP_ADMIN_ACTIVE}")
grant_group_realm_role "${AA_GID}" "${TEST_ROLE}"
add_user_to_group "${LEAD_USERNAME}" "${AA_GID}"

echo ">> Ensuring group '${GROUP_ADMIN_ELIGIBLE}' (eligible, no standing roles)..."
AE_GID=$(ensure_group "${GROUP_ADMIN_ELIGIBLE}")
add_user_to_group "${ADMIN_USERNAME}"  "${AE_GID}"
add_user_to_group "${ADMIN2_USERNAME}" "${AE_GID}"

# ---- Persist the client secret back into .env -------------------------------
if grep -q '^PIM_CLIENT_SECRET=' "$ENV_FILE"; then
  sed -i "s|^PIM_CLIENT_SECRET=.*|PIM_CLIENT_SECRET=\"${SECRET}\"|" "$ENV_FILE"
else
  echo "PIM_CLIENT_SECRET=\"${SECRET}\"" >> "$ENV_FILE"
fi
echo ">> PIM_CLIENT_SECRET written to .env"

echo ""
echo "SETUP COMPLETE."
echo "  realm         = ${KC_REALM}"
echo "  kc version    = ${KC_VERSION}"
echo "  test role     = ${TEST_ROLE}"
echo "  users         = ${LEAD_USERNAME}, ${ADMIN_USERNAME}, ${ADMIN2_USERNAME}, ${BREAKGLASS_USERNAME}"
echo "  groups        = ${GROUP_BREAKGLASS} (realm-admin), ${GROUP_ADMIN_ACTIVE} (${TEST_ROLE}), ${GROUP_ADMIN_ELIGIBLE} (no roles)"
echo "  pim client    = ${PIM_CLIENT_ID} (view-users, manage-users, view-realm, query-users, query-clients)"
echo "Next: run ./plumbing_test.sh"
