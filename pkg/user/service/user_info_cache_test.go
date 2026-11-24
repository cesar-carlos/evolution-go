package user_service

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/types"
)

func testMemoService(now *time.Time) *userService {
	return &userService{
		now:           func() time.Time { return *now },
		queryCooldown: 45 * time.Second,
		queryLimit:    newInstanceQueryLimit(),
		userMemo:      newUserInfoMemo(),
	}
}

func userInfoConnect(client profileQueryClient, lids store.LIDStore, connects *atomic.Int32) func() (profileQueryClient, store.LIDStore, error) {
	return func() (profileQueryClient, store.LIDStore, error) {
		if connects != nil {
			connects.Add(1)
		}
		return client, lids, nil
	}
}

func TestUserInfoCacheHitSkipsIQAndIsolatesInstances(t *testing.T) {
	now := time.Date(2026, 11, 24, 12, 0, 0, 0, time.UTC)
	svc := testMemoService(&now)
	jid := types.NewJID("14155550100", types.DefaultUserServer)
	var infoCalls, pictureCalls atomic.Int32
	client := queryClientFake{info: func(context.Context, []types.JID) (map[types.JID]types.UserInfo, error) {
		infoCalls.Add(1)
		return map[types.JID]types.UserInfo{jid: {Status: "hello", PictureID: "photo"}}, nil
	}, picture: func(context.Context, types.JID, *whatsmeow.GetProfilePictureParams) (*types.ProfilePictureInfo, error) {
		pictureCalls.Add(1)
		return &types.ProfilePictureInfo{URL: "https://example.test/photo", ID: "photo"}, nil
	}}
	first, err := svc.userInfoFromClient(context.Background(), "one", []types.JID{jid}, userInfoConnect(client, nil, nil))
	if err != nil || first.Users[jid].PictureURL != "https://example.test/photo" || infoCalls.Load() != 1 {
		t.Fatalf("first=%+v err=%v calls=%d", first, err, infoCalls.Load())
	}
	second, err := svc.userInfoFromClient(context.Background(), "one", []types.JID{jid}, userInfoConnect(client, nil, nil))
	if err != nil || second.Users[jid].PictureURL != "https://example.test/photo" || infoCalls.Load() != 1 || pictureCalls.Load() != 1 {
		t.Fatalf("cache hit still queried WhatsApp: %+v err=%v info=%d pic=%d", second, err, infoCalls.Load(), pictureCalls.Load())
	}
	if _, err := svc.userInfoFromClient(context.Background(), "two", []types.JID{jid}, userInfoConnect(client, nil, nil)); err != nil || infoCalls.Load() != 2 {
		t.Fatalf("neighbor instance reused cache: err=%v calls=%d", err, infoCalls.Load())
	}
}

func TestUserInfoCacheExpiresPictureURLAndUSync(t *testing.T) {
	now := time.Date(2026, 11, 24, 12, 0, 0, 0, time.UTC)
	svc := testMemoService(&now)
	jid := types.NewJID("14155550100", types.DefaultUserServer)
	var infoCalls atomic.Int32
	client := queryClientFake{info: func(context.Context, []types.JID) (map[types.JID]types.UserInfo, error) {
		infoCalls.Add(1)
		return map[types.JID]types.UserInfo{jid: {Status: "hello", PictureID: "photo"}}, nil
	}, picture: func(context.Context, types.JID, *whatsmeow.GetProfilePictureParams) (*types.ProfilePictureInfo, error) {
		return &types.ProfilePictureInfo{URL: "https://example.test/photo", ID: "photo"}, nil
	}}
	if _, err := svc.userInfoFromClient(context.Background(), "one", []types.JID{jid}, userInfoConnect(client, nil, nil)); err != nil {
		t.Fatal(err)
	}
	now = now.Add(pictureURLCacheTTL + time.Second)
	cached := svc.lookupCachedUserInfo("one", []types.JID{jid})
	if cached == nil || cached.Users[jid].PictureURL != "" || cached.Users[jid].Status != "hello" {
		t.Fatalf("expired picture should stay as usync cache: %+v", cached)
	}
	now = now.Add(userInfoCacheTTL)
	if got := svc.lookupCachedUserInfo("one", []types.JID{jid}); got != nil {
		t.Fatalf("expired usync still cached: %+v", got)
	}
}

