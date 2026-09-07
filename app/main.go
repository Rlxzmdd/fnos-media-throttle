package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/fnos-media-throttle/fnos-media-throttle/api"
	"github.com/fnos-media-throttle/fnos-media-throttle/service"
	"github.com/fnos-media-throttle/fnos-media-throttle/storage"
)

func main() {
	if err := run(); err != nil {
		log.Printf("fnos-media-throttle stopped with error: %v", err)
		os.Exit(1)
	}
}

func run() (runErr error) {
	dataDirectory := firstEnvironment("DATA_DIR", "TRIM_PKGVAR")
	if dataDirectory == "" {
		dataDirectory = "../db/runtime"
	}
	if err := os.MkdirAll(dataDirectory, 0700); err != nil {
		return fmt.Errorf("create data directory: %w", err)
	}
	store, err := storage.Open(filepath.Join(dataDirectory, "throttle.db"))
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			runErr = errors.Join(runErr, fmt.Errorf("close store: %w", err))
		}
	}()

	engine := service.NewEngine(store)
	engineContext, stopEngine := context.WithCancel(context.Background())
	engineDone := make(chan struct{})
	go func() {
		defer close(engineDone)
		engine.Run(engineContext)
	}()

	port := firstEnvironment("PORT", "TRIM_SERVICE_PORT")
	if port == "" {
		port = "18788"
	}
	bindAddress := os.Getenv("BIND_ADDR")
	if bindAddress == "" {
		bindAddress = "0.0.0.0"
	}
	server := &http.Server{
		Addr:              bindAddress + ":" + port,
		Handler:           api.New(store, engine),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	serverErrors := make(chan error, 1)
	go func() {
		serverErrors <- server.ListenAndServe()
	}()

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)

	log.Printf("fnos-media-throttle listening on %s", server.Addr)
	var serveErr error
	select {
	case received := <-signals:
		log.Printf("received %s, starting graceful shutdown", received)
	case err := <-serverErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			serveErr = fmt.Errorf("http server: %w", err)
			log.Printf("http server stopped unexpectedly: %v", err)
		}
	}

	// Stop the polling engine first and wait for the active tick to finish. This
	// prevents a late polling write from racing with the recovery below.
	stopEngine()
	engine.StopCommands()

	shutdownContext, cancelShutdown := context.WithTimeout(context.Background(), 5*time.Second)
	shutdownErr := server.Shutdown(shutdownContext)
	cancelShutdown()
	if shutdownErr != nil {
		log.Printf("graceful HTTP shutdown failed: %v", shutdownErr)
		_ = server.Close()
	}

	<-engineDone
	log.Printf("polling engine stopped")

	restoreContext, cancelRestore := context.WithTimeout(context.Background(), 20*time.Second)
	restoreErr := engine.Shutdown(restoreContext, 3*time.Second)
	cancelRestore()
	if restoreErr != nil {
		log.Printf("best-effort downloader recovery completed with errors: %v", restoreErr)
	} else {
		log.Printf("best-effort downloader recovery completed")
	}

	return errors.Join(serveErr, shutdownErr, restoreErr)
}

func firstEnvironment(names ...string) string {
	for _, name := range names {
		if value := os.Getenv(name); value != "" {
			return value
		}
	}
	return ""
}
