# Phase 4 — Findings (Slack webhook notifications)

**Date:** 2026-08-15  
**Status:** **PASS** (DofD met)  
**Image:** lab `pim:0.3.0`  
**Namespace:** `keycloak-pim`  
**Channel:** Slack free workspace → `#new-channel` (Incoming Webhook)

## DofD checklist

| # | Criterion | Status |
| --- | --- | --- |
| 1 | In-app queue remains primary approve path | **PASS** (Phase 3 UI) |
| 2 | Pending notifies approvers via email **or** webhook | **PASS** — Slack Incoming Webhook |
| 3 | Deep-link into app (authenticated approve) | **PASS** — `/?requestId=` in Slack + UI highlight |
| 4 | Requester notified on approve / expire / revoke | **PASS** — channel messages (lab shared channel) |
| 5 | Noise controls (no spam on reconcile) | **PASS** — SQLite `notifications` PK `(request_id, kind)` |

## Lab validation (request `4e5e3d7b`)

| Time (UTC) | Event | Evidence |
| --- | --- | --- |
| 23:14:03 | `admin1` creates request | Slack *PIM pending approval*; `notify sent kind=pending_approvers` |
| 23:15:12 | `permanent1` approves | Slack *PIM approved*; audit `grant_active`; PIM `/me` → `active: true` |
| 23:16:18 | early release | Slack *PIM access ended*; audit `early_release`; `/me` → `active: false` |

End-to-end: **Slack → approve in UI → Keycloak `admin-active` → release → revoke**.

## Config

```powershell
# Secret only — never commit the webhook URL
kubectl -n keycloak-pim create secret generic pim-notify-webhook `
  --from-literal=url='https://hooks.slack.com/services/...' `
  --dry-run=client -o yaml | kubectl apply -f -

$KC_IP = kubectl get svc keycloak -n keycloak -o jsonpath='{.status.loadBalancer.ingress[0].ip}'
helm upgrade --install pim ./deploy/helm/pim -n keycloak-pim `
  --set image.repository=keycloak-jit-access `
  --set image.tag=0.3.0 `
  --set sessionSecret="$(openssl rand -hex 32)" `
  --set notify.enabled=true `
  --set keycloak.publicURL="http://${KC_IP}:8080" `
  --set publicURL="http://127.0.0.1:8081"

kubectl -n keycloak-pim port-forward svc/pim 8081:8080
```

## Implementation notes

- Package: `internal/notify` — Slack/Teams-style `{"text":"..."}` POST.
- Env: `NOTIFY_WEBHOOK_URL` from secret `pim-notify-webhook` / key `url`.
- Helm: `notify.enabled=true` + `notify.existingSecret`.
- **Rocket.Chat / Teams:** same webhook pattern; swap URL in the secret (Slack-compatible payloads often work; adjust JSON if needed).
- Email/SMTP and full Rocket.Chat hosting left for a later pass.
- Rotate the webhook in Slack app settings if it was exposed outside the lab secret.
