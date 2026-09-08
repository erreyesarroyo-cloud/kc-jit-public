package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/erreyesarroyo-cloud/keycloak-jit-access/internal/api"
	"github.com/erreyesarroyo-cloud/keycloak-jit-access/internal/auth"
	"github.com/erreyesarroyo-cloud/keycloak-jit-access/internal/config"
	"github.com/erreyesarroyo-cloud/keycloak-jit-access/internal/keycloak"
	"github.com/erreyesarroyo-cloud/keycloak-jit-access/internal/notify"
	"github.com/erreyesarroyo-cloud/keycloak-jit-access/internal/store"
	"github.com/erreyesarroyo-cloud/keycloak-jit-access/internal/worker"
)

func main() {
	cfg := config.FromEnv()
	log.Printf("pim starting realm=%s kc=%s publicKC=%s requestTTL=%s activeTTL=%s oidc=%v ns-hint=keycloak-pim",
		cfg.Realm, cfg.BaseURL, cfg.BrowserIssuer(), cfg.RequestTTL, cfg.ActiveTTL, cfg.OIDCEnabled)

	db, err := store.Open(cfg.DBPath)
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer db.Close()

	kc := keycloak.New(cfg)
	am := auth.NewManager(cfg)
	n := notify.New(cfg.NotifyWebhookURL, cfg.PublicURL, db)
	svc := api.NewService(cfg, db, kc, am, n)
	log.Printf("notify webhook enabled=%v", n.Enabled())
	if err := svc.Reconcile(context.Background()); err != nil {
		log.Printf("startup reconcile warning: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go worker.Run(ctx, svc, cfg.PollInterval)

	srv := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           api.NewRouter(svc),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("listening on %s publicURL=%s", cfg.ListenAddr, cfg.PublicURL)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("http: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	cancel()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	_ = srv.Shutdown(shutdownCtx)
	log.Printf("pim stopped")
}
