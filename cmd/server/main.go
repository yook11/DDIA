package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"ddia/app/httpapi"
	"ddia/app/post"
	"ddia/app/thread"
	"ddia/app/user"
	"ddia/postgres"
)

type config struct {
	primaryURL string
	httpAddr   string
}

func readConfig(getenv func(string) string) (config, error) {
	cfg := config{
		primaryURL: strings.TrimSpace(getenv("PRIMARY_DATABASE_URL")),
		httpAddr:   strings.TrimSpace(getenv("HTTP_ADDR")),
	}
	if cfg.primaryURL == "" {
		return config{}, errors.New("PRIMARY_DATABASE_URL is required")
	}
	if cfg.httpAddr == "" {
		cfg.httpAddr = "127.0.0.1:18080"
	}
	return cfg, nil
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Getenv); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}

func run(ctx context.Context, getenv func(string) string) error {
	cfg, err := readConfig(getenv)
	if err != nil {
		return err
	}

	startupCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	primary, err := postgres.OpenPrimary(startupCtx, cfg.primaryURL)
	cancel()
	if err != nil {
		return err
	}
	defer primary.Close()

	users := user.NewService(user.NewRepository(primary))
	threads := thread.NewService(thread.NewRepository(primary))
	posts := post.NewService(post.NewRepository(primary))
	server := &http.Server{
		Handler: httpapi.NewHandler(httpapi.Services{
			Users:   users,
			Threads: threads,
			Posts:   posts,
		}),
		ReadHeaderTimeout: 5 * time.Second,
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	listener, err := net.Listen("tcp", cfg.httpAddr)
	if err != nil {
		return fmt.Errorf("HTTP listen failed: %w", err)
	}
	defer listener.Close()
	log.Printf("HTTP server listening on %s", listener.Addr())
	return serve(ctx, server, listener)
}

func serve(ctx context.Context, server *http.Server, listener net.Listener) error {
	done := make(chan error, 1)
	go func() { done <- server.Serve(listener) }()

	select {
	case err := <-done:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("HTTP server failed: %w", err)
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			_ = server.Close()
			return fmt.Errorf("HTTP shutdown failed: %w", err)
		}
		if err := <-done; err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("HTTP server failed: %w", err)
		}
		return nil
	}
}
