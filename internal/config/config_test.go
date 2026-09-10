package config

import (
	"testing"
	"time"
)

func TestActiveTTLDefaultIsNineHours(t *testing.T) {
	t.Setenv("ACTIVE_TTL", "")
	t.Setenv("REQUEST_TTL", "")
	cfg := FromEnv()
	if cfg.ActiveTTL != 9*time.Hour {
		t.Fatalf("ActiveTTL=%s", cfg.ActiveTTL)
	}
	if cfg.RequestTTL != time.Hour {
		t.Fatalf("RequestTTL=%s", cfg.RequestTTL)
	}
}
