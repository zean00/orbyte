package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"clinic/internal/platform/app"
)

func main() {
	application, err := app.New()
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

	log.Printf("core platform server listening on %s", application.Address())
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Printf("server error: %v", err)
		os.Exit(1)
	}
	log.Printf("server stopped")
}
