package user_service

import (
	"context"
	"errors"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/evolution-foundation/evolution-go/pkg/utils"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/types"
)

const (
	userInfoCacheTTL        = 2 * time.Minute
	pictureURLCacheTTL      = 60 * time.Second
	maxUserInfoCacheEntries = 1024
)

type cachedUserEntry struct {
	info        UserInfo
	responseJID types.JID
	usyncAt     time.Time
	pictureAt   time.Time
}

type inflightUserInfo struct {
	done   chan struct{}
	joined chan struct{}
	uc     *UserCollection
	err    error
}

type userInfoMemo struct {
	mu       sync.Mutex
	entries  map[string]cachedUserEntry
	inflight map[string]*inflightUserInfo
}

func newUserInfoMemo() *userInfoMemo {
	return &userInfoMemo{
		entries:  make(map[string]cachedUserEntry),
		inflight: make(map[string]*inflightUserInfo),
	}
}

func userInfoCacheKey(instanceID string, jid types.JID) string {
	return instanceID + "\x00" + jid.String()
}

func userInfoInflightKey(instanceID string, jids []types.JID) string {
	parts := make([]string, len(jids))
	for i, jid := range jids {
		parts[i] = jid.String()
	}
	sort.Strings(parts)
	return instanceID + "\x00" + strings.Join(parts, "\x00")
}

func cloneUserInfo(info UserInfo) UserInfo {
	cloned := info
	if info.LID != nil {
		lid := *info.LID
		cloned.LID = &lid
	}
	if len(info.Devices) > 0 {
		cloned.Devices = append([]types.JID(nil), info.Devices...)
	}
	return cloned
}

func cloneUserCollection(uc *UserCollection) *UserCollection {
	if uc == nil {
		return nil
	}
	cloned := &UserCollection{Users: make(map[types.JID]UserInfo, len(uc.Users))}
	for jid, info := range uc.Users {
		cloned.Users[jid] = cloneUserInfo(info)
	}
	return cloned
}

func (u *userService) lookupCachedUserInfo(instanceID string, jids []types.JID) *UserCollection {
	if u == nil || u.userMemo == nil || instanceID == "" || len(jids) == 0 {
		return nil
	}
	now := u.currentTime()
	cooling := u.errIfQueryCoolingDown(instanceID) != nil
	u.userMemo.mu.Lock()
	defer u.userMemo.mu.Unlock()
	users := make(map[types.JID]UserInfo, len(jids))
	for _, jid := range jids {
		entry, ok := u.userMemo.entries[userInfoCacheKey(instanceID, jid)]
		if !ok || now.Sub(entry.usyncAt) > userInfoCacheTTL {
			return nil
		}
		info := cloneUserInfo(entry.info)
		if cooling || entry.pictureAt.IsZero() || now.Sub(entry.pictureAt) > pictureURLCacheTTL {
			info.PictureURL = ""
		}
		key := entry.responseJID
		if key.IsEmpty() {
			key = jid
		}
		users[key] = info
	}
	return &UserCollection{Users: users}
}

func (u *userService) storeCachedUserInfo(instanceID string, requested []types.JID, uc *UserCollection) {
	if u == nil || u.userMemo == nil || instanceID == "" || uc == nil {
		return
	}
	now := u.currentTime()
	u.userMemo.mu.Lock()
	defer u.userMemo.mu.Unlock()
	if u.userMemo.entries == nil {
		u.userMemo.entries = make(map[string]cachedUserEntry)
	}
	for jid, info := range uc.Users {
		u.putCachedUserLocked(instanceID, jid, jid, info, now)
		if info.LID == nil {
			continue
		}
		if lid, ok := utils.ParseJID(*info.LID); ok {
			lid = utils.CanonicalJID(lid).ToNonAD()
			u.putCachedUserLocked(instanceID, lid, jid, info, now)
		}
	}
	for _, requested := range requested {
		if requested.IsEmpty() {
			continue
		}
		if _, exists := u.userMemo.entries[userInfoCacheKey(instanceID, requested)]; exists {
			continue
		}
		responseJID, info, ok := matchRequestedUser(uc, requested)
		if !ok {
			continue
		}
		u.putCachedUserLocked(instanceID, requested, responseJID, info, now)
	}
	u.userMemo.evictUserInfoLocked(now)
}

func matchRequestedUser(uc *UserCollection, requested types.JID) (types.JID, UserInfo, bool) {
	if uc == nil {
		return types.EmptyJID, UserInfo{}, false
	}
	if info, ok := uc.Users[requested]; ok {
		return requested, info, true
	}
	if len(uc.Users) != 1 {
		return types.EmptyJID, UserInfo{}, false
	}
	for jid, info := range uc.Users {
		return jid, info, true
	}
	return types.EmptyJID, UserInfo{}, false
}

