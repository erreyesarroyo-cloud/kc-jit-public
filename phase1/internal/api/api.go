package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/erreyesarroyo-cloud/keycloak-pim-extension/internal/auth"
	"github.com/erreyesarroyo-cloud/keycloak-pim-extension/internal/config"
	"github.com/erreyesarroyo-cloud/keycloak-pim-extension/internal/keycloak"
	"github.com/erreyesarroyo-cloud/keycloak-pim-extension/internal/notify"
	"github.com/erreyesarroyo-cloud/keycloak-pim-extension/internal/store"
	"github.com/erreyesarroyo-cloud/keycloak-pim-extension/internal/ui"
)

type Service struct {
	cfg    config.Config
	db     *store.Store
	kc     *keycloak.Client
	auth   *auth.Manager
	notify *notify.Notifier
}

func NewService(cfg config.Config, db *store.Store, kc *keycloak.Client, am *auth.Manager, n *notify.Notifier) *Service {
	return &Service{cfg: cfg, db: db, kc: kc, auth: am, notify: n}
}

type createRequestBody struct {
	Username      string `json:"username"`
	Justification string `json:"justification"`
}

type approveRequestBody struct {
	Notes string `json:"notes"`
}

const minJustificationLen = 8
const minApprovalNotesLen = 8

func NewRouter(svc *Service) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	mux.HandleFunc("GET /auth/login", svc.handleLogin)
	mux.HandleFunc("GET /auth/callback", svc.handleCallback)
	mux.HandleFunc("POST /auth/logout", svc.handleLogout)
	mux.HandleFunc("GET /auth/logout", svc.handleLogout)

	mux.HandleFunc("GET /api/v1/me", svc.handleMe)
	mux.HandleFunc("GET /api/v1/requests", svc.handleList)
	mux.HandleFunc("POST /api/v1/requests", svc.handleCreate)
	mux.HandleFunc("GET /api/v1/requests/{id}", svc.handleGet)
	mux.HandleFunc("POST /api/v1/requests/{id}/approve", svc.handleApprove)
	mux.HandleFunc("POST /api/v1/requests/{id}/release", svc.handleRelease)
	mux.HandleFunc("GET /api/v1/audit", svc.handleAudit)

	mux.Handle("/", ui.Handler())
	return mux
}

func (s *Service) handleLogin(w http.ResponseWriter, r *http.Request) {
	if !s.cfg.OIDCEnabled || s.auth == nil {
		http.Error(w, "OIDC disabled", http.StatusNotFound)
		return
	}
	u, err := s.auth.LoginURL()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	http.Redirect(w, r, u, http.StatusFound)
}

func (s *Service) handleCallback(w http.ResponseWriter, r *http.Request) {
	if !s.cfg.OIDCEnabled || s.auth == nil {
		http.Error(w, "OIDC disabled", http.StatusNotFound)
		return
	}
	if err := s.auth.HandleCallback(w, r); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	http.Redirect(w, r, "/", http.StatusFound)
}

func (s *Service) handleLogout(w http.ResponseWriter, r *http.Request) {
	if s.auth != nil {
		s.auth.Clear(w)
	}
	http.Redirect(w, r, "/", http.StatusFound)
}

func (s *Service) handleMe(w http.ResponseWriter, r *http.Request) {
	actor := s.actor(r)
	if actor == "" {
		writeErr(w, http.StatusUnauthorized, errors.New("not authenticated"))
		return
	}
	eligible, _ := s.kc.InGroup(actor, s.cfg.GroupEligible)
	permanent, _ := s.kc.InGroup(actor, s.cfg.GroupPermanent)
	active, _ := s.kc.InGroup(actor, s.cfg.GroupActive)
	breakGlass, _ := s.kc.InGroup(actor, s.cfg.GroupBreakGlass)
	if breakGlass {
		_ = s.audit("break_glass_session", actor, actor, "", "", "break-glass user authenticated to PIM", true)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"username":    actor,
		"eligible":    eligible,
		"permanent":   permanent,
		"active":      active,
		"breakGlass":  breakGlass,
		"requestTTL":  s.cfg.RequestTTL.String(),
		"activeTTL":   s.cfg.ActiveTTL.String(),
		"oidcEnabled": s.cfg.OIDCEnabled,
	})
}

