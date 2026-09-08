package worker

import (
	"context"
	"log"
	"time"

	"github.com/erreyesarroyo-cloud/keycloak-jit-access/internal/api"
)

func Run(ctx context.Context, svc *api.Service, every time.Duration) {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := svc.Reconcile(ctx); err != nil {
				log.Printf("reconcile: %v", err)
			}
		}
	}
}
