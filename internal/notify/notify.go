package notify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/erreyesarroyo-cloud/keycloak-jit-access/internal/store"
)

// Notifier sends lifecycle alerts (Slack/Teams-style incoming webhooks).
type Notifier struct {
	webhookURL string
	publicURL  string
	client     *http.Client
	db         *store.Store
}

func New(webhookURL, publicURL string, db *store.Store) *Notifier {
	return &Notifier{
		webhookURL: strings.TrimSpace(webhookURL),
		publicURL:  strings.TrimRight(strings.TrimSpace(publicURL), "/"),
		client:     &http.Client{Timeout: 10 * time.Second},
		db:         db,
	}
}

func (n *Notifier) Enabled() bool {
	return n != nil && n.webhookURL != ""
}

func (n *Notifier) DeepLink(requestID string) string {
	if n.publicURL == "" {
		return ""
	}
	return fmt.Sprintf("%s/?requestId=%s", n.publicURL, requestID)
}

// NotifyOnce posts at most once per (requestID, kind). Returns whether a send was attempted.
func (n *Notifier) NotifyOnce(requestID, kind, text string) bool {
	if !n.Enabled() {
		return false
	}
	ok, err := n.db.TryMarkNotified(requestID, kind)
	if err != nil {
		log.Printf("notify mark %s/%s: %v", requestID, kind, err)
		return false
	}
	if !ok {
		return false // already sent — noise control
	}
	if err := n.post(text); err != nil {
		log.Printf("notify send %s/%s: %v", requestID, kind, err)
		return false
	}
	log.Printf("notify sent kind=%s request=%s", kind, requestID)
	return true
}

func (n *Notifier) PendingApprovers(req *store.Request) {
	link := n.DeepLink(req.ID)
	text := fmt.Sprintf(
		":lock: *PIM pending approval*\nRequester: `%s`\nJustification: %s\nExpires: %s\n<%s|Open in PIM to approve>",
		req.Username, req.Justification, req.RequestExpiresAt.UTC().Format(time.RFC3339), link,
	)
	n.NotifyOnce(req.ID, "pending_approvers", text)
}

func (n *Notifier) Approved(req *store.Request) {
	link := n.DeepLink(req.ID)
	exp := ""
	if req.ActiveExpiresAt != nil {
		exp = req.ActiveExpiresAt.UTC().Format(time.RFC3339)
	}
	text := fmt.Sprintf(
		":white_check_mark: *PIM approved*\nUser `%s` elevated by `%s`\nNotes: %s\nActive until: %s\n<%s|Open in PIM>",
		req.Username, req.ApprovedBy, req.ApprovalNotes, exp, link,
	)
	n.NotifyOnce(req.ID, "approved_requester", text)
}

func (n *Notifier) RequestExpired(req *store.Request) {
	text := fmt.Sprintf(
		":hourglass: *PIM request expired*\nUser `%s` — no approval within TTL.\nJustification: %s",
		req.Username, req.Justification,
	)
	n.NotifyOnce(req.ID, "expired_requester", text)
}

func (n *Notifier) Released(req *store.Request, reason string) {
	text := fmt.Sprintf(
		":unlock: *PIM access ended* (%s)\nUser `%s` removed from admin-active.\nJustification: %s",
		reason, req.Username, req.Justification,
	)
	n.NotifyOnce(req.ID, "released_"+reason, text)
}

func (n *Notifier) post(text string) error {
	body, _ := json.Marshal(map[string]string{"text": text})
	resp, err := n.client.Post(n.webhookURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
	if resp.StatusCode >= 300 {
		return fmt.Errorf("webhook %s: %s", resp.Status, string(raw))
	}
	return nil
}
