package whatsmeow_service

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go.mau.fi/whatsmeow/proto/waAdv"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
)

func testAuthStore(t *testing.T) (*authStore, string) {
	t.Helper()
	dir := t.TempDir()
	s := newAuthStore(context.Background(), nil, "", dir, "")
	t.Cleanup(func() {
		if err := s.close(context.Background()); err != nil {
			t.Error(err)
		}
	})
	return s, dir
}

func TestAuthStoreRetryReuseAndSQLiteSessions(t *testing.T) {
	s, dir := testAuthStore(t)
	if _, err := s.get(context.Background()); err == nil {
		t.Fatal("missing directory must fail")
	}
	if err := os.Mkdir(filepath.Join(dir, "dbdata"), 0700); err != nil {
		t.Fatal(err)
	}
	c, err := s.get(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if next, err := s.get(context.Background()); err != nil || next != c {
		t.Fatal("successful store was not reused", err)
	}
	device := c.NewDevice()
	jid := types.NewJID("5511999999999", types.DefaultUserServer)
	device.ID = &jid
	device.Account = &waAdv.ADVSignedDeviceIdentity{Details: []byte{}, AccountSignature: make([]byte, 64), AccountSignatureKey: make([]byte, 32), DeviceSignature: make([]byte, 64)}
	if err := device.Save(context.Background()); err != nil {
		t.Fatal(err)
	}
	loaded, err := c.GetDevice(context.Background(), jid)
	if err != nil || loaded == nil || *loaded.ID != jid {
		t.Fatal("persisted session missing", err)
	}
	if s.ownedDB.Stats().MaxOpenConnections != 1 {
		t.Fatal("SQLite pool must be single-connection")
	}
	var mode string
	if err := s.ownedDB.QueryRow("PRAGMA journal_mode").Scan(&mode); err != nil || mode != "wal" {
		t.Fatal("WAL not enabled", mode, err)
	}
	if err := s.close(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := s.get(context.Background()); !errors.Is(err, errAuthStoreClosed) {
		t.Fatal("closed store reopened", err)
	}
}

func TestAuthStoreConcurrentInitializationAndIndependentCancellation(t *testing.T) {
	s, _ := testAuthStore(t)
	entered, release := make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	expected := &sqlstore.Container{}
	s.initialize = func(ctx context.Context) (*sqlstore.Container, *sql.DB, error) {
		calls.Add(1)
		close(entered)
		select {
		case <-release:
			return expected, nil, nil
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() { _, err := s.get(ctx); result <- err }()
	<-entered
	cancel()
	if err := <-result; !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for range 32 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c, err := s.get(context.Background())
			if err != nil || c != expected {
				t.Error("shared initialization failed", err)
			}
		}()
	}
	close(release)
	wg.Wait()
	if calls.Load() != 1 {
		t.Fatal("duplicate initialization", calls.Load())
	}
}

func TestAuthStoreShutdownCancelsInitialization(t *testing.T) {
	s, _ := testAuthStore(t)
	entered := make(chan struct{})
	s.initialize = func(ctx context.Context) (*sqlstore.Container, *sql.DB, error) {
		deadline, ok := ctx.Deadline()
		if !ok || time.Until(deadline) > authStoreInitTimeout {
			t.Error("initialization has no bounded deadline")
		}
		close(entered)
		<-ctx.Done()
		return nil, nil, ctx.Err()
	}
	done := make(chan error, 1)
	go func() { _, err := s.get(context.Background()); done <- err }()
	<-entered
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := s.close(ctx); err != nil {
		t.Fatal(err)
	}
	if err := <-done; !errors.Is(err, errAuthStoreClosed) {
		t.Fatal(err)
	}
}

func TestAuthStoreRequiresBorrowedPostgresPool(t *testing.T) {
	missing := newAuthStore(context.Background(), nil, "configured", "", "")
	defer missing.close(context.Background())
	if _, err := missing.get(context.Background()); err == nil {
		t.Fatal("missing PostgreSQL handle accepted")
	}
}

func TestAuthStoreInitializationDeadlineIsRetryable(t *testing.T) {
	s, _ := testAuthStore(t)
	s.initTimeout = time.Millisecond
	s.initialize = func(ctx context.Context) (*sqlstore.Container, *sql.DB, error) {
		<-ctx.Done()
		return nil, nil, ctx.Err()
	}
	if _, err := s.get(context.Background()); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal(err)
	}
	expected := &sqlstore.Container{}
	s.initialize = func(context.Context) (*sqlstore.Container, *sql.DB, error) { return expected, nil, nil }
	if c, err := s.get(context.Background()); err != nil || c != expected {
		t.Fatal("timeout poisoned store", err)
	}
}
