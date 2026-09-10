#!/usr/bin/env bash
# LAB ONLY — tear down the original AKS lab cluster.
set -euo pipefail
cd "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
terraform destroy -auto-approve -input=false
echo "Lab cluster destroyed. Docs/scripts untouched."
