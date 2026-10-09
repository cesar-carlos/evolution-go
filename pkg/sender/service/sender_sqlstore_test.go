package sender_service

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/lib/pq"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	_ "modernc.org/sqlite"
)

func TestLIDResolutionSQLStores(t *testing.T) {
	for _, dialect := range []string{"sqlite", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			dsn := "file:" + filepath.Join(t.TempDir(), "session.db") + "?_pragma=foreign_keys(1)"
			if dialect == "postgres" {
				dsn = os.Getenv("SENDER_TEST_POSTGRES_DSN")
				if dsn == "" {
					t.Skip("requires an isolated PostgreSQL database; CI provides one")
				}
			}
			ctx := context.Background()
			container, err := sqlstore.New(ctx, dialect, dsn, nil)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := container.Close(); err != nil {
					t.Error(err)
				}
			})
			lid, pn := types.NewJID("12345", types.HiddenUserServer), types.NewJID("5511999999999", types.DefaultUserServer)
			if err := container.LIDMap.PutLIDMapping(ctx, lid, pn); err != nil {
				t.Fatal(err)
			}
			client := whatsmeow.NewClient(&store.Device{LIDs: container.LIDMap}, nil)
			s := NewService(testClients{"session": client})
			result, err := s.ResolveLIDs(ctx, "session", "12345,99999")
			if err != nil || result["12345"] != pn.User || len(result) != 1 {
				t.Fatalf("%v %v", result, err)
			}
		})
	}
}
