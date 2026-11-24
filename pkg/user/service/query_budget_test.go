package user_service

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	instance_model "github.com/evolution-foundation/evolution-go/pkg/instance/model"
	"go.mau.fi/whatsmeow"
)

func TestQueryRetryAfter(t *testing.T) {
	if got := QueryRetryAfter(queryRateLimitedError{retryAfter: 12 * time.Second}); got != 12*time.Second {
		t.Fatalf("wrapped remaining = %s", got)
	}
	if got := QueryRetryAfter(fmt.Errorf("usync: %w", whatsmeow.ErrIQRateOverLimit)); got != queryOverLimitCooldown {
		t.Fatalf("bare overlimit = %s", got)
	}
	if got := QueryRetryAfter(errors.New("other")); got != 0 {
		t.Fatalf("other = %s", got)
	}
}

func TestQueryCooldownIsolatesInstancesAndExpires(t *testing.T) {
	now := time.Date(2026, 11, 24, 12, 0, 0, 0, time.UTC)
	svc := &userService{
		now:           func() time.Time { return now },
		queryCooldown: 45 * time.Second,
		queryLimit:    newInstanceQueryLimit(),
		userMemo:      newUserInfoMemo(),
	}
	if err := svc.errIfQueryCoolingDown("one"); err != nil {
		t.Fatal(err)
	}
	if err := svc.markAndWrapOverLimit("one"); !errors.Is(err, whatsmeow.ErrIQRateOverLimit) {
		t.Fatalf("mark = %v", err)
	}
	if err := svc.errIfQueryCoolingDown("one"); !errors.Is(err, whatsmeow.ErrIQRateOverLimit) {
		t.Fatalf("same instance still querying: %v", err)
	}
	if QueryRetryAfter(svc.errIfQueryCoolingDown("one")) != 45*time.Second {
		t.Fatal("retry-after should match remaining cooldown")
	}
	if err := svc.errIfQueryCoolingDown("two"); err != nil {
		t.Fatalf("neighbor instance blocked: %v", err)
	}
	now = now.Add(45 * time.Second)
	if err := svc.errIfQueryCoolingDown("one"); err != nil {
		t.Fatalf("cooldown did not expire: %v", err)
	}
}

func TestGetAvatarShortCircuitsCooldownWithoutClient(t *testing.T) {
	now := time.Date(2026, 11, 24, 12, 0, 0, 0, time.UTC)
	svc := &userService{
		now:           func() time.Time { return now },
		queryCooldown: time.Minute,
		queryLimit:    newInstanceQueryLimit(),
		userMemo:      newUserInfoMemo(),
	}
	_ = svc.markAndWrapOverLimit("one")
	_, err := svc.GetAvatar(nil, &GetAvatarStruct{Number: "14155550100"}, &instance_model.Instance{Id: "one"})
	if !errors.Is(err, whatsmeow.ErrIQRateOverLimit) {
		t.Fatalf("avatar during cooldown = %v", err)
	}
}

func TestQueryCooldownConcurrentMarks(t *testing.T) {
	svc := &userService{
		now:           time.Now,
		queryCooldown: time.Minute,
		queryLimit:    newInstanceQueryLimit(),
		userMemo:      newUserInfoMemo(),
	}
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = svc.markAndWrapOverLimit("one")
			_ = svc.errIfQueryCoolingDown("one")
			_ = svc.errIfQueryCoolingDown("two")
		}()
	}
	wg.Wait()
	if err := svc.errIfQueryCoolingDown("one"); !errors.Is(err, whatsmeow.ErrIQRateOverLimit) {
		t.Fatalf("lost cooldown under race: %v", err)
	}
}
