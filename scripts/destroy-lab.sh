#!/usr/bin/env bash
# Tear down lab AKS (keeps all Phase 0 docs/scripts in git).
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT}/infra/aks"
terraform destroy -auto-approve -input=false
echo "Lab cluster destroyed. Docs/scripts untouched."
