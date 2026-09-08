# Deploy PIM (Phases 1–3) to namespace keycloak-pim

Prerequisites: Phase 0 lab ready (Keycloak up, `phase0/.env` with secrets).

```powershell
cd keycloack_extention_pim

# secrets
$env:PIM_CLIENT_SECRET = (Select-String -Path phase0\.env -Pattern '^PIM_CLIENT_SECRET=').Line.Split('=',2)[1].Trim('"')
$env:OIDC_CLIENT_SECRET = (Select-String -Path phase0\.env -Pattern '^OIDC_CLIENT_SECRET=').Line.Split('=',2)[1].Trim('"')

kubectl create namespace keycloak-pim --dry-run=client -o yaml | kubectl apply -f -
kubectl -n keycloak-pim create secret generic pim-service-credentials `
  --from-literal=client-secret="$env:PIM_CLIENT_SECRET" `
  --dry-run=client -o yaml | kubectl apply -f -
kubectl -n keycloak-pim create secret generic pim-oidc-credentials `
  --from-literal=client-secret="$env:OIDC_CLIENT_SECRET" `
  --dry-run=client -o yaml | kubectl apply -f -

# ACR build (replace registry if needed)
$ACR = (az acr list -g kc-pim-rg --query "[0].name" -o tsv)
if (-not $ACR) { az acr create -g kc-pim-rg -n kcpimlabacr$((Get-Random -Max 9999)) --sku Basic }
$ACR = (az acr list -g kc-pim-rg --query "[0].name" -o tsv)
az aks update -g kc-pim-rg -n kc-pim-aks --attach-acr $ACR
az acr build -r $ACR -t pim:0.3.0 ./phase1

$KC_IP = kubectl get svc keycloak -n keycloak -o jsonpath='{.status.loadBalancer.ingress[0].ip}'
helm upgrade --install pim ./charts/pim -n keycloak-pim `
  --set image.repository="$ACR.azurecr.io/pim" `
  --set image.tag=0.3.0 `
  --set notify.enabled=true `
  --set keycloak.publicURL="http://${KC_IP}:8080" `
  --set publicURL="http://127.0.0.1:8081"

# Phase 4 Slack/Teams webhook (optional; create secret first)
# kubectl -n keycloak-pim create secret generic pim-notify-webhook --from-literal=url='https://hooks.slack.com/services/...'


kubectl -n keycloak-pim rollout status deploy/pim
kubectl -n keycloak-pim port-forward svc/pim 8081:8080
# open http://127.0.0.1:8081
```

## API additions (Phase 2–3)

| Method | Path | Notes |
| --- | --- | --- |
| GET | `/api/v1/audit` | `?breakGlass=true&requestId=&actor=&limit=` |
| GET | `/api/v1/me` | actor groups + flags |
| GET | `/auth/login` | OIDC start |
| GET | `/auth/callback` | OIDC callback |
| GET | `/` | thin UI |

Lab header auth (`X-Actor-Username`) remains enabled by default.
