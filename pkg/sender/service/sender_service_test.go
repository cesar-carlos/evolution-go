package sender_service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/types"
)

type testLIDs struct {
	store.LIDStore
	phone string
	err   error
	calls int
}

func (l *testLIDs) GetPNForLID(ctx context.Context, jid types.JID) (types.JID, error) {
	l.calls++
	if err := ctx.Err(); err != nil {
		return types.EmptyJID, err
	}
	if l.err != nil {
		return types.EmptyJID, l.err
	}
	if jid.User != "12345" {
		return types.EmptyJID, nil
	}
	return types.NewJID(l.phone, types.DefaultUserServer), nil
}

type testClients map[string]*whatsmeow.Client

func (clients testClients) GetClient(id string) *whatsmeow.Client { return clients[id] }

func TestLIDStoreIsolationAndValidation(t *testing.T) {
	a, b := &testLIDs{phone: "5511111"}, &testLIDs{phone: "5522222"}
	s := NewService(testClients{
		"a": whatsmeow.NewClient(&store.Device{LIDs: a}, nil),
		"b": whatsmeow.NewClient(&store.Device{LIDs: b}, nil),
	})
	for _, item := range []struct{ id, phone string }{{"a", "5511111"}, {"b", "5522222"}} {
		result, err := s.ResolveLIDs(context.Background(), item.id, "12345,12345,99999")
		if err != nil || len(result) != 1 || result["12345"] != item.phone {
			t.Fatalf("%s: %v, %v", item.id, result, err)
		}
	}
	if a.calls != 2 || b.calls != 2 {
		t.Fatal("duplicates queried or wrong store accessed")
	}
	for _, raw := range []string{"", "123,a", "123@lid", strings.Repeat("1", 33), strings.Repeat("1,", 100) + "1"} {
		if _, err := s.ResolveLIDs(context.Background(), "a", raw); !errors.Is(err, ErrInvalidLIDs) {
			t.Fatalf("accepted %q: %v", raw, err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.ResolveLIDs(ctx, "a", "12345"); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if a.calls != 2 {
		t.Fatal("cancelled query reached store")
	}
	a.err = errors.New("store failed")
	if _, err := s.ResolveLIDs(context.Background(), "a", "12345"); err == nil {
		t.Fatal("store failure hidden")
	}
	if result, err := s.ResolveLIDs(context.Background(), "missing", "12345"); err != nil || len(result) != 0 {
		t.Fatalf("%v %v", result, err)
	}
}
