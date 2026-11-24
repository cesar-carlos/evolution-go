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

func TestUserInfoCacheKeepsResponseJIDForLIDLookup(t *testing.T) {
	now := time.Date(2026, 11, 24, 12, 0, 0, 0, time.UTC)
	svc := testMemoService(&now)
	pn := types.NewJID("14155550100", types.DefaultUserServer)
	lid := types.NewJID("123", types.HiddenUserServer)
	lids := queryLIDsFake{pn: func(context.Context, types.JID) (types.JID, error) {
		return pn, nil
	}, lid: func(context.Context, types.JID) (types.JID, error) {
		return types.EmptyJID, errors.New("unused")
	}}
	var infoCalls atomic.Int32
	client := queryClientFake{info: func(context.Context, []types.JID) (map[types.JID]types.UserInfo, error) {
		infoCalls.Add(1)
		return map[types.JID]types.UserInfo{pn: {Status: "hello", LID: lid}}, nil
	}, picture: func(context.Context, types.JID, *whatsmeow.GetProfilePictureParams) (*types.ProfilePictureInfo, error) {
		return nil, nil
	}}
	first, err := svc.userInfoFromClient(context.Background(), "one", []types.JID{lid}, userInfoConnect(client, lids, nil))
	if err != nil || first.Users[pn].Status != "hello" || len(first.Users) != 1 {
		t.Fatalf("live=%+v err=%v", first, err)
	}
	cached, err := svc.userInfoFromClient(context.Background(), "one", []types.JID{lid}, userInfoConnect(client, lids, nil))
	if err != nil || infoCalls.Load() != 1 {
		t.Fatalf("cache err=%v calls=%d", err, infoCalls.Load())
	}
	if _, ok := cached.Users[pn]; !ok {
		t.Fatalf("cached users keyed by request LID: %+v", cached.Users)
	}
	if _, ok := cached.Users[lid]; ok {
		t.Fatalf("cached response rekeyed to LID: %+v", cached.Users)
	}
}

func TestUserInfoCacheHitsMultipleLIDs(t *testing.T) {
	now := time.Date(2026, 11, 24, 12, 0, 0, 0, time.UTC)
	svc := testMemoService(&now)
	firstPN := types.NewJID("14155550100", types.DefaultUserServer)
	secondPN := types.NewJID("14155550101", types.DefaultUserServer)
	firstLID := types.NewJID("111", types.HiddenUserServer)
	secondLID := types.NewJID("222", types.HiddenUserServer)
	var infoCalls atomic.Int32
	client := queryClientFake{info: func(context.Context, []types.JID) (map[types.JID]types.UserInfo, error) {
		infoCalls.Add(1)
		return map[types.JID]types.UserInfo{
			firstPN:  {Status: "one", LID: firstLID},
			secondPN: {Status: "two", LID: secondLID},
		}, nil
	}, picture: func(context.Context, types.JID, *whatsmeow.GetProfilePictureParams) (*types.ProfilePictureInfo, error) {
		return nil, nil
	}}
	requested := []types.JID{firstLID, secondLID}
	if _, err := svc.userInfoFromClient(context.Background(), "one", requested, userInfoConnect(client, nil, nil)); err != nil {
		t.Fatal(err)
	}
	cached, err := svc.userInfoFromClient(context.Background(), "one", requested, userInfoConnect(client, nil, nil))
	if err != nil || infoCalls.Load() != 1 {
		t.Fatalf("multi LID cache err=%v calls=%d", err, infoCalls.Load())
	}
	if cached.Users[firstPN].Status != "one" || cached.Users[secondPN].Status != "two" {
		t.Fatalf("canonical keys missing: %+v", cached.Users)
	}
}