func (s *Service) handleAudit(w http.ResponseWriter, r *http.Request) {
	f := store.AuditFilter{RequestID: r.URL.Query().Get("requestId"), Actor: r.URL.Query().Get("actor")}
	if v := r.URL.Query().Get("breakGlass"); v != "" {
		b := v == "1" || strings.EqualFold(v, "true")
		f.BreakGlass = &b
	}
	if lim := r.URL.Query().Get("limit"); lim != "" {
		if n, err := strconv.Atoi(lim); err == nil {
			f.Limit = n
		}
	}
	items, err := s.db.ListAudit(f)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Service) handleList(w http.ResponseWriter, r *http.Request) {
	items, err := s.db.List()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Service) handleGet(w http.ResponseWriter, r *http.Request) {
	item, err := s.db.Get(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Service) handleCreate(w http.ResponseWriter, r *http.Request) {
	actor := s.actor(r)
	var body createRequestBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	body.Username = strings.TrimSpace(body.Username)
	body.Justification = strings.TrimSpace(body.Justification)
	if body.Username == "" {
		body.Username = actor
	}
	if body.Username == "" || body.Justification == "" {
		writeErr(w, http.StatusBadRequest, errors.New("username and justification required"))
		return
	}
	if len(body.Justification) < minJustificationLen {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("justification must be at least %d characters", minJustificationLen))
		return
	}
	if actor != "" && actor != body.Username {
		writeErr(w, http.StatusForbidden, errors.New("actor may only request for self"))
		return
	}
	ok, err := s.kc.InGroup(body.Username, s.cfg.GroupEligible)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err)
		return
	}
	if !ok {
		writeErr(w, http.StatusForbidden, fmt.Errorf("user %s is not in %s", body.Username, s.cfg.GroupEligible))
		return
	}
	now := time.Now().UTC()
	req := &store.Request{
		ID:               newID(),
		Username:         body.Username,
		Justification:    body.Justification,
		Status:           store.StatusPending,
		CreatedAt:        now,
		RequestExpiresAt: now.Add(s.cfg.RequestTTL),
	}
	if err := s.db.Create(req); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	bg, _ := s.kc.InGroup(body.Username, s.cfg.GroupBreakGlass)
	_ = s.audit("request_created", actor, body.Username, req.ID, body.Justification, "pending elevation request", bg)
	if s.notify != nil {
		s.notify.PendingApprovers(req)
	}
	writeJSON(w, http.StatusCreated, req)
}

func (s *Service) handleApprove(w http.ResponseWriter, r *http.Request) {
	actor := s.actor(r)
	if actor == "" {
		writeErr(w, http.StatusUnauthorized, errors.New("authentication required"))
		return
	}
	ok, err := s.kc.InGroup(actor, s.cfg.GroupPermanent)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err)
		return
	}
	if !ok {
		writeErr(w, http.StatusForbidden, fmt.Errorf("actor %s is not in %s", actor, s.cfg.GroupPermanent))
		return
	}
	var body approveRequestBody
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&body); err != nil && !errors.Is(err, io.EOF) {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	body.Notes = strings.TrimSpace(body.Notes)
	if body.Notes == "" {
		writeErr(w, http.StatusBadRequest, errors.New("approver notes are required"))
		return
	}
	if len(body.Notes) < minApprovalNotesLen {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("approver notes must be at least %d characters", minApprovalNotesLen))
		return
	}
	req, err := s.db.Get(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusNotFound, err)
		return
	}
	if req.Status != store.StatusPending {
		writeErr(w, http.StatusConflict, fmt.Errorf("request status is %s", req.Status))
		return
	}
	now := time.Now().UTC()
	if now.After(req.RequestExpiresAt) {
		req.Status = store.StatusExpired
		_ = s.db.Update(req)
		_ = s.audit("request_expired", "system", req.Username, req.ID, req.Justification, "expired before approve", false)
		if s.notify != nil {
			s.notify.RequestExpired(req)
		}
		writeErr(w, http.StatusConflict, errors.New("request expired"))
		return
	}
	if err := s.kc.AddToGroup(req.Username, s.cfg.GroupActive); err != nil {
		writeErr(w, http.StatusBadGateway, err)
		return
	}
	exp := now.Add(s.cfg.ActiveTTL)
	req.Status = store.StatusApproved
	req.ApprovedAt = &now
	req.ApprovedBy = actor
	req.ApprovalNotes = body.Notes
	req.ActiveExpiresAt = &exp
	if err := s.db.Update(req); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	bg, _ := s.kc.InGroup(actor, s.cfg.GroupBreakGlass)
	_ = s.audit("request_approved", actor, req.Username, req.ID, req.Justification, "notes: "+body.Notes, bg)
	_ = s.audit("grant_active", actor, req.Username, req.ID, req.Justification, "user added to "+s.cfg.GroupActive, bg)
	if s.notify != nil {
		s.notify.Approved(req)
	}
	writeJSON(w, http.StatusOK, req)
}

