package whatsmeow_service

import (
	"context"
	"errors"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)

// StopReason distinguishes intentional stops from transport failures. A stopped
// or deleted instance is never automatically restarted by an old callback.
type StopReason string

const (
	StopManual    StopReason = "Disconnected"
	StopLogout    StopReason = "Logged out"
	StopDeleted   StopReason = "Deleted"
	StopQRExpired StopReason = "QR expired"
	stopRestart   StopReason = "Reconnecting"
	stopShutdown  StopReason = "Server shutdown"
)

var errSessionsClosed = errors.New("session service is shutting down")

type sessionSlot struct {
	gate            chan struct{}
	mu              sync.RWMutex
	run             *sessionRun
	deletedToken    string
	pairingExpired  bool
	reconnecting    bool
	operationCancel context.CancelFunc
}

type sessionRun struct {
	ctx             context.Context
	cancel          context.CancelFunc
	done            chan struct{}
	ready           chan struct{}
	readyOnce       sync.Once
	readyErr        error
	pairingReady    chan struct{}
	pairingOnce     sync.Once
	transportCtx    context.Context
	transportCancel context.CancelFunc
	slot            *sessionSlot
	data            *ClientData
	client          *MyClient // protected by slot.mu
	workMu          sync.Mutex
	stopping        bool
	workers         sync.WaitGroup
	presenceOnce    sync.Once
	qrMu            sync.Mutex
	cleanup         func()
	stopAction      func(*sessionRun) error
	stopErr         error
	paired          atomic.Bool
	qrCount         atomic.Int32
}

func (r *sessionRun) signalReady(err error) {
	r.readyOnce.Do(func() { r.readyErr = err; close(r.ready) })
}

func (r *sessionRun) signalPairingReady() {
	r.pairingOnce.Do(func() { close(r.pairingReady) })
}

func (r *sessionRun) beginWork() bool {
	r.workMu.Lock()
	defer r.workMu.Unlock()
	if r.stopping || r.ctx.Err() != nil {
		return false
	}
	r.workers.Add(1)
	return true
}

func (r *sessionRun) worker(fn func()) {
	if !r.beginWork() {
		return
	}
	go func() { defer r.workers.Done(); fn() }()
}

func (r *sessionRun) current() bool {
	if r == nil {
		return true
	} // isolated event fixtures do not own a runtime.
	r.slot.mu.RLock()
	defer r.slot.mu.RUnlock()
	return r.slot.run == r && r.ctx.Err() == nil
}

type sessionRegistry struct {
	mu         sync.Mutex
	slots      map[string]*sessionSlot
	closed     bool
	ctx        context.Context
	cancel     context.CancelFunc
	operations sync.WaitGroup
	wait       func(context.Context, time.Duration) error
}

func newSessionRegistry() *sessionRegistry {
	ctx, cancel := context.WithCancel(context.Background())
	return &sessionRegistry{slots: make(map[string]*sessionSlot), ctx: ctx, cancel: cancel, wait: waitSession}
}

func waitSession(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (s *sessionRegistry) slot(id string) (*sessionSlot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, errSessionsClosed
	}
	slot := s.slots[id]
	if slot == nil {
		slot = &sessionSlot{gate: make(chan struct{}, 1)}
		slot.gate <- struct{}{}
		s.slots[id] = slot
	}
	return slot, nil
}

func lockSession(ctx context.Context, slot *sessionSlot) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-slot.gate:
		return nil
	}
}
func unlockSession(slot *sessionSlot) { slot.gate <- struct{}{} }

func (s *sessionRegistry) start(ctx context.Context, cd *ClientData, execute func(*sessionRun)) (*sessionRun, error) {
	if cd == nil || cd.Instance == nil {
		return nil, errors.New("instance is required")
	}
	slot, err := s.slot(cd.Instance.Id)
	if err != nil {
		return nil, err
	}
	if err = lockSession(ctx, slot); err != nil {
		return nil, err
	}
	defer unlockSession(slot)
	return s.startLocked(ctx, slot, cd, execute)
}

