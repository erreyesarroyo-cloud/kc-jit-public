# Deploy PIM (Phases 1–3) to namespace keycloak-pim

Prerequisites: Phase 0 lab ready (Keycloak up, `scripts/.env` with secrets).

```powershell
cd keycloak-jit-access

# secrets
$env:PIM_CLIENT_SECRET = (Select-String -Path scripts\.env -Pattern '^PIM_CLIENT_SECRET=').Line.Split('=',2)[1].Trim('"')
$env:OIDC_CLIENT_SECRET = (Select-String -Path scripts\.env -Pattern '^OIDC_CLIENT_SECRET=').Line.Split('=',2)[1].Trim('"')
$env:SESSION_SECRET = -join ((1..32) | ForEach-Object { '{0:x}' -f (Get-Random -Max 16) })

kubectl create namespace keycloak-pim --dry-run=client -o yaml | kubectl apply -f -
kubectl -n keycloak-pim create secret generic pim-service-credentials `
  --from-literal=client-secret="$env:PIM_CLIENT_SECRET" `
  --dry-run=client -o yaml | kubectl apply -f -
kubectl -n keycloak-pim create secret generic pim-oidc-credentials `
  --from-literal=client-secret="$env:OIDC_CLIENT_SECRET" `
  --dry-run=client -o yaml | kubectl apply -f -

# Build and push to your registry, then set image.repository accordingly.
docker build -t keycloak-jit-access:0.1.0 .

$KC_IP = kubectl get svc keycloak -n keycloak -o jsonpath='{.status.loadBalancer.ingress[0].ip}'
helm upgrade --install pim ./deploy/helm/pim -n keycloak-pim `
  --set image.repository=keycloak-jit-access `
  --set image.tag=0.1.0 `
  --set sessionSecret="$env:SESSION_SECRET" `
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

Lab header auth (`X-Actor-Username`) is off in the Helm chart (`allowHeaderAuth: false`). Enable it only for plumbing tests with `--set allowHeaderAuth=true`.
