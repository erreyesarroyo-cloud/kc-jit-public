# Optional lab: AKS

This directory is **not** part of the public install path. It is the Terraform used
to prove the first lab on Azure Kubernetes Service (Free-tier control plane).

To try the project, use Docker Compose at the repo root, or the generic Helm
charts in [`../../helm/`](../../helm/). See [`../../README.md`](../../README.md).

## Prerequisites

- Azure CLI (`az login`)
- Terraform >= 1.5
- Subscription with quota for 1x `Standard_B2s` (or change `vm_size`)

## Apply

```powershell
cd deploy\infra\aks
copy terraform.tfvars.example terraform.tfvars
# edit terraform.tfvars if needed

terraform init
terraform plan
terraform apply
```

One-shot recreate (AKS + Keycloak + Phase 0 fixtures):

```bash
./bootstrap.sh
```

## Kubeconfig

```powershell
terraform output -raw kube_config_command | Invoke-Expression
kubectl get nodes
```

## Install Keycloak

```powershell
cd ..\..\helm\keycloak
helm upgrade --install keycloak . -n keycloak --create-namespace
```

## Destroy

```bash
./destroy.sh
```

or `terraform destroy` in this directory.

## Notes

- `sku_tier = Free` — no control-plane SLA; lab only.
- Single node pool hosts system pods and Keycloak; do not run heavy app workloads here.
- VM cost is **not** free; Free tier only removes the AKS control-plane fee. Use a free-trial credit or stop/destroy when idle.
