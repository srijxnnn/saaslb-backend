package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"saaslb-backend/internal/config"
	"saaslb-backend/internal/dodo"
	"saaslb-backend/internal/domain"
	"saaslb-backend/internal/httpapi"
	"saaslb-backend/internal/store"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := store.Open(ctx, cfg.MongoURI, cfg.MongoDB)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer db.Close()

	if err := db.Migrate(ctx); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	period := domain.CurrentPeriodKey(time.Now())
	if err := db.EnsurePeriod(ctx, period); err != nil {
		log.Fatalf("period: %v", err)
	}
	if err := db.RefreshEmptyTaglines(ctx); err != nil {
		log.Printf("tagline refresh: %v", err)
	}

	var payments *dodo.Client
	if cfg.PaymentsMode == "dodo" && cfg.DodoAPIKey != "" && cfg.DodoProductID != "" {
		payments = dodo.New(cfg.DodoBaseURL(), cfg.DodoAPIKey, cfg.DodoProductID)
	}

	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           httpapi.New(cfg, db, payments),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("saaslb api listening on %s (%s)", server.Addr, cfg.PaymentsMode)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	<-ctx.Done()
	shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = server.Shutdown(shutdown)
}
