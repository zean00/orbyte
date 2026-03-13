package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"clinic/internal/modules"
	"clinic/internal/platform/app"
)

func main() {
	profile := os.Getenv("APP_DOMAIN_PROFILE")
	manifests, err := modules.ForProfile(profile)
	if err != nil {
		log.Printf("app startup error: %v", err)
		os.Exit(1)
	}
	application, err := app.New(app.Options{
		Profile:           profile,
		BusinessManifests: manifests,
	})
	if err != nil {
		log.Printf("app startup error: %v", err)
		os.Exit(1)
	}
	defer func() {
		if err := application.Close(); err != nil {
			log.Printf("app close error: %v", err)
		}
	}()
	server := &http.Server{
		Addr:              application.Address(),
		Handler:           application.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	application.StartBackground(ctx)

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Printf("server shutdown error: %v", err)
		}
	}()

	log.Printf("core platform server listening on %s profile=%s business_modules=%v", application.Address(), application.Profile(), application.BusinessModuleKeys())
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Printf("server error: %v", err)
		os.Exit(1)
	}
	log.Printf("server stopped")
}
