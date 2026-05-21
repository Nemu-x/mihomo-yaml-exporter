package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/nemu-x/mihomo-yaml-exporter/internal/config"
	"github.com/nemu-x/mihomo-yaml-exporter/internal/engine"
	"github.com/nemu-x/mihomo-yaml-exporter/internal/metrics"
	"github.com/nemu-x/mihomo-yaml-exporter/internal/server"
	"github.com/nemu-x/mihomo-yaml-exporter/internal/subscription"
)

func main() {
	log.SetFlags(log.LstdFlags | log.LUTC)

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	log.Printf("starting mihomo-yaml-exporter on %s (subscription=%s)", cfg.ListenAddr, subscription.RedactURL(cfg.SubscriptionURL))

	reg := metrics.NewRegistry()
	eng := engine.New(cfg, reg)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go eng.Run(ctx)

	srv := server.New(eng)
	httpSrv := &http.Server{
		Addr:    cfg.ListenAddr,
		Handler: srv.Handler(),
	}

	go func() {
		log.Printf("listening on %s", cfg.ListenAddr)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("http: %v", err)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Printf("shutting down")
	cancel()
	_ = httpSrv.Shutdown(context.Background())
}
