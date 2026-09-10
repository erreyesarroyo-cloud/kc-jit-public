#!/usr/bin/env bash
# LAB ONLY — recreate the original AKS lab (not required to run this project).
# Prefer: docker compose up --build   or   helm charts in deploy/helm/
#
# Prerequisites: az login, terraform, helm, kubectl, curl, jq, bash
set -euo pipefail

AKS_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "${AKS_DIR}/../../.." && pwd)"
CHART_DIR="${ROOT}/deploy/helm/keycloak"
SCRIPTS_DIR="${ROOT}/scripts"

echo "==> Terraform apply (optional AKS lab)"
cd "${AKS_DIR}"
if [[ ! -f terraform.tfvars ]]; then
  cp terraform.tfvars.example terraform.tfvars
fi
terraform init -input=false
terraform apply -auto-approve -input=false

echo "==> kubeconfig"
az aks get-credentials --resource-group kc-pim-rg --name kc-pim-aks --overwrite-existing

echo "==> Helm install Keycloak"
helm upgrade --install keycloak "${CHART_DIR}" -n keycloak --create-namespace --wait --timeout 15m

echo "==> Wait for LoadBalancer EXTERNAL-IP"
EXTERNAL_IP=""
for i in $(seq 1 60); do
  EXTERNAL_IP=$(kubectl get svc keycloak -n keycloak -o jsonpath='{.status.loadBalancer.ingress[0].ip}' 2>/dev/null || true)
  if [[ -n "${EXTERNAL_IP}" ]]; then
    break
  fi
  sleep 5
done
if [[ -z "${EXTERNAL_IP}" ]]; then
  echo "ERROR: LoadBalancer IP not ready" >&2
  exit 1
fi
KC_URL="http://${EXTERNAL_IP}:8080"
echo "    Keycloak URL: ${KC_URL}"

echo "==> Wait for Keycloak HTTP"
for i in $(seq 1 60); do
  code=$(curl -s -o /dev/null -w '%{http_code}' "${KC_URL}/" || true)
  if [[ "${code}" == "302" || "${code}" == "200" ]]; then
    break
  fi
  sleep 5
done

echo "==> Phase 0 setup (realm, groups, users, pim-service)"
export KC_LAB_BASE_URL="${KC_URL}"
sed -i 's/\r$//' "${SCRIPTS_DIR}/run-lab-setup.sh" "${SCRIPTS_DIR}/setup.sh" "${SCRIPTS_DIR}/plumbing_test.sh"
bash "${SCRIPTS_DIR}/run-lab-setup.sh"

echo ""
echo "PHASE 0 LAB READY"
echo "  Keycloak: ${KC_URL}"
echo "  Admin:    admin / ChangeMe-LabOnly!  (change me)"
echo "  Realm:    pim-test"
