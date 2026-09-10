#!/usr/bin/env bash
set -euo pipefail

WORK=/tmp/kc-pim-phase0
SRC_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

rm -rf "$WORK"
mkdir -p "$WORK"
cp "$SRC_DIR/setup.sh" "$SRC_DIR/plumbing_test.sh" "$WORK/"
sed -i 's/\r$//' "$WORK/setup.sh" "$WORK/plumbing_test.sh"

# Prefer KC_LAB_BASE_URL (any Keycloak URL), else discover a keycloak Service LB IP.
if [[ -z "${KC_LAB_BASE_URL:-}" ]]; then
  LB_IP=$(kubectl get svc keycloak -n keycloak -o jsonpath='{.status.loadBalancer.ingress[0].ip}' 2>/dev/null || true)
  if [[ -n "${LB_IP}" ]]; then
    KC_LAB_BASE_URL="http://${LB_IP}:8080"
  else
    echo "ERROR: set KC_LAB_BASE_URL or ensure keycloak LoadBalancer has an EXTERNAL-IP" >&2
    exit 1
  fi
fi

cat > "$WORK/.env" <<EOF
KC_BASE_URL=${KC_LAB_BASE_URL}
KC_REALM=pim-test
KC_ADMIN_REALM=master
KC_ADMIN_CLIENT_ID=admin-cli
KC_ADMIN_USERNAME=admin
KC_ADMIN_PASSWORD=ChangeMe-LabOnly!
PIM_CLIENT_ID=pim-service
PIM_CLIENT_SECRET=
FIXTURE_PASSWORD=123456789
TEST_ROLE=pim-test-role
LEAD_USERNAME=superadmin1
LEAD_PASSWORD=123456789
ADMIN_USERNAME=admin1
ADMIN_PASSWORD=123456789
ADMIN2_USERNAME=admin2
ADMIN2_PASSWORD=123456789
PERMANENT_USERNAME=permanent1
PERMANENT_PASSWORD=123456789
BREAKGLASS_USERNAME=breakglass1
BREAKGLASS_PASSWORD=123456789
GROUP_BREAKGLASS=break-glass
GROUP_ADMIN_ACTIVE=admin-active
GROUP_ADMIN_ELIGIBLE=admin-eligible
GROUP_ADMIN_PERMANENT=admin-permanent
EOF

cd "$WORK"
set -a
# shellcheck disable=SC1091
source ./.env
set +a

echo ">> Target: ${KC_BASE_URL} realm=${KC_REALM}"

TOKEN=$(curl -sf \
  -d "client_id=${KC_ADMIN_CLIENT_ID}" \
  -d "username=${KC_ADMIN_USERNAME}" \
  -d "password=${KC_ADMIN_PASSWORD}" \
  -d "grant_type=password" \
  "${KC_BASE_URL}/realms/${KC_ADMIN_REALM}/protocol/openid-connect/token" | jq -r .access_token)

if [[ -z "$TOKEN" || "$TOKEN" == "null" ]]; then
  echo "ERROR: failed to obtain admin token" >&2
  exit 1
fi
echo ">> Admin token OK"

CODE=$(curl -s -o /tmp/realm.json -w "%{http_code}" \
  -H "Authorization: Bearer ${TOKEN}" -H "Content-Type: application/json" \
  "${KC_BASE_URL}/admin/realms/pim-test")

if [[ "$CODE" == "404" ]]; then
  curl -sf -H "Authorization: Bearer ${TOKEN}" -H "Content-Type: application/json" \
    -X POST "${KC_BASE_URL}/admin/realms" \
    -d '{"realm":"pim-test","enabled":true}'
  echo ">> realm pim-test created"
elif [[ "$CODE" == "200" ]]; then
  echo ">> realm pim-test exists"
else
  echo "ERROR: unexpected status ${CODE}" >&2
  exit 1
fi

chmod +x setup.sh plumbing_test.sh
./setup.sh

# Persist .env (with PIM_CLIENT_SECRET) back into the repo path (gitignored)
cp .env "$SRC_DIR/.env"
sed -i 's/\r$//' "$SRC_DIR/.env" || true

echo ">> Running plumbing_test.sh"
./plumbing_test.sh
