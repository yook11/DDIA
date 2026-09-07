package main

import (
	"context"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestReadConfigRequiresPrimaryURL(t *testing.T) {
	for _, value := range []string{"", "   "} {
		_, err := readConfig(func(string) string { return value })
		if err == nil || !strings.Contains(err.Error(), "PRIMARY_DATABASE_URL is required") {
			t.Fatalf("unexpected error: %v", err)
		}
	}
}

func TestReadConfigUsesLoopbackDefaultAndAddressOverride(t *testing.T) {
	for _, address := range []string{"", "127.0.0.1:19080"} {
		env := map[string]string{"PRIMARY_DATABASE_URL": "configured URL", "HTTP_ADDR": address}
		cfg, err := readConfig(func(key string) string { return env[key] })
		if err != nil {
			t.Fatal(err)
		}
		want := address
		if want == "" {
			want = "127.0.0.1:18080"
		}
		if cfg.httpAddr != want || cfg.primaryURL != env["PRIMARY_DATABASE_URL"] {
			t.Fatalf("unexpected config: %+v", cfg)
		}
	}
}

func TestRunChecksDatabaseBeforeListening(t *testing.T) {
	env := map[string]string{
		"PRIMARY_DATABASE_URL": "postgres://test:private-password@localhost:bad-port/ddia",
		"HTTP_ADDR":            "invalid address",
	}
	err := run(context.Background(), func(key string) string { return env[key] })
	if err == nil || err.Error() != "invalid PRIMARY_DATABASE_URL" {
		t.Fatalf("unexpected startup error: %v", err)
	}
}

func TestServeStopsWhenContextIsCanceled(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := serve(ctx, &http.Server{Handler: http.NotFoundHandler()}, listener); err != nil {
		t.Fatal(err)
	}
	connection, err := net.DialTimeout("tcp", listener.Addr().String(), time.Second)
	if err == nil {
		connection.Close()
		t.Fatal("HTTP listener remained open")
	}
}

func TestServeLetsInFlightRequestFinishBeforeShutdown(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	started := make(chan struct{})
	release := make(chan struct{})
	shutdownStarted := make(chan struct{})
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		select {
		case <-release:
			_, _ = io.WriteString(w, "finished")
		case <-r.Context().Done():
		}
	})}
	defer server.Close()
	server.RegisterOnShutdown(func() { close(shutdownStarted) })
	serverDone := make(chan error, 1)
	go func() { serverDone <- serve(ctx, server, listener) }()
	requestDone := make(chan error, 1)
	go func() {
		client := &http.Client{Timeout: 3 * time.Second}
		response, err := client.Get("http://" + listener.Addr().String())
		if err == nil {
			_, err = io.Copy(io.Discard, response.Body)
			response.Body.Close()
		}
		requestDone <- err
	}()
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("request did not start")
	}
	cancel()
	select {
	case <-shutdownStarted:
	case <-time.After(3 * time.Second):
		t.Fatal("shutdown did not start")
	}
	select {
	case err := <-serverDone:
		t.Fatalf("shutdown returned before the request finished: %v", err)
	default:
	}
	close(release)
	if err := <-requestDone; err != nil {
		t.Fatal(err)
	}
	if err := <-serverDone; err != nil {
		t.Fatal(err)
	}
}
