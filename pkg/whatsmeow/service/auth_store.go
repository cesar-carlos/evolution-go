package whatsmeow_service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"time"

	"go.mau.fi/whatsmeow/store/sqlstore"
	waLog "go.mau.fi/whatsmeow/util/log"
	_ "modernc.org/sqlite"
)

const authStoreInitTimeout = 30 * time.Second

var errAuthStoreClosed = errors.New("authentication store is shutting down")

type authStoreAttempt struct {
	done      chan struct{}
	container *sqlstore.Container
	err       error
}

// authStore owns SQLite sessions, but only borrows the PostgreSQL pool supplied
// by main. A failed initialization is retryable; only success is retained.
type authStore struct {
	mu          sync.Mutex
	ctx         context.Context
	cancel      context.CancelFunc
	closed      bool
	attempt     *authStoreAttempt
	container   *sqlstore.Container
	ownedDB     *sql.DB
	initTimeout time.Duration
	initialize  func(context.Context) (*sqlstore.Container, *sql.DB, error)
}

func newAuthStore(parent context.Context, authDB *sql.DB, postgresDSN, exPath, debug string) *authStore {
	ctx, cancel := context.WithCancel(parent)
	s := &authStore{ctx: ctx, cancel: cancel, initTimeout: authStoreInitTimeout}
	s.initialize = func(ctx context.Context) (*sqlstore.Container, *sql.DB, error) {
		var log waLog.Logger
		if debug != "" {
			log = waLog.Stdout("Database", debug, true)
		}
		db, dialect := authDB, "postgres"
		var owned *sql.DB
		if postgresDSN != "" {
			if db == nil {
				return nil, nil, errors.New("PostgreSQL auth database handle is missing")
			}
		} else {
			dialect = "sqlite"
			dsn := fmt.Sprintf("file:%s/dbdata/main.db?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)", exPath)
			var err error
			db, err = sql.Open(dialect, dsn)
			if err != nil {
				return nil, nil, fmt.Errorf("open SQLite auth store: %w", err)
			}
			// Sessions use main.db, not the application's users.db. A single
			// connection serializes SQLite writes and keeps PRAGMAs consistent.
			db.SetMaxOpenConns(1)
			owned = db
		}
		container := sqlstore.NewWithDB(db, dialect, log)
		if err := container.Upgrade(ctx); err != nil {
			if owned != nil {
				_ = owned.Close() // Preserve the initialization error.
			}
			return nil, nil, fmt.Errorf("upgrade auth store: %w", err)
		}
		return container, owned, nil
	}
	return s
}

func (s *authStore) get(ctx context.Context) (*sqlstore.Container, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	if s.closed || s.ctx.Err() != nil {
		s.mu.Unlock()
		return nil, errAuthStoreClosed
	}
	if s.container != nil {
		container := s.container
		s.mu.Unlock()
		return container, nil
	}
	attempt := s.attempt
	if attempt == nil {
		attempt = &authStoreAttempt{done: make(chan struct{})}
		s.attempt = attempt
		go s.runInitialization(attempt)
	}
	s.mu.Unlock()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-s.ctx.Done():
		return nil, errAuthStoreClosed
	case <-attempt.done:
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		return attempt.container, attempt.err
	}
}

func (s *authStore) runInitialization(attempt *authStoreAttempt) {
	ctx, cancel := context.WithTimeout(s.ctx, s.initTimeout)
	container, owned, err := s.initialize(ctx)
	cancel()
	s.mu.Lock()
	if s.closed || s.ctx.Err() != nil {
		if owned != nil {
			s.mu.Unlock()
			_ = owned.Close() // Shutdown has precedence over publishing a store.
			s.mu.Lock()
		}
		container, owned, err = nil, nil, errAuthStoreClosed
	}
	if err == nil {
		s.container, s.ownedDB = container, owned
	}
	attempt.container, attempt.err = container, err
	s.attempt = nil
	close(attempt.done)
	s.mu.Unlock()
}

func (s *authStore) close(ctx context.Context) error {
	s.mu.Lock()
	s.closed = true
	s.cancel()
	attempt := s.attempt
	s.mu.Unlock()
	if attempt != nil {
		select {
		case <-attempt.done:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	s.mu.Lock()
	db := s.ownedDB
	s.ownedDB = nil
	s.mu.Unlock()
	if db != nil {
		return db.Close()
	}
	return nil
}

// Shutdown releases the owned authentication store. The PostgreSQL pool is
// borrowed from main and is closed there after service shutdown.
func (w whatsmeowService) Shutdown(ctx context.Context) error {
	if err := w.sessions.shutdown(ctx); err != nil {
		return err
	}
	return w.authStore.close(ctx)
}