func (s *Service) handleRelease(w http.ResponseWriter, r *http.Request) {
	actor := s.actor(r)
	req, err := s.db.Get(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusNotFound, err)
		return
	}
	if req.Status != store.StatusApproved {
		writeErr(w, http.StatusConflict, fmt.Errorf("request status is %s", req.Status))
		return
	}
	if actor != "" && actor != req.Username {
		perm, err := s.kc.InGroup(actor, s.cfg.GroupPermanent)
		if err != nil {
			writeErr(w, http.StatusBadGateway, err)
			return
		}
		if !perm {
			writeErr(w, http.StatusForbidden, errors.New("only subject or admin-permanent can release"))
			return
		}
	}
	if err := s.release(req, actor, "early_release"); err != nil {
		writeErr(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, req)
}

func (s *Service) release(req *store.Request, actor, action string) error {
	if err := s.kc.RemoveFromGroup(req.Username, s.cfg.GroupActive); err != nil {
		return err
	}
	now := time.Now().UTC()
	req.Status = store.StatusReleased
	req.ReleasedAt = &now
	if err := s.db.Update(req); err != nil {
		return err
	}
	if actor == "" {
		actor = "system"
	}
	_ = s.audit(action, actor, req.Username, req.ID, req.Justification, "removed from "+s.cfg.GroupActive, false)
	if s.notify != nil {
		reason := "release"
		if action == "grant_expired" {
			reason = "expire"
		} else if action == "early_release" {
			reason = "early_release"
		}
		s.notify.Released(req, reason)
	}
	return nil
}

func (s *Service) Reconcile(ctx context.Context) error {
	_ = ctx
	now := time.Now().UTC()
	pending, err := s.db.PendingExpired(now)
	if err != nil {
		return err
	}
	for i := range pending {
		pending[i].Status = store.StatusExpired
		if err := s.db.Update(&pending[i]); err != nil {
			log.Printf("expire request %s: %v", pending[i].ID, err)
		} else {
			log.Printf("expired pending request %s user=%s", pending[i].ID, pending[i].Username)
			_ = s.audit("request_expired", "system", pending[i].Username, pending[i].ID, pending[i].Justification, "request TTL elapsed", false)
			if s.notify != nil {
				s.notify.RequestExpired(&pending[i])
			}
		}
	}
	active, err := s.db.ActiveExpired(now)
	if err != nil {
		return err
	}
	for i := range active {
		if err := s.kc.RemoveFromGroup(active[i].Username, s.cfg.GroupActive); err != nil {
			log.Printf("revoke active %s: %v", active[i].ID, err)
			continue
		}
		now := time.Now().UTC()
		active[i].Status = store.StatusReleased
		active[i].ReleasedAt = &now
		if err := s.db.Update(&active[i]); err != nil {
			log.Printf("update released %s: %v", active[i].ID, err)
		} else {
			log.Printf("revoked active grant %s user=%s", active[i].ID, active[i].Username)
			_ = s.audit("grant_expired", "system", active[i].Username, active[i].ID, active[i].Justification, "active TTL elapsed; revoked", false)
			if s.notify != nil {
				s.notify.Released(&active[i], "expire")
			}
		}
	}
	return nil
}

func (s *Service) actor(r *http.Request) string {
	return auth.Actor(s.auth, s.cfg, r)
}

func (s *Service) audit(action, actor, subject, requestID, justification, detail string, breakGlass bool) error {
	e := &store.AuditEvent{
		Action:        action,
		Actor:         actor,
		Subject:       subject,
		RequestID:     requestID,
		Justification: justification,
		Detail:        detail,
		BreakGlass:    breakGlass,
	}
	if err := s.db.AppendAudit(e); err != nil {
		log.Printf("audit write failed action=%s: %v", action, err)
		return err
	}
	if breakGlass {
		log.Printf("ALERT break-glass action=%s actor=%s subject=%s request=%s", action, actor, subject, requestID)
	}
	return nil
}

func newID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, err error) {
	writeJSON(w, code, map[string]string{"error": err.Error()})
}
