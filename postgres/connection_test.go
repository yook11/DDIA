package postgres

import (
	"context"
	"net"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

func TestOpenPrimaryHidesConnectionStringOnConfigurationError(t *testing.T) {
	pool, err := OpenPrimary(context.Background(), "postgres://test:private-password@localhost:bad-port/ddia")
	if pool != nil || err == nil || strings.Contains(err.Error(), "private-password") {
		t.Fatalf("unexpected result: pool=%v err=%v", pool, err)
	}
}

func TestOpenPrimaryRejectsUnreachableDatabase(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	listener.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	pool, err := OpenPrimary(ctx, "postgres://test:private-password@"+address+"/ddia?sslmode=disable")
	if pool != nil || err == nil || strings.Contains(err.Error(), "private-password") {
		t.Fatalf("unexpected result: pool=%v err=%v", pool, err)
	}
}

func TestOpenPrimaryHonorsCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	pool, err := OpenPrimary(ctx, "postgres://test@127.0.0.1:1/ddia?sslmode=disable")
	if pool != nil || err == nil {
		t.Fatalf("unexpected result: pool=%v err=%v", pool, err)
	}
}

func TestOpenPrimaryDeadlineIncludesFailedConnectionCleanup(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	accepted := make(chan net.Conn, 1)
	go func() {
		connection, err := listener.Accept()
		if err == nil {
			accepted <- connection
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	start := time.Now()
	pool, err := OpenPrimary(ctx, "postgres://test@"+listener.Addr().String()+"/ddia?sslmode=disable")
	if pool != nil {
		pool.Close()
		t.Fatal("nonresponding server was accepted")
	}
	if err == nil || time.Since(start) > 2*time.Second {
		t.Fatalf("deadline not honored: elapsed=%v err=%v", time.Since(start), err)
	}
	select {
	case connection := <-accepted:
		connection.Close()
	case <-time.After(time.Second):
		t.Fatal("test connection was never accepted")
	}
}

func TestOpenPrimaryRejectsReplica(t *testing.T) {
	dsn := integrationDSN(t, "DDIA_TEST_REPLICA_DATABASE_URL")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	pool, err := OpenPrimary(ctx, dsn)
	if pool != nil {
		pool.Close()
		t.Fatal("replica was accepted")
	}
	if err == nil || !strings.Contains(err.Error(), "writable primary") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestOpenPrimaryRejectsReadOnlyPrimarySession(t *testing.T) {
	dsn := integrationDSN(t, "DDIA_TEST_PRIMARY_DATABASE_URL")
	u, err := url.Parse(dsn)
	if err != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") {
		t.Fatal("this test requires a PostgreSQL URL")
	}
	query := u.Query()
	query.Set("default_transaction_read_only", "on")
	u.RawQuery = query.Encode()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	pool, err := OpenPrimary(ctx, u.String())
	if pool != nil {
		pool.Close()
		t.Fatal("read-only primary session was accepted")
	}
	if err == nil || !strings.Contains(err.Error(), "writable primary") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func integrationDSN(t *testing.T, name string) string {
	t.Helper()
	dsn := os.Getenv(name)
	if dsn == "" {
		t.Skip(name + " is not set")
	}
	return dsn
}
