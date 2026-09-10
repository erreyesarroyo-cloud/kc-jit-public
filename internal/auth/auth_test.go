package auth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/erreyesarroyo-cloud/keycloak-jit-access/internal/config"
)

func TestActorHeaderAuth(t *testing.T) {
	cfg := config.Config{AllowHeaderAuth: true, OIDCEnabled: false}
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("X-Actor-Username", " alice ")
	if got := Actor(nil, cfg, r); got != "alice" {
		t.Fatalf("got %q", got)
	}
}

func TestActorHeaderAuthDisabled(t *testing.T) {
	cfg := config.Config{AllowHeaderAuth: false, OIDCEnabled: false}
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("X-Actor-Username", "alice")
	if got := Actor(nil, cfg, r); got != "" {
		t.Fatalf("header must be ignored, got %q", got)
	}
}

func TestSessionRoundTripAndTamper(t *testing.T) {
	cfg := config.Config{SessionSecret: "test-secret", OIDCEnabled: true}
	m := NewManager(cfg)
	rec := httptest.NewRecorder()
	sess := Session{Username: "bob", Expiry: time.Now().Add(time.Hour).Unix()}
	if err := m.setCookie(rec, sess); err != nil {
		t.Fatal(err)
	}
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("cookies=%d", len(cookies))
	}

	okReq := httptest.NewRequest(http.MethodGet, "/", nil)
	okReq.AddCookie(cookies[0])
	got, err := m.FromRequest(okReq)
	if err != nil {
		t.Fatal(err)
	}
	if got.Username != "bob" {
		t.Fatalf("username=%q", got.Username)
	}
	if Actor(m, cfg, okReq) != "bob" {
		t.Fatalf("Actor=%q", Actor(m, cfg, okReq))
	}

	bad := *cookies[0]
	bad.Value = bad.Value + "tamper"
	badReq := httptest.NewRequest(http.MethodGet, "/", nil)
	badReq.AddCookie(&bad)
	if _, err := m.FromRequest(badReq); err == nil {
		t.Fatal("tampered cookie should fail")
	}
}

func TestLoginURLIncludesPKCE(t *testing.T) {
	cfg := config.Config{
		OIDCClientID: "pim-web",
		PublicURL:    "http://localhost:8081",
		Realm:        "pim-test",
		BaseURL:      "http://keycloak:8080",
	}
	m := NewManager(cfg)
	u, err := m.LoginURL()
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"/realms/pim-test/protocol/openid-connect/auth",
		"client_id=pim-web",
		"code_challenge_method=S256",
		"code_challenge=",
	} {
		if !strings.Contains(u, want) {
			t.Fatalf("LoginURL missing %q: %s", want, u)
		}
	}
}