func (s *sessionRegistry) startLocked(ctx context.Context, slot *sessionSlot, cd *ClientData, execute func(*sessionRun)) (*sessionRun, error) {
	for {
		s.mu.Lock()
		slot.mu.Lock()
		if s.closed {
			slot.mu.Unlock()
			s.mu.Unlock()
			return nil, errSessionsClosed
		}
		if slot.deletedToken != "" && slot.deletedToken == cd.Instance.Token {
			slot.mu.Unlock()
			s.mu.Unlock()
			return nil, errors.New("instance was deleted")
		}
		old := slot.run
		if old != nil {
			if old.ctx.Err() == nil {
				slot.mu.Unlock()
				s.mu.Unlock()
				return old, nil
			}
			slot.mu.Unlock()
			s.mu.Unlock()
			select {
			case <-old.done:
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-s.ctx.Done():
				return nil, errSessionsClosed
			}
			continue
		}
		copyData := *cd
		copyInstance := *cd.Instance
		copyData.Instance = &copyInstance
		copyData.Subscriptions = append([]string(nil), cd.Subscriptions...)
		runCtx, cancel := context.WithCancel(s.ctx)
		transportCtx, transportCancel := context.WithCancel(s.ctx)
		r := &sessionRun{ctx: runCtx, cancel: cancel, transportCtx: transportCtx, transportCancel: transportCancel,
			done: make(chan struct{}), ready: make(chan struct{}), pairingReady: make(chan struct{}), slot: slot, data: &copyData}
		slot.run = r
		slot.deletedToken = ""
		slot.pairingExpired = false
		slot.mu.Unlock()
		s.mu.Unlock()
		go func() {
			defer func() {
				r.workMu.Lock()
				r.stopping = true
				r.cancel()
				r.workMu.Unlock()
				r.workers.Wait()
				if r.stopAction != nil {
					r.stopErr = r.stopAction(r)
				}
				if r.cleanup != nil {
					r.cleanup()
				}
				r.transportCancel()
				r.signalReady(errors.New("session ended before connecting"))
				slot.mu.Lock()
				if slot.run == r {
					slot.run = nil
				}
				slot.mu.Unlock()
				close(r.done)
			}()
			execute(r)
		}()
		return r, nil
	}
}

func (s *sessionRegistry) stop(ctx context.Context, id string, reason StopReason, token string, action func(*sessionRun) error) error {
	slot, err := s.slot(id)
	if err != nil {
		return err
	}
	// Cancel a pending reconnect before waiting for its per-instance gate.
	slot.mu.Lock()
	if slot.operationCancel != nil {
		slot.operationCancel()
	}
	slot.mu.Unlock()
	if err = lockSession(ctx, slot); err != nil {
		return err
	}
	defer unlockSession(slot)
	slot.mu.Lock()
	r := slot.run
	if reason == StopManual || reason == StopLogout || reason == StopDeleted {
		slot.pairingExpired = true
	}
	if reason == StopDeleted {
		slot.deletedToken = token
		if r != nil {
			slot.deletedToken = r.data.Instance.Token
		}
	}
	if r != nil {
		r.workMu.Lock()
		if !r.stopping {
			r.stopAction = action
		}
		r.cancel()
		r.workMu.Unlock()
	}
	slot.mu.Unlock()
	if r == nil {
		if action != nil {
			return action(nil)
		}
		return nil
	}
	select {
	case <-r.done:
	case <-ctx.Done():
		return ctx.Err()
	}
	return r.stopErr
}

