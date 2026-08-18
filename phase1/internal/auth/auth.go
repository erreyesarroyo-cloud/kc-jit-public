package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/jadeuc/keycloak-pim/internal/config"
)

const cookieName = "pim_session"

type Session struct {
	Username string   `json:"username"`
	Groups   []string `json:"groups"`
	Expiry   int64    `json:"exp"`
}

type pendingAuth struct {
	expiry       time.Time
	codeVerifier string
}

type Manager struct {
	cfg        config.Config
	httpClient *http.Client
	mu         sync.Mutex
	states     map[string]pendingAuth
}

func NewManager(cfg config.Config) *Manager {
	return &Manager{
		cfg:        cfg,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		states:     map[string]pendingAuth{},
	}
}

func (m *Manager) LoginURL() (string, error) {
	state, err := randomToken(16)
	if err != nil {
		return "", err
	}
	verifier, err := randomToken(32)
	if err != nil {
		return "", err
	}
	challenge := pkceChallengeS256(verifier)
	m.mu.Lock()
	m.states[state] = pendingAuth{
		expiry:       time.Now().Add(10 * time.Minute),
		codeVerifier: verifier,
	}
	m.mu.Unlock()
	q := url.Values{
		"client_id":             {m.cfg.OIDCClientID},
		"response_type":         {"code"},
		"scope":                 {"openid profile"},
		"redirect_uri":          {m.cfg.RedirectURL()},
		"state":                 {state},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
	}
	return fmt.Sprintf("%s/realms/%s/protocol/openid-connect/auth?%s",
		strings.TrimRight(m.cfg.BrowserIssuer(), "/"), m.cfg.Realm, q.Encode()), nil
}

func (m *Manager) HandleCallback(w http.ResponseWriter, r *http.Request) error {
	if errParam := r.URL.Query().Get("error"); errParam != "" {
		desc := r.URL.Query().Get("error_description")
		if desc != "" {
			return fmt.Errorf("oidc: %s: %s", errParam, desc)
		}
		return fmt.Errorf("oidc: %s", errParam)
	}
	state := r.URL.Query().Get("state")
	code := r.URL.Query().Get("code")
	if state == "" || code == "" {
		return errors.New("missing state or code")
	}
	m.mu.Lock()
	pending, ok := m.states[state]
	delete(m.states, state)
	m.mu.Unlock()
	if !ok || time.Now().After(pending.expiry) {
		return errors.New("invalid or expired state")
	}

	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {m.cfg.RedirectURL()},
		"client_id":     {m.cfg.OIDCClientID},
		"client_secret": {m.cfg.OIDCClientSecret},
		"code_verifier": {pending.codeVerifier},
	}
	tokURL := fmt.Sprintf("%s/realms/%s/protocol/openid-connect/token",
		strings.TrimRight(m.cfg.BaseURL, "/"), m.cfg.Realm)
	resp, err := m.httpClient.PostForm(tokURL, form)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return fmt.Errorf("token exchange: %s: %s", resp.Status, string(body))
	}
	var tok struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(body, &tok); err != nil {
		return err
	}
	if tok.AccessToken == "" {
		return errors.New("empty access_token")
	}

	ui, err := m.userInfo(tok.AccessToken)
	if err != nil {
		return err
	}
	if ui.PreferredUsername == "" {
		return errors.New("no preferred_username in userinfo")
	}
	sess := Session{
		Username: ui.PreferredUsername,
		Groups:   ui.Groups,
		Expiry:   time.Now().Add(8 * time.Hour).Unix(),
	}
	return m.setCookie(w, sess)
}

type userInfo struct {
	PreferredUsername string   `json:"preferred_username"`
	Groups            []string `json:"groups"`
}

func (m *Manager) userInfo(accessToken string) (*userInfo, error) {
	req, err := http.NewRequest(http.MethodGet,
		fmt.Sprintf("%s/realms/%s/protocol/openid-connect/userinfo",
			strings.TrimRight(m.cfg.BaseURL, "/"), m.cfg.Realm), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := m.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("userinfo: %s: %s", resp.Status, string(raw))
	}
	var ui userInfo
	if err := json.Unmarshal(raw, &ui); err != nil {
		return nil, err
	}
	for i, g := range ui.Groups {
		if idx := strings.LastIndex(g, "/"); idx >= 0 {
			ui.Groups[i] = g[idx+1:]
		}
	}
	return &ui, nil
}

func (m *Manager) Clear(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func (m *Manager) FromRequest(r *http.Request) (*Session, error) {
	c, err := r.Cookie(cookieName)
	if err != nil || c.Value == "" {
		return nil, errors.New("no session")
	}
	raw, err := base64.RawURLEncoding.DecodeString(c.Value)
	if err != nil {
		return nil, errors.New("bad session")
	}
	parts := strings.SplitN(string(raw), ".", 2)
	if len(parts) != 2 {
		return nil, errors.New("bad session")
	}
	payloadB, err1 := base64.RawURLEncoding.DecodeString(parts[0])
	sigB, err2 := base64.RawURLEncoding.DecodeString(parts[1])
	if err1 != nil || err2 != nil {
		return nil, errors.New("bad session encoding")
	}
	mac := hmac.New(sha256.New, []byte(m.cfg.SessionSecret))
	mac.Write(payloadB)
	if !hmac.Equal(mac.Sum(nil), sigB) {
		return nil, errors.New("bad session signature")
	}
	var sess Session
	if err := json.Unmarshal(payloadB, &sess); err != nil {
		return nil, errors.New("bad session")
	}
	if time.Now().Unix() > sess.Expiry {
		return nil, errors.New("session expired")
	}
	return &sess, nil
}

func (m *Manager) setCookie(w http.ResponseWriter, sess Session) error {
	payload, err := json.Marshal(sess)
	if err != nil {
		return err
	}
	mac := hmac.New(sha256.New, []byte(m.cfg.SessionSecret))
	mac.Write(payload)
	sig := mac.Sum(nil)
	val := base64.RawURLEncoding.EncodeToString(payload) + "." + base64.RawURLEncoding.EncodeToString(sig)
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    base64.RawURLEncoding.EncodeToString([]byte(val)),
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int((8 * time.Hour).Seconds()),
	})
	return nil
}

func randomToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func pkceChallengeS256(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func Actor(m *Manager, cfg config.Config, r *http.Request) string {
	if m != nil && cfg.OIDCEnabled {
		if sess, err := m.FromRequest(r); err == nil && sess.Username != "" {
			return sess.Username
		}
	}
	if cfg.AllowHeaderAuth {
		return strings.TrimSpace(r.Header.Get("X-Actor-Username"))
	}
	return ""
}

func SessionOrNil(m *Manager, r *http.Request) *Session {
	if m == nil {
		return nil
	}
	s, err := m.FromRequest(r)
	if err != nil {
		return nil
	}
	return s
}

func InList(haystack []string, needle string) bool {
	for _, h := range haystack {
		if h == needle {
			return true
		}
	}
	return false
}