func (u *userService) putCachedUserLocked(instanceID string, lookup, response types.JID, info UserInfo, now time.Time) {
	if lookup.IsEmpty() || response.IsEmpty() {
		return
	}
	entry := cachedUserEntry{info: cloneUserInfo(info), responseJID: response, usyncAt: now}
	if info.PictureURL != "" {
		entry.pictureAt = now
	}
	u.userMemo.entries[userInfoCacheKey(instanceID, lookup)] = entry
}

func (m *userInfoMemo) evictUserInfoLocked(now time.Time) {
	for key, entry := range m.entries {
		if now.Sub(entry.usyncAt) > userInfoCacheTTL {
			delete(m.entries, key)
		}
	}
	for len(m.entries) >= maxUserInfoCacheEntries {
		var oldestKey string
		var oldest time.Time
		for key, entry := range m.entries {
			if oldestKey == "" || entry.usyncAt.Before(oldest) {
				oldestKey = key
				oldest = entry.usyncAt
			}
		}
		if oldestKey == "" {
			return
		}
		delete(m.entries, oldestKey)
	}
}

func (u *userService) userInfoFromClient(ctx context.Context, instanceID string, jids []types.JID, connect func() (profileQueryClient, store.LIDStore, error)) (*UserCollection, error) {
	if cached := u.lookupCachedUserInfo(instanceID, jids); cached != nil {
		return cached, nil
	}
	if err := u.errIfQueryCoolingDown(instanceID); err != nil {
		return nil, err
	}
	if connect == nil {
		return nil, errors.New("client disconnected")
	}
	client, lids, err := connect()
	if err != nil {
		return nil, err
	}
	return u.coalescedQueryUserInfo(ctx, instanceID, client, lids, jids)
}

func detachQueryContext(ctx context.Context) (context.Context, context.CancelFunc) {
	ctx = queryContext(ctx)
	base := context.WithoutCancel(ctx)
	if deadline, ok := ctx.Deadline(); ok {
		return context.WithDeadline(base, deadline)
	}
	return context.WithTimeout(base, userInfoRequestTimeout+pictureURLEnrichBudget)
}

func waitUserInfoCall(ctx context.Context, call *inflightUserInfo) (*UserCollection, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-call.done:
		if call.err != nil {
			return nil, call.err
		}
		return cloneUserCollection(call.uc), nil
	}
}

func (u *userService) finishUserInfoQuery(instanceID string, jids []types.JID, uc *UserCollection, err error) (*UserCollection, error) {
	if errors.Is(err, whatsmeow.ErrIQRateOverLimit) {
		wrapped := u.markAndWrapOverLimit(instanceID)
		if uc != nil && len(uc.Users) > 0 {
			u.storeCachedUserInfo(instanceID, jids, uc)
			return uc, nil
		}
		return nil, wrapped
	}
	if err == nil {
		u.storeCachedUserInfo(instanceID, jids, uc)
	}
	return uc, err
}

func (u *userService) coalescedQueryUserInfo(ctx context.Context, instanceID string, client profileQueryClient, lids store.LIDStore, jids []types.JID) (*UserCollection, error) {
	if u == nil || u.userMemo == nil {
		uc, err := queryUserInfo(ctx, client, lids, jids)
		return u.finishUserInfoQuery(instanceID, jids, uc, err)
	}
	key := userInfoInflightKey(instanceID, jids)
	u.userMemo.mu.Lock()
	if u.userMemo.inflight == nil {
		u.userMemo.inflight = make(map[string]*inflightUserInfo)
	}
	if call, ok := u.userMemo.inflight[key]; ok {
		u.userMemo.mu.Unlock()
		select {
		case call.joined <- struct{}{}:
		default:
		}
		return waitUserInfoCall(ctx, call)
	}
	call := &inflightUserInfo{done: make(chan struct{}), joined: make(chan struct{}, 1)}
	u.userMemo.inflight[key] = call
	u.userMemo.mu.Unlock()

	queryCtx, stopQuery := detachQueryContext(ctx)
	go func() {
		defer stopQuery()
		uc, err := queryUserInfo(queryCtx, client, lids, jids)
		uc, err = u.finishUserInfoQuery(instanceID, jids, uc, err)
		call.uc = cloneUserCollection(uc)
		call.err = err
		close(call.done)
		u.userMemo.mu.Lock()
		delete(u.userMemo.inflight, key)
		u.userMemo.mu.Unlock()
	}()
	return waitUserInfoCall(ctx, call)
}