func (s *sessionRegistry) reconnect(id string, expected *sessionRun, load func(context.Context) (*ClientData, error), execute func(*sessionRun)) error {
	slot, err := s.slot(id)
	if err != nil {
		return err
	}
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return errSessionsClosed
	}
	slot.mu.Lock()
	if slot.reconnecting || (expected != nil && slot.run != expected) {
		slot.mu.Unlock()
		s.mu.Unlock()
		return nil
	}
	ctx, cancel := context.WithCancel(s.ctx)
	slot.reconnecting = true
	slot.operationCancel = cancel
	s.operations.Add(1)
	slot.mu.Unlock()
	s.mu.Unlock()
	go func() {
		defer s.operations.Done()
		defer cancel()
		defer func() { slot.mu.Lock(); slot.reconnecting = false; slot.operationCancel = nil; slot.mu.Unlock() }()
		if lockSession(ctx, slot) != nil {
			return
		}
		defer unlockSession(slot)
		for attempt := 0; attempt < 5; attempt++ {
			if expected != nil { // Automatic reconnects use bounded backoff.
				delay := time.Second * time.Duration(1<<attempt)
				jitter := time.Duration(rand.Int63n(int64(delay/5) + 1))
				if s.wait(ctx, delay+jitter) != nil {
					return
				}
			}
			slot.mu.Lock()
			old := slot.run
			if old != nil {
				old.cancel()
			}
			slot.mu.Unlock()
			if old != nil {
				select {
				case <-old.done:
				case <-ctx.Done():
					return
				}
			}
			cd, err := load(ctx)
			if err != nil {
				return
			}
			if expected != nil && cd.Instance.Jid == "" {
				return
			}
			cd.automatic = expected != nil
			r, err := s.startLocked(ctx, slot, cd, execute)
			if err != nil {
				return
			}
			select {
			case <-r.ready:
			case <-ctx.Done():
				r.cancel()
				return
			}
			if r.readyErr == nil {
				return
			}
			if expected == nil {
				return
			} // Explicit API request is one attempt.
		}
	}()
	return nil
}

func (s *sessionRegistry) beginShutdown() {
	s.mu.Lock()
	s.closed = true
	s.mu.Unlock()
}

func (s *sessionRegistry) shutdown(ctx context.Context) error {
	s.beginShutdown()
	s.cancel()
	s.mu.Lock()
	var runs []*sessionRun
	for _, slot := range s.slots {
		slot.mu.RLock()
		if slot.run != nil {
			runs = append(runs, slot.run)
		}
		slot.mu.RUnlock()
	}
	s.mu.Unlock()
	for _, r := range runs {
		select {
		case <-r.done:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	done := make(chan struct{})
	go func() { s.operations.Wait(); close(done) }()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// StopInstance stops only this instance and waits for its workers. Logout is
// performed after transport workers stop, while the shared store remains open.
func (w whatsmeowService) StopInstance(ctx context.Context, id string, reason StopReason) error {
	var token string
	if reason == StopDeleted {
		instance, err := w.instanceRepository.GetInstanceByIDContext(ctx, id)
		if err != nil {
			return err
		}
		token = instance.Token
	}
	return w.sessions.stop(ctx, id, reason, token, func(r *sessionRun) error {
		if reason == StopLogout || reason == StopDeleted {
			if r != nil && r.client != nil && r.client.WAClient.IsConnected() && r.client.WAClient.Store.ID != nil {
				if err := r.client.WAClient.Logout(ctx); err != nil {
					return err
				}
			}
		}
		// Persist stop state while the instance gate is still held. An old stop
		// must not overwrite a replacement execution's Connected event.
		instance, err := w.instanceRepository.GetInstanceByIDContext(ctx, id)
		if err != nil {
			return err
		}
		instance.Connected = false
		instance.DisconnectReason = string(reason)
		if reason == StopManual {
			instance.Events = ""
		}
		return w.instanceRepository.Update(instance)
	})
}

// BeginShutdown prevents new session starts before HTTP requests are drained.
func (w whatsmeowService) BeginShutdown() { w.sessions.beginShutdown() }
