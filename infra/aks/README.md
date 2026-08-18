# AKS (Free tier) — Keycloak PIM access cluster

Terraform for a small **AKS Free** control-plane cluster used as the **access / management** cluster (Keycloak + PIM later).

## Prerequisites

- Azure CLI (`az login`)
- Terraform >= 1.5
- Subscription with quota for 1x `Standard_B2s` (or change `vm_size`)

## Apply

```powershell
cd infra\aks
copy terraform.tfvars.example terraform.tfvars
# edit terraform.tfvars if needed

terraform init
terraform plan
terraform apply
```

## Kubeconfig

```powershell
terraform output -raw kube_config_command | Invoke-Expression
kubectl get nodes
```

## Install Keycloak

```powershell
cd ..\..\charts\keycloak
helm upgrade --install keycloak . -n keycloak --create-namespace
```

## Destroy

```powershell
cd infra\aks
terraform destroy
```

## Notes

- `sku_tier = Free` — no control-plane SLA; lab only.
- Single node pool hosts system pods and Keycloak; do not run heavy app workloads here.
- VM cost is **not** free; Free tier only removes the AKS control-plane fee. Use a free-trial credit or stop/destroy when idle.
