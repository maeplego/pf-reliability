package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/portfolio/pf-reliability/apps/api/internal/auth"
	"github.com/portfolio/pf-reliability/apps/api/internal/clock"
	"github.com/portfolio/pf-reliability/apps/api/internal/config"
	"github.com/portfolio/pf-reliability/apps/api/internal/incident"
	"github.com/portfolio/pf-reliability/apps/api/internal/seed"
	"github.com/portfolio/pf-reliability/apps/api/internal/store/memory"
	"github.com/portfolio/pf-reliability/apps/api/internal/web"
)

func main() {
	cfg, err := config.FromEnv()
	if err != nil {
		log.Fatal(err)
	}
	ctx := context.Background()
	clk := clock.System{}
	st := memory.New()
	svc := incident.NewService(st, clk.Now)
	if err := seed.Ensure(ctx, svc, cfg.IntegrationKey, cfg.WebhookSecret); err != nil {
		log.Fatal(err)
	}

	mw := auth.New(cfg.DevAuth)
	handler := web.New(svc, cfg.CORSOrigin, mw, func() error { return st.Ping(context.Background()) }, cfg.IntegrationKey).Routes()
	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("reliability api listening on %s (devAuth=%v integration=%s store=memory)", cfg.HTTPAddr, cfg.DevAuth, cfg.IntegrationKey)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	log.Println("shutting down")
	shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutCtx)
}