func TestUserInfoCooldownUsesCacheWithoutPictureOrIQ(t *testing.T) {
	now := time.Date(2026, 11, 24, 12, 0, 0, 0, time.UTC)
	svc := testMemoService(&now)
	jid := types.NewJID("14155550100", types.DefaultUserServer)
	var infoCalls atomic.Int32
	client := queryClientFake{info: func(context.Context, []types.JID) (map[types.JID]types.UserInfo, error) {
		infoCalls.Add(1)
		return map[types.JID]types.UserInfo{jid: {Status: "hello", PictureID: "photo"}}, nil
	}, picture: func(context.Context, types.JID, *whatsmeow.GetProfilePictureParams) (*types.ProfilePictureInfo, error) {
		return &types.ProfilePictureInfo{URL: "https://example.test/photo", ID: "photo"}, nil
	}}
	if _, err := svc.userInfoFromClient(context.Background(), "one", []types.JID{jid}, userInfoConnect(client, nil, nil)); err != nil {
		t.Fatal(err)
	}
	_ = svc.markAndWrapOverLimit("one")
	var connects atomic.Int32
	uc, err := svc.userInfoFromClient(context.Background(), "one", []types.JID{jid}, userInfoConnect(client, nil, &connects))
	if err != nil || uc.Users[jid].PictureURL != "" || uc.Users[jid].Status != "hello" || infoCalls.Load() != 1 || connects.Load() != 0 {
		t.Fatalf("cooldown cache=%+v err=%v info=%d connect=%d", uc, err, infoCalls.Load(), connects.Load())
	}
	missJID := types.NewJID("14155550101", types.DefaultUserServer)
	_, err = svc.userInfoFromClient(context.Background(), "one", []types.JID{missJID}, userInfoConnect(client, nil, &connects))
	if !errors.Is(err, whatsmeow.ErrIQRateOverLimit) || connects.Load() != 0 {
		t.Fatalf("cooldown miss should 429 without IQ: err=%v connect=%d", err, connects.Load())
	}
}

func TestUserInfoDoesNotCacheRateLimit(t *testing.T) {
	now := time.Date(2026, 11, 24, 12, 0, 0, 0, time.UTC)
	svc := testMemoService(&now)
	jid := types.NewJID("14155550100", types.DefaultUserServer)
	client := queryClientFake{info: func(context.Context, []types.JID) (map[types.JID]types.UserInfo, error) {
		return nil, whatsmeow.ErrIQRateOverLimit
	}, picture: func(context.Context, types.JID, *whatsmeow.GetProfilePictureParams) (*types.ProfilePictureInfo, error) {
		t.Fatal("picture IQ after usync overlimit")
		return nil, nil
	}}
	_, err := svc.userInfoFromClient(context.Background(), "one", []types.JID{jid}, userInfoConnect(client, nil, nil))
	if !errors.Is(err, whatsmeow.ErrIQRateOverLimit) {
		t.Fatalf("err=%v", err)
	}
	if got := svc.lookupCachedUserInfo("one", []types.JID{jid}); got != nil {
		t.Fatalf("cached overlimit: %+v", got)
	}
}

func TestUserInfoCoalescesInflightQueries(t *testing.T) {
	now := time.Date(2026, 11, 24, 12, 0, 0, 0, time.UTC)
	svc := testMemoService(&now)
	jid := types.NewJID("14155550100", types.DefaultUserServer)
	release := make(chan struct{})
	started := make(chan struct{})
	var infoCalls atomic.Int32
	client := queryClientFake{info: func(context.Context, []types.JID) (map[types.JID]types.UserInfo, error) {
		if infoCalls.Add(1) == 1 {
			close(started)
			<-release
		}
		return map[types.JID]types.UserInfo{jid: {Status: "hello"}}, nil
	}, picture: func(context.Context, types.JID, *whatsmeow.GetProfilePictureParams) (*types.ProfilePictureInfo, error) {
		return nil, nil
	}}
	connect := userInfoConnect(client, nil, nil)
	errCh := make(chan error, 2)
	go func() {
		_, err := svc.userInfoFromClient(context.Background(), "one", []types.JID{jid}, connect)
		errCh <- err
	}()
	<-started
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, err := svc.userInfoFromClient(context.Background(), "one", []types.JID{jid}, connect)
		errCh <- err
	}()
	key := userInfoInflightKey("one", []types.JID{jid})
	svc.userMemo.mu.Lock()
	call := svc.userMemo.inflight[key]
	svc.userMemo.mu.Unlock()
	if call == nil {
		t.Fatal("missing inflight call")
	}
	<-call.joined
	close(release)
	wg.Wait()
	if err := <-errCh; err != nil {
		t.Fatal(err)
	}
	if err := <-errCh; err != nil {
		t.Fatal(err)
	}
	if infoCalls.Load() != 1 {
		t.Fatalf("coalesced IQ count = %d", infoCalls.Load())
	}
}
