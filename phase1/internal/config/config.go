package config

import (
	"os"
	"strings"
	"time"
)

type Config struct {
	ListenAddr string
	DBPath     string
	PublicURL  string

	BaseURL       string // server-side Keycloak (in-cluster OK)
	PublicBaseURL string // browser-facing Keycloak (LB / port-forward)
	Realm         string
	ClientID      string
	ClientSecret  string

	OIDCEnabled      bool
	OIDCClientID     string
	OIDCClientSecret string
	OIDCRedirectURL  string
	SessionSecret    string
	AllowHeaderAuth  bool

	NotifyWebhookURL string

	GroupEligible   string
	GroupActive     string
	GroupPermanent  string
	GroupBreakGlass string

	RequestTTL   time.Duration
	ActiveTTL    time.Duration
	PollInterval time.Duration
}

func FromEnv() Config {
	return Config{
		ListenAddr:       getenv("LISTEN_ADDR", ":8080"),
		DBPath:           getenv("DB_PATH", "/data/pim.db"),
		PublicURL:        strings.TrimRight(getenv("PUBLIC_URL", "http://127.0.0.1:8081"), "/"),
		BaseURL:          getenv("KC_BASE_URL", "http://keycloak.keycloak.svc.cluster.local:8080"),
		PublicBaseURL:    strings.TrimRight(getenv("KC_PUBLIC_URL", ""), "/"),
		Realm:            getenv("KC_REALM", "pim-test"),
		ClientID:         getenv("PIM_CLIENT_ID", "pim-service"),
		ClientSecret:     os.Getenv("PIM_CLIENT_SECRET"),
		OIDCEnabled:      boolEnv("OIDC_ENABLED", true),
		OIDCClientID:     getenv("OIDC_CLIENT_ID", "pim-web"),
		OIDCClientSecret: os.Getenv("OIDC_CLIENT_SECRET"),
		OIDCRedirectURL:  getenv("OIDC_REDIRECT_URL", ""),
		SessionSecret:    getenv("SESSION_SECRET", "lab-only-change-me"),
		AllowHeaderAuth:  boolEnv("ALLOW_HEADER_AUTH", true),
		NotifyWebhookURL: strings.TrimSpace(os.Getenv("NOTIFY_WEBHOOK_URL")),
		GroupEligible:    getenv("GROUP_ADMIN_ELIGIBLE", "admin-eligible"),
		GroupActive:      getenv("GROUP_ADMIN_ACTIVE", "admin-active"),
		GroupPermanent:   getenv("GROUP_ADMIN_PERMANENT", "admin-permanent"),
		GroupBreakGlass:  getenv("GROUP_BREAKGLASS", "break-glass"),
		RequestTTL:       durationEnv("REQUEST_TTL", time.Hour),
		ActiveTTL:        durationEnv("ACTIVE_TTL", 9*time.Hour),
		PollInterval:     durationEnv("POLL_INTERVAL", 30*time.Second),
	}
}

func (c Config) RedirectURL() string {
	if c.OIDCRedirectURL != "" {
		return c.OIDCRedirectURL
	}
	return c.PublicURL + "/auth/callback"
}

// BrowserIssuer is the Keycloak base URL used in browser redirects.
func (c Config) BrowserIssuer() string {
	if c.PublicBaseURL != "" {
		return c.PublicBaseURL
	}
	return c.BaseURL
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func durationEnv(k string, def time.Duration) time.Duration {
	v := os.Getenv(k)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return def
	}
	return d
}

func boolEnv(k string, def bool) bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(k)))
	if v == "" {
		return def
	}
	return v == "1" || v == "true" || v == "yes" || v == "on"
}
