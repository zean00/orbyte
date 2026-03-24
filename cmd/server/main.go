package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"orbyte/internal/modules"
	"orbyte/internal/platform/app"
	"orbyte/internal/platform/runtimeconfig"
)

func main() {
	runtime := runtimeconfig.Current()
	profile := runtime.DomainProfile()
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
		ReadTimeout:       runtime.HTTPReadTimeout(),
		WriteTimeout:      runtime.HTTPWriteTimeout(),
		IdleTimeout:       runtime.HTTPIdleTimeout(),
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	application.StartBackground(ctx)

	go func() {
		<-ctx.Done()
		application.PrepareShutdown()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Printf("server shutdown error: %v", err)
		}
	}()

	log.Printf("orbyte server listening on %s profile=%s business_modules=%v", application.Address(), application.Profile(), application.BusinessModuleKeys())
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Printf("server error: %v", err)
		os.Exit(1)
	}
	log.Printf("server stopped")
}

func durationFromEnv(key string, fallback time.Duration) time.Duration {
	return runtimeconfig.DurationFromEnv(key, fallback)
}
