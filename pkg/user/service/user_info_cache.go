package user_service

import (
	"context"
	"errors"
	"sort"
	"strings"
	"sync"
	"time"

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
	info      UserInfo
	usyncAt   time.Time
	pictureAt time.Time
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
		users[jid] = info
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
		u.putCachedUserLocked(instanceID, jid, info, now)
	}
	if len(requested) == 1 && len(uc.Users) == 1 {
		for _, info := range uc.Users {
			u.putCachedUserLocked(instanceID, requested[0], info, now)
		}
	}
	u.userMemo.evictUserInfoLocked(now)
}

func (u *userService) putCachedUserLocked(instanceID string, jid types.JID, info UserInfo, now time.Time) {
	entry := cachedUserEntry{info: cloneUserInfo(info), usyncAt: now}
	if info.PictureURL != "" {
		entry.pictureAt = now
	}
	u.userMemo.entries[userInfoCacheKey(instanceID, jid)] = entry
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
	uc, err := u.coalescedQueryUserInfo(ctx, instanceID, client, lids, jids)
	if errors.Is(err, whatsmeow.ErrIQRateOverLimit) {
		return nil, u.markAndWrapOverLimit(instanceID)
	}
	if err == nil {
		u.storeCachedUserInfo(instanceID, jids, uc)
	}
	return uc, err
}

func (u *userService) coalescedQueryUserInfo(ctx context.Context, instanceID string, client profileQueryClient, lids store.LIDStore, jids []types.JID) (*UserCollection, error) {
	if u == nil || u.userMemo == nil {
		return queryUserInfo(ctx, client, lids, jids)
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
	call := &inflightUserInfo{done: make(chan struct{}), joined: make(chan struct{}, 1)}
	u.userMemo.inflight[key] = call
	u.userMemo.mu.Unlock()

	uc, err := queryUserInfo(ctx, client, lids, jids)
	call.uc = cloneUserCollection(uc)
	call.err = err
	close(call.done)

	u.userMemo.mu.Lock()
	delete(u.userMemo.inflight, key)
	u.userMemo.mu.Unlock()
	return cloneUserCollection(uc), err
}
