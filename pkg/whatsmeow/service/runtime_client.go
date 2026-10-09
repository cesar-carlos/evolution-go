package whatsmeow_service

import (
	"context"
	"errors"
	"time"

	"go.mau.fi/whatsmeow"
)

func (m *MyClient) runtimeContext() context.Context {
	if m.run == nil {
		return context.Background()
	}
	return m.run.ctx
}

func (m *MyClient) async(fn func()) {
	if m.run != nil {
		m.run.worker(fn)
		return
	}
	go fn() // Unit fixtures can exercise event delivery without a live session.
}

// snapshot isolates settings and instance mutations made by an event from HTTP
// updates and from the next event. Runtime identity and worker ownership remain shared.
func (m *MyClient) snapshot() *MyClient {
	if m.settingsMu != nil {
		m.settingsMu.RLock()
		defer m.settingsMu.RUnlock()
	}
	copy := *m
	if m.Instance != nil {
		instance := *m.Instance
		copy.Instance = &instance
	}
	copy.subscriptions = append([]string(nil), m.subscriptions...)
	return &copy
}

func (w whatsmeowService) runtimeClient(id string) *MyClient {
	if w.sessions == nil {
		return nil
	}
	slot, err := w.sessions.slot(id)
	if err != nil {
		return nil
	}
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	if slot.run == nil || slot.run.ctx.Err() != nil {
		return nil
	}
	return slot.run.client
}

// WaitClient waits for the owned startup handshake, bounded by the caller's
// context and a maximum of 30 seconds. Cancellation does not stop the session.
func (w whatsmeowService) WaitClient(ctx context.Context, id string) (*whatsmeow.Client, error) {
	return w.waitClient(ctx, id, false)
}

// WaitPairing waits until QR or passkey pairing is available, rather than
// assuming the WebSocket handshake also completed pairing initialization.
func (w whatsmeowService) WaitPairing(ctx context.Context, id string) (*whatsmeow.Client, error) {
	return w.waitClient(ctx, id, true)
}

func (w whatsmeowService) waitClient(ctx context.Context, id string, pairing bool) (*whatsmeow.Client, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	slot, err := w.sessions.slot(id)
	if err != nil {
		return nil, err
	}
	slot.mu.RLock()
	r := slot.run
	slot.mu.RUnlock()
	if r == nil {
		return nil, errors.New("no active session found")
	}
	select {
	case <-r.ready:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	if r.readyErr != nil {
		return nil, r.readyErr
	}
	if pairing && !r.paired.Load() {
		select {
		case <-r.pairingReady:
		case <-r.ctx.Done():
			return nil, errors.New("session ended before pairing became available")
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	client := w.GetClient(id)
	if client == nil {
		return nil, errors.New("session ended while waiting for connection")
	}
	return client, nil
}

// PairingExpired prevents polling QR endpoints from starting endless pairing cycles.
// An explicit connect or pair request can still start a new execution.
func (w whatsmeowService) PairingExpired(id string) bool {
	slot, err := w.sessions.slot(id)
	if err != nil {
		return true
	}
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	return slot.pairingExpired
}
