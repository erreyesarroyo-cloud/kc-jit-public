package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/erreyesarroyo-cloud/keycloak-jit-access/internal/config"
	"github.com/erreyesarroyo-cloud/keycloak-jit-access/internal/store"
)

type fakeGroups struct {
	mu     sync.Mutex
	member map[string]map[string]bool
}

func newFake(groups map[string][]string) *fakeGroups {
	f := &fakeGroups{member: map[string]map[string]bool{}}
	for user, gs := range groups {
		f.member[user] = map[string]bool{}
		for _, g := range gs {
			f.member[user][g] = true
		}
	}
	return f
}

func (f *fakeGroups) InGroup(username, groupName string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.member[username][groupName], nil
}

func (f *fakeGroups) AddToGroup(username, groupName string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.member[username] == nil {
		f.member[username] = map[string]bool{}
	}
	f.member[username][groupName] = true
	return nil
}

func (f *fakeGroups) RemoveFromGroup(username, groupName string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.member[username] != nil {
		delete(f.member[username], groupName)
	}
	return nil
}

func testCfg() config.Config {
	return config.Config{
		GroupEligible:   "admin-eligible",
		GroupActive:     "admin-active",
		GroupPermanent:  "admin-permanent",
		GroupBreakGlass: "break-glass",
		RequestTTL:      time.Hour,
		ActiveTTL:       9 * time.Hour,
		AllowHeaderAuth: true,
		OIDCEnabled:     false,
	}
}

func testEnv(t *testing.T, groups map[string][]string) (*Service, *fakeGroups, http.Handler) {
	t.Helper()
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	kc := newFake(groups)
	svc := NewService(testCfg(), db, kc, nil, nil)
	return svc, kc, NewRouter(svc)
}

func doJSON(t *testing.T, h http.Handler, method, path, actor, body string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	if actor != "" {
		r.Header.Set("X-Actor-Username", actor)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	return rec
}

func TestCreateRejectsNonEligible(t *testing.T) {
	_, _, h := testEnv(t, map[string][]string{"alice": {}})
	rec := doJSON(t, h, http.MethodPost, "/api/v1/requests", "alice",
		`{"justification":"need access for incident"}`)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestCreateRejectsShortJustification(t *testing.T) {
	_, _, h := testEnv(t, map[string][]string{"alice": {"admin-eligible"}})
	rec := doJSON(t, h, http.MethodPost, "/api/v1/requests", "alice",
		`{"justification":"short"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestGrantRevokeFlow(t *testing.T) {
	svc, kc, h := testEnv(t, map[string][]string{
		"alice": {"admin-eligible"},
		"bob":   {"admin-permanent"},
	})

	rec := doJSON(t, h, http.MethodPost, "/api/v1/requests", "alice",
		`{"justification":"need access for incident"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", rec.Code, rec.Body.String())
	}
	var created store.Request
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}

	rec = doJSON(t, h, http.MethodPost, "/api/v1/requests/"+created.ID+"/approve", "alice",
		`{"notes":"looks reasonable"}`)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("eligible must not approve: status=%d body=%s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, h, http.MethodPost, "/api/v1/requests/"+created.ID+"/approve", "bob", `{}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("notes required: status=%d body=%s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, h, http.MethodPost, "/api/v1/requests/"+created.ID+"/approve", "bob",
		`{"notes":"approved for the window"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("approve status=%d body=%s", rec.Code, rec.Body.String())
	}
	ok, _ := kc.InGroup("alice", "admin-active")
	if !ok {
		t.Fatal("alice should be in admin-active after approve")
	}

	rec = doJSON(t, h, http.MethodPost, "/api/v1/requests/"+created.ID+"/release", "mallory", ``)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("stranger release: status=%d body=%s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, h, http.MethodPost, "/api/v1/requests/"+created.ID+"/release", "alice", ``)
	if rec.Code != http.StatusOK {
		t.Fatalf("release status=%d body=%s", rec.Code, rec.Body.String())
	}
	ok, _ = kc.InGroup("alice", "admin-active")
	if ok {
		t.Fatal("alice should be removed from admin-active after release")
	}

	items, err := svc.db.ListAudit(store.AuditFilter{RequestID: created.ID, Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) == 0 {
		t.Fatal("expected audit entries for the grant/revoke flow")
	}
}

func TestApproveExpiredRequest(t *testing.T) {
	svc, _, h := testEnv(t, map[string][]string{
		"alice": {"admin-eligible"},
		"bob":   {"admin-permanent"},
	})
	now := time.Now().UTC()
	req := &store.Request{
		ID:               "expired-1",
		Username:         "alice",
		Justification:    "need access now",
		Status:           store.StatusPending,
		CreatedAt:        now.Add(-2 * time.Hour),
		RequestExpiresAt: now.Add(-time.Minute),
	}
	if err := svc.db.Create(req); err != nil {
		t.Fatal(err)
	}
	rec := doJSON(t, h, http.MethodPost, "/api/v1/requests/expired-1/approve", "bob",
		`{"notes":"too late anyway"}`)
	if rec.Code != http.StatusConflict {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestReconcileRevokesExpiredGrant(t *testing.T) {
	svc, kc, _ := testEnv(t, map[string][]string{
		"alice": {"admin-eligible", "admin-active"},
	})
	now := time.Now().UTC()
	exp := now.Add(-time.Minute)
	req := &store.Request{
		ID:               "stale-1",
		Username:         "alice",
		Justification:    "need access now",
		Status:           store.StatusApproved,
		CreatedAt:        now.Add(-10 * time.Hour),
		RequestExpiresAt: now.Add(-9 * time.Hour),
		ApprovedBy:       "bob",
		ActiveExpiresAt:  &exp,
	}
	if err := svc.db.Create(req); err != nil {
		t.Fatal(err)
	}
	if err := svc.Reconcile(context.Background()); err != nil {
		t.Fatal(err)
	}
	got, err := svc.db.Get("stale-1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != store.StatusReleased {
		t.Fatalf("status=%s", got.Status)
	}
	ok, _ := kc.InGroup("alice", "admin-active")
	if ok {
		t.Fatal("expired grant must be removed from admin-active")
	}
}

func TestHeaderAuthRequiredWhenEnabled(t *testing.T) {
	_, _, h := testEnv(t, map[string][]string{"bob": {"admin-permanent"}})
	rec := doJSON(t, h, http.MethodPost, "/api/v1/requests/x/approve", "",
		`{"notes":"approved for the window"}`)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}