func TestUserInfoCoalesceWaiterSurvivesOwnerCancel(t *testing.T) {
	now := time.Date(2026, 11, 24, 12, 0, 0, 0, time.UTC)
	svc := testMemoService(&now)
	jid := types.NewJID("14155550100", types.DefaultUserServer)
	release := make(chan struct{})
	started := make(chan struct{})
	var infoCalls atomic.Int32
	client := queryClientFake{info: func(ctx context.Context, _ []types.JID) (map[types.JID]types.UserInfo, error) {
		if infoCalls.Add(1) == 1 {
			close(started)
			<-release
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		return map[types.JID]types.UserInfo{jid: {Status: "hello"}}, nil
	}, picture: func(context.Context, types.JID, *whatsmeow.GetProfilePictureParams) (*types.ProfilePictureInfo, error) {
		return nil, nil
	}}
	connect := userInfoConnect(client, nil, nil)
	ownerCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ownerErr := make(chan error, 1)
	go func() {
		_, err := svc.userInfoFromClient(ownerCtx, "one", []types.JID{jid}, connect)
		ownerErr <- err
	}()
	<-started
	waiterErr := make(chan error, 1)
	go func() {
		uc, err := svc.userInfoFromClient(context.Background(), "one", []types.JID{jid}, connect)
		if err == nil && (uc == nil || uc.Users[jid].Status != "hello") {
			waiterErr <- errors.New("waiter missing usync")
			return
		}
		waiterErr <- err
	}()
	key := userInfoInflightKey("one", []types.JID{jid})
	deadline := time.Now().Add(2 * time.Second)
	for {
		svc.userMemo.mu.Lock()
		call := svc.userMemo.inflight[key]
		svc.userMemo.mu.Unlock()
		if call != nil {
			select {
			case <-call.joined:
			case <-time.After(2 * time.Second):
				t.Fatal("waiter did not join")
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("missing inflight call")
		}
		time.Sleep(5 * time.Millisecond)
	}
	cancel()
	if err := <-ownerErr; !errors.Is(err, context.Canceled) {
		t.Fatalf("owner err=%v", err)
	}
	close(release)
	if err := <-waiterErr; err != nil {
		t.Fatalf("waiter inherited owner cancel: %v", err)
	}
	if infoCalls.Load() != 1 {
		t.Fatalf("coalesced IQ count = %d", infoCalls.Load())
	}
}

func TestUserInfoPictureOverLimitArmsCooldown(t *testing.T) {
	now := time.Date(2026, 11, 24, 12, 0, 0, 0, time.UTC)
	svc := testMemoService(&now)
	jid := types.NewJID("14155550100", types.DefaultUserServer)
	miss := types.NewJID("14155550101", types.DefaultUserServer)
	var infoCalls, pictureCalls, connects atomic.Int32
	client := queryClientFake{info: func(context.Context, []types.JID) (map[types.JID]types.UserInfo, error) {
		infoCalls.Add(1)
		return map[types.JID]types.UserInfo{jid: {Status: "hello", PictureID: "photo"}}, nil
	}, picture: func(context.Context, types.JID, *whatsmeow.GetProfilePictureParams) (*types.ProfilePictureInfo, error) {
		pictureCalls.Add(1)
		return nil, whatsmeow.ErrIQRateOverLimit
	}}
	uc, err := svc.userInfoFromClient(context.Background(), "one", []types.JID{jid}, userInfoConnect(client, nil, &connects))
	if err != nil || uc.Users[jid].Status != "hello" || uc.Users[jid].PictureURL != "" {
		t.Fatalf("picture overlimit should keep usync: %+v err=%v", uc, err)
	}
	if err := svc.errIfQueryCoolingDown("one"); !errors.Is(err, whatsmeow.ErrIQRateOverLimit) {
		t.Fatalf("cooldown not armed: %v", err)
	}
	cached, err := svc.userInfoFromClient(context.Background(), "one", []types.JID{jid}, userInfoConnect(client, nil, &connects))
	if err != nil || cached.Users[jid].Status != "hello" || infoCalls.Load() != 1 || pictureCalls.Load() != 1 {
		t.Fatalf("cached after picture overlimit: %+v err=%v info=%d pic=%d", cached, err, infoCalls.Load(), pictureCalls.Load())
	}
	_, err = svc.userInfoFromClient(context.Background(), "one", []types.JID{miss}, userInfoConnect(client, nil, &connects))
	if !errors.Is(err, whatsmeow.ErrIQRateOverLimit) || connects.Load() != 1 {
		t.Fatalf("later miss should 429 without IQ: err=%v connect=%d", err, connects.Load())
	}
}
