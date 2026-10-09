package whatsmeow_service

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	instance_model "github.com/evolution-foundation/evolution-go/pkg/instance/model"
	"github.com/evolution-foundation/evolution-go/pkg/passkey/ceremony"
	"go.mau.fi/whatsmeow"
)

func testSessions(t *testing.T) (*sessionRegistry, context.Context) {
	t.Helper()
	s := newSessionRegistry()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(func() {
		defer cancel()
		if err := s.shutdown(ctx); err != nil {
			t.Error(err)
		}
	})
	return s, ctx
}
func sessionData(id string) *ClientData {
	return &ClientData{Instance: &instance_model.Instance{Id: id, Token: "token-" + id, Jid: "5511999999999:1@s.whatsapp.net"}}
}
func liveSession(r *sessionRun) { r.signalReady(nil); <-r.ctx.Done() }

func TestSessionStopActionPrecedesTransportCleanup(t *testing.T) {
	s, ctx := testSessions(t)
	workerStopped := make(chan struct{})
	var actionRan bool
	r, err := s.start(ctx, sessionData("one"), func(r *sessionRun) {
		r.cleanup = func() {
			if !actionRan {
				t.Error("transport cleanup preceded stop action")
			}
		}
		r.worker(func() { <-r.ctx.Done(); close(workerStopped) })
		liveSession(r)
	})
	if err != nil {
		t.Fatal(err)
	}
	<-r.ready
	err = s.stop(ctx, "one", StopLogout, "", func(r *sessionRun) error {
		select {
		case <-workerStopped:
		default:
			t.Error("stop action preceded worker completion")
		}
		if r.transportCtx.Err() != nil {
			t.Error("transport cancelled before logout")
		}
		actionRan = true
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !actionRan || r.transportCtx.Err() == nil {
		t.Fatal("stop action or transport cleanup missing")
	}
}

func TestWaitPairingRequiresPairingEvent(t *testing.T) {
	s, ctx := testSessions(t)
	w := whatsmeowService{sessions: s, queryClients: newQueryClientIndex(nil)}
	client := &whatsmeow.Client{}
	w.registerQueryClient("one", client)
	r, err := s.start(ctx, sessionData("one"), liveSession)
	if err != nil {
		t.Fatal(err)
	}
	<-r.ready
	if got, err := w.WaitClient(ctx, "one"); err != nil || got != client {
		t.Fatal(got, err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := w.WaitPairing(cancelled, "one"); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if r.ctx.Err() != nil {
		t.Fatal("caller cancelled the session")
	}
	r.signalPairingReady()
	if got, err := w.WaitPairing(ctx, "one"); err != nil || got != client {
		t.Fatal(got, err)
	}
}

func TestSessionConcurrentStartsAndStops(t *testing.T) {
	s, ctx := testSessions(t)
	var calls atomic.Int32
	var wg sync.WaitGroup
	results := make(chan *sessionRun, 32)
	for range 32 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, err := s.start(ctx, sessionData("one"), func(r *sessionRun) { calls.Add(1); liveSession(r) })
			if err != nil {
				t.Error(err)
				return
			}
			<-r.ready
			results <- r
		}()
	}
	wg.Wait()
	close(results)
	var first *sessionRun
	for r := range results {
		if first == nil {
			first = r
		}
		if first != r {
			t.Fatal("duplicate session")
		}
	}
	if calls.Load() != 1 {
		t.Fatal("duplicate worker", calls.Load())
	}
	for range 32 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := s.stop(ctx, "one", StopManual, "", nil); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if first.current() || first.beginWork() {
		t.Fatal("stopped execution still accepts callbacks")
	}
}

func TestSessionStopWaitsWorkersAndPreservesOtherInstances(t *testing.T) {
	s, ctx := testSessions(t)
	entered, release := make(chan struct{}), make(chan struct{})
	r, err := s.start(ctx, sessionData("one"), func(r *sessionRun) { r.worker(func() { close(entered); <-release }); liveSession(r) })
	if err != nil {
		t.Fatal(err)
	}
	<-entered
	done := make(chan error, 1)
	go func() { done <- s.stop(ctx, "one", StopManual, "", nil) }()
	<-r.ctx.Done()
	other, err := s.start(ctx, sessionData("two"), liveSession)
	if err != nil {
		t.Fatal(err)
	}
	<-other.ready
	select {
	case <-done:
		t.Fatal("stop returned with active worker")
	default:
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if !other.current() {
		t.Fatal("another instance stopped")
	}
	newRun, err := s.start(ctx, sessionData("one"), liveSession)
	if err != nil {
		t.Fatal(err)
	}
	<-newRun.ready
	if r.current() || !newRun.current() {
		t.Fatal("execution identity lost")
	}
}

func TestSessionDeletionDuringStartupRejectsStaleStart(t *testing.T) {
	s, ctx := testSessions(t)
	entered := make(chan struct{})
	r, err := s.start(ctx, sessionData("one"), func(r *sessionRun) { close(entered); <-r.ctx.Done() })
	if err != nil {
		t.Fatal(err)
	}
	<-entered
	if err := s.stop(ctx, "one", StopDeleted, "token-one", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := s.start(ctx, sessionData("one"), liveSession); err == nil {
		t.Fatal("deleted instance restarted")
	}
	if r.current() {
		t.Fatal("deleted execution remains current")
	}
	newData := sessionData("one")
	newData.Instance.Token = "new-token"
	if _, err := s.start(ctx, newData, liveSession); err != nil {
		t.Fatal("recreated instance rejected", err)
	}
}

func TestSessionReconnectIsCoalescedAndBounded(t *testing.T) {
	s, ctx := testSessions(t)
	old, err := s.start(ctx, sessionData("one"), liveSession)
	if err != nil {
		t.Fatal(err)
	}
	<-old.ready
	entered, release := make(chan struct{}), make(chan struct{})
	var waits []time.Duration
	s.wait = func(ctx context.Context, d time.Duration) error {
		waits = append(waits, d)
		if len(waits) == 1 {
			close(entered)
			select {
			case <-release:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		return nil
	}
	var attempts atomic.Int32
	load := func(context.Context) (*ClientData, error) { return sessionData("one"), nil }
	execute := func(r *sessionRun) { attempts.Add(1); r.signalReady(errors.New("transport unavailable")) }
	if err := s.reconnect("one", old, load, execute); err != nil {
		t.Fatal(err)
	}
	<-entered
	var wg sync.WaitGroup
	for range 32 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := s.reconnect("one", old, load, execute); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	close(release)
	s.operations.Wait()
	if attempts.Load() != 5 || len(waits) != 5 {
		t.Fatal("unbounded or duplicate retry", attempts.Load(), waits)
	}
	for i, d := range waits {
		base := time.Second * time.Duration(1<<i)
		if d < base || d > base+base/5 {
			t.Fatal("invalid backoff", d)
		}
	}
}

func TestSessionManualStopCancelsPendingReconnect(t *testing.T) {
	s, ctx := testSessions(t)
	old, err := s.start(ctx, sessionData("one"), liveSession)
	if err != nil {
		t.Fatal(err)
	}
	<-old.ready
	entered := make(chan struct{})
	s.wait = func(ctx context.Context, d time.Duration) error { close(entered); <-ctx.Done(); return ctx.Err() }
	var attempts atomic.Int32
	if err := s.reconnect("one", old, func(context.Context) (*ClientData, error) { attempts.Add(1); return sessionData("one"), nil }, liveSession); err != nil {
		t.Fatal(err)
	}
	<-entered
	if err := s.stop(ctx, "one", StopManual, "", nil); err != nil {
		t.Fatal(err)
	}
	s.operations.Wait()
	if attempts.Load() != 0 {
		t.Fatal("manual stop triggered reconnection")
	}
}

func TestSessionShutdownRejectsNewWork(t *testing.T) {
	s, ctx := testSessions(t)
	r, err := s.start(ctx, sessionData("one"), liveSession)
	if err != nil {
		t.Fatal(err)
	}
	<-r.ready
	if err := s.shutdown(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := s.start(ctx, sessionData("two"), liveSession); !errors.Is(err, errSessionsClosed) {
		t.Fatal(err)
	}
	if err := s.reconnect("one", r, nil, liveSession); !errors.Is(err, errSessionsClosed) {
		t.Fatal(err)
	}
}

func TestIntentionalStopBlocksQRPollingUntilExplicitStart(t *testing.T) {
	s, ctx := testSessions(t)
	w := whatsmeowService{sessions: s}
	r, err := s.start(ctx, sessionData("one"), liveSession)
	if err != nil {
		t.Fatal(err)
	}
	<-r.ready
	if err := s.stop(ctx, "one", StopManual, "", nil); err != nil {
		t.Fatal(err)
	}
	if !w.PairingExpired("one") {
		t.Fatal("polling may restart an intentionally stopped session")
	}
	next, err := s.start(ctx, sessionData("one"), liveSession)
	if err != nil {
		t.Fatal(err)
	}
	<-next.ready
	if w.PairingExpired("one") {
		t.Fatal("explicit connect failed to reset pairing state")
	}
}

func TestSettingsSnapshotsAreIndependentDuringUpdates(t *testing.T) {
	m := &MyClient{settingsMu: &sync.RWMutex{}, Instance: &instance_model.Instance{Id: "one"}, subscriptions: []string{"Message"}}
	var wg sync.WaitGroup
	for range 32 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 20 {
				m.settingsMu.Lock()
				m.Instance = &instance_model.Instance{Id: "one", AlwaysOnline: true}
				m.subscriptions = []string{"ALL"}
				m.settingsMu.Unlock()
				snapshot := m.snapshot()
				snapshot.Instance.Id = "local-event"
				snapshot.subscriptions[0] = "local-event"
			}
		}()
	}
	wg.Wait()
	if got := m.snapshot(); got.Instance.Id != "one" || got.subscriptions[0] != "ALL" {
		t.Fatal("event mutated live settings")
	}
}

func TestRuntimeOperationBelongsToExecution(t *testing.T) {
	s, ctx := testSessions(t)
	w := whatsmeowService{sessions: s}
	r, err := s.start(ctx, sessionData("one"), func(r *sessionRun) {
		r.slot.mu.Lock()
		r.client = &MyClient{run: r}
		r.slot.mu.Unlock()
		liveSession(r)
	})
	if err != nil {
		t.Fatal(err)
	}
	<-r.ready
	m, done, err := w.runtimeOperation("one")
	if err != nil {
		t.Fatal(err)
	}
	stopped := make(chan error, 1)
	go func() { stopped <- s.stop(ctx, "one", StopManual, "", nil) }()
	<-m.runtimeContext().Done()
	select {
	case err := <-stopped:
		t.Fatal("stop bypassed request completion", err)
	default:
	}
	if _, _, err := w.runtimeOperation("one"); err == nil {
		t.Fatal("stopping execution accepted request")
	}
	done()
	if err := <-stopped; err != nil {
		t.Fatal(err)
	}
}

func TestOldCeremonyCannotAuthorizeReplacementClient(t *testing.T) {
	s, ctx := testSessions(t)
	w := whatsmeowService{sessions: s, passkeyCeremony: ceremony.NewStore()}
	oldToken := w.passkeyCeremony.Start("one", []byte(`{}`))
	w.passkeyCeremony.Clear("one")
	newToken := w.passkeyCeremony.Start("one", []byte(`{}`))
	r, err := s.start(ctx, sessionData("one"), func(r *sessionRun) {
		r.slot.mu.Lock()
		r.client = &MyClient{run: r}
		r.slot.mu.Unlock()
		liveSession(r)
	})
	if err != nil {
		t.Fatal(err)
	}
	<-r.ready
	if _, _, err := w.ceremonyOperation("one", oldToken); err == nil {
		t.Fatal("old token authorized replacement")
	}
	if _, _, err := w.ceremonyOperation("other", newToken); err == nil {
		t.Fatal("cross-instance token authorized")
	}
	_, done, err := w.ceremonyOperation("one", newToken)
	if err != nil {
		t.Fatal(err)
	}
	done()
}

type gateWaitContext struct {
	context.Context
	waiting chan struct{}
	once    sync.Once
}

func (c *gateWaitContext) Done() <-chan struct{} {
	c.once.Do(func() { close(c.waiting) })
	return c.Context.Done()
}

func TestStopIntentRejectsRacingReconnectBeforeGate(t *testing.T) {
	s, ctx := testSessions(t)
	r, err := s.start(ctx, sessionData("one"), liveSession)
	if err != nil {
		t.Fatal(err)
	}
	<-r.ready
	if err := lockSession(ctx, r.slot); err != nil {
		t.Fatal(err)
	}
	stopCtx := &gateWaitContext{Context: ctx, waiting: make(chan struct{})}
	stopped := make(chan error, 1)
	go func() { stopped <- s.stop(stopCtx, "one", StopManual, "", nil) }()
	<-stopCtx.waiting // Stop intent is registered, but the gate is still held.
	var loads atomic.Int32
	s.wait = func(context.Context, time.Duration) error { return nil }
	if err := s.reconnect("one", r, func(context.Context) (*ClientData, error) {
		loads.Add(1)
		return sessionData("one"), nil
	}, liveSession); err != nil {
		t.Fatal(err)
	}
	unlockSession(r.slot)
	if err := <-stopped; err != nil {
		t.Fatal(err)
	}
	s.operations.Wait()
	if loads.Load() != 0 {
		t.Fatal("reconnect bypassed pending stop intent")
	}
}
