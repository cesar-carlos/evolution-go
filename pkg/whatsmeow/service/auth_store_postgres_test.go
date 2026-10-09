package whatsmeow_service

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"sync"
	"testing"
	"time"
)

func TestAuthStorePostgresReusesBoundedPool(t *testing.T) {
	dsn := os.Getenv("AUTH_STORE_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("AUTH_STORE_TEST_POSTGRES_DSN requires an isolated PostgreSQL database")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	admin, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	schema := fmt.Sprintf("auth_store_%d", time.Now().UnixNano())
	if _, err := admin.ExecContext(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, err := admin.ExecContext(context.Background(), "DROP SCHEMA "+schema+" CASCADE"); err != nil {
			t.Error(err)
		}
	}()
	u, err := url.Parse(dsn)
	if err != nil || u.Scheme == "" {
		t.Fatal("test DSN must be a PostgreSQL URL")
	}
	q := u.Query()
	q.Set("search_path", schema)
	q.Set("application_name", schema)
	u.RawQuery = q.Encode()
	db, err := sql.Open("postgres", u.String())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(2)
	for range 20 {
		s := newAuthStore(ctx, db, u.String(), "", "")
		var wg sync.WaitGroup
		for range 32 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				c, err := s.get(ctx)
				if err != nil {
					t.Error(err)
					return
				}
				if _, err := c.GetAllDevices(ctx); err != nil {
					t.Error(err)
				}
			}()
		}
		wg.Wait()
		if s.ownedDB != nil {
			t.Fatal("borrowed PostgreSQL became owned")
		}
		if err := s.close(ctx); err != nil {
			t.Fatal(err)
		}
		if err := db.PingContext(ctx); err != nil {
			t.Fatal("shared pool was closed", err)
		}
		var count int
		if err := admin.QueryRowContext(ctx, "SELECT count(*) FROM pg_stat_activity WHERE application_name=$1", schema).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count > 4 || db.Stats().MaxOpenConnections != 4 {
			t.Fatalf("pool grew or limits changed: count=%d stats=%+v", count, db.Stats())
		}
	}
}
